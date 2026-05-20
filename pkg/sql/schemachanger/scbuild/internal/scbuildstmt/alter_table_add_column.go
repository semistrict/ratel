// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package scbuildstmt

import (
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/schemaexpr"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	"github.com/cockroachdb/cockroach/pkg/sql/privilege"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scerrors"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scpb"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/catid"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlerrors"
	"github.com/cockroachdb/cockroach/pkg/sql/sqltelemetry"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
)

func alterTableAddColumn(
	b BuildCtx, tn *tree.TableName, tbl *scpb.Table, t *tree.AlterTableAddColumn,
) {
	b.IncrementSchemaChangeAlterCounter("table", "add_column")
	d := t.ColumnDef
	// Check column non-existence.
	{
		elts := b.ResolveColumn(tbl.TableID, d.Name, ResolveParams{
			IsExistenceOptional: true,
			RequiredPrivilege:   privilege.CREATE,
		})
		_, _, col := scpb.FindColumn(elts)
		if col != nil {
			if t.IfNotExists {
				return
			}
			panic(sqlerrors.NewColumnAlreadyExistsError(string(d.Name), tn.Object()))
		}
	}
	if d.IsSerial {
		panic(scerrors.NotImplementedErrorf(d, "contains serial data type"))
	}
	if d.IsComputed() {
		d.Computed.Expr = schemaexpr.MaybeRewriteComputedColumn(d.Computed.Expr, b.SessionData())
	}
	// Some of the building for the index exists below but end-to-end support is
	// not complete so we return an error.
	if d.Unique.IsUnique {
		panic(scerrors.NotImplementedErrorf(d, "contains unique constraint"))
	}
	if d.Nullable.Nullability == tree.NotNull && !d.HasDefaultExpr() && !d.IsComputed() {
		panic(scerrors.NotImplementedErrorf(d, "contains NOT NULL without a default expression"))
	}
	cdd, err := tabledesc.MakeColumnDefDescs(
		b, d, b.SemaCtx(), b.EvalCtx(), tree.ColumnDefaultExprInAddColumn,
	)
	if err != nil {
		panic(err)
	}
	desc := cdd.ColumnDescriptor
	if desc.Type.Family() == types.EnumFamily {
		b.IncrementEnumCounter(sqltelemetry.EnumInTable)
	} else {
		b.IncrementSchemaChangeAddColumnTypeCounter(desc.Type.TelemetryName())
	}
	if desc.HasDefault() {
		b.IncrementSchemaChangeAddColumnQualificationCounter("default_expr")
	}
	if desc.HasOnUpdate() {
		b.IncrementSchemaChangeAddColumnQualificationCounter("on_update")
	}
	spec := addColumnSpec{
		tbl: tbl,
		col: &scpb.Column{
			TableID:                 tbl.TableID,
			ColumnID:                b.NextTableColumnID(tbl),
			IsHidden:                desc.Hidden,
			IsInaccessible:          desc.Inaccessible,
			GeneratedAsIdentityType: desc.GeneratedAsIdentityType,
			PgAttributeNum:          desc.PGAttributeNum,
		},
	}
	if ptr := desc.GeneratedAsIdentitySequenceOption; ptr != nil {
		spec.col.GeneratedAsIdentitySequenceOption = *ptr
	}
	spec.name = &scpb.ColumnName{
		TableID:  tbl.TableID,
		ColumnID: spec.col.ColumnID,
		Name:     string(d.Name),
	}
	spec.colType = &scpb.ColumnType{
		TableID:    tbl.TableID,
		ColumnID:   spec.col.ColumnID,
		IsNullable: desc.Nullable,
		IsVirtual:  desc.Virtual,
	}
	if desc.IsComputed() {
		expr := b.ComputedColumnExpression(tbl, d)
		spec.colType.ComputeExpr = b.WrapExpression(tbl.TableID, expr)
		spec.colType.TypeT = scpb.TypeT{Type: desc.Type}
	} else {
		spec.colType.TypeT = b.ResolveTypeRef(d.Type)
	}
	if desc.HasDefault() {
		spec.def = &scpb.ColumnDefaultExpression{
			TableID:    tbl.TableID,
			ColumnID:   spec.col.ColumnID,
			Expression: *b.WrapExpression(tbl.TableID, cdd.DefaultExpr),
		}
	}
	if desc.HasOnUpdate() {
		spec.onUpdate = &scpb.ColumnOnUpdateExpression{
			TableID:    tbl.TableID,
			ColumnID:   spec.col.ColumnID,
			Expression: *b.WrapExpression(tbl.TableID, cdd.OnUpdateExpr),
		}
	}
	// Add secondary indexes for this column.
	if newPrimary := addColumn(b, spec); newPrimary != nil {
		if idx := cdd.PrimaryKeyOrUniqueIndexDescriptor; idx != nil {
			idx.ID = b.NextTableIndexID(tbl)
			addSecondaryIndexTargetsForAddColumn(b, tbl, idx, newPrimary.SourceIndexID)
		}
	}
}

type addColumnSpec struct {
	tbl      *scpb.Table
	col      *scpb.Column
	name     *scpb.ColumnName
	colType  *scpb.ColumnType
	def      *scpb.ColumnDefaultExpression
	onUpdate *scpb.ColumnOnUpdateExpression
	comment  *scpb.ColumnComment
	notNull  bool
}

// addColumn is a helper function which adds column element targets and returns
// the current primary index, if one exists.
func addColumn(b BuildCtx, spec addColumnSpec, _ ...tree.NodeFormatter) (backing *scpb.PrimaryIndex) {
	b.Add(spec.col)
	b.Add(spec.name)
	b.Add(spec.colType)
	if spec.def != nil {
		b.Add(spec.def)
	}
	if spec.onUpdate != nil {
		b.Add(spec.onUpdate)
	}
	if spec.comment != nil {
		b.Add(spec.comment)
	}
	if !spec.colType.IsNullable {
		b.Add(&scpb.ColumnNotNull{
			TableID:  spec.tbl.TableID,
			ColumnID: spec.col.ColumnID,
		})
	}
	// Add or update primary index for non-virtual columns.
	if spec.colType.IsVirtual {
		return nil
	}
	requiresBackfill := spec.colType.ComputeExpr != nil || spec.def != nil || !spec.colType.IsNullable
	var existing *scpb.PrimaryIndex
	publicTargets := b.QueryByID(spec.tbl.TableID).Filter(
		func(_ scpb.Status, target scpb.TargetStatus, _ scpb.Element) bool {
			return target == scpb.ToPublic
		},
	)
	scpb.ForEachPrimaryIndex(publicTargets, func(status scpb.Status, _ scpb.TargetStatus, idx *scpb.PrimaryIndex) {
		existing = idx
	})
	if !requiresBackfill || existing == nil {
		return existing
	}

	existingSpec := makeIndexSpec(b, spec.tbl.TableID, existing.IndexID)
	b.Drop(existingSpec.primary)
	if existingSpec.name != nil {
		b.Drop(existingSpec.name)
	}
	if existingSpec.partitioning != nil {
		b.Drop(existingSpec.partitioning)
	}

	replacement := existingSpec.clone()
	replacementIndexID := b.NextTableIndexID(spec.tbl)
	replacement.primary.IndexID = replacementIndexID
	replacement.primary.ConstraintID = b.NextTableConstraintID(spec.tbl.TableID)
	replacement.primary.SourceIndexID = existing.IndexID
	replacement.constrComment = nil
	if replacement.name != nil {
		replacement.name.IndexID = replacementIndexID
	}
	if replacement.partitioning != nil {
		replacement.partitioning.IndexID = replacementIndexID
	}
	if replacement.data != nil {
		replacement.data.IndexID = replacementIndexID
	}
	var storedOrdinal uint32
	for _, ic := range replacement.columns {
		ic.IndexID = replacementIndexID
		if ic.Kind == scpb.IndexColumn_STORED && ic.OrdinalInKind >= storedOrdinal {
			storedOrdinal = ic.OrdinalInKind + 1
		}
	}
	replacement.columns = append(replacement.columns, &scpb.IndexColumn{
		TableID:       spec.tbl.TableID,
		IndexID:       replacementIndexID,
		ColumnID:      spec.col.ColumnID,
		OrdinalInKind: storedOrdinal,
		Kind:          scpb.IndexColumn_STORED,
	})
	replacement.apply(func(e scpb.Element) { b.Add(e) })
	return replacement.primary
}

func getImplicitSecondaryIndexName(
	b BuildCtx, descID descpb.ID, indexID descpb.IndexID, numImplicitColumns int,
) string {
	return fmt.Sprintf("idx_%d", indexID)
}

func addSecondaryIndexTargetsForAddColumn(
	b BuildCtx, tbl *scpb.Table, desc *descpb.IndexDescriptor, sourceID catid.IndexID,
) {
	index := scpb.Index{
		TableID:       tbl.TableID,
		IndexID:       desc.ID,
		IsUnique:      desc.Unique,
		IsInverted:    desc.Type == descpb.IndexDescriptor_INVERTED,
		SourceIndexID: sourceID,
	}
	if desc.Sharded.IsSharded {
		index.Sharding = &desc.Sharded
	}
	b.Add(&scpb.SecondaryIndex{Index: index})
	b.Add(&scpb.IndexName{
		TableID: tbl.TableID,
		IndexID: index.IndexID,
		Name:    desc.Name,
	})
	if p := &desc.Partitioning; len(p.List)+len(p.Range) > 0 {
		b.Add(&scpb.IndexPartitioning{
			TableID:                tbl.TableID,
			IndexID:                index.IndexID,
			PartitioningDescriptor: *protoutil.Clone(p).(*catpb.PartitioningDescriptor),
		})
	}
}
