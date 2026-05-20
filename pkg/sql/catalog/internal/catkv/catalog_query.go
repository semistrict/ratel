// Copyright 2022 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package catkv

import (
	"context"
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/config/zonepb"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catalogkeys"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descbuilder"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/nstree"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc/valueside"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlerrors"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/encoding"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
	"github.com/cockroachdb/errors"
)

// catalogQuery holds the state necessary to perform a catalog query.
//
// Objects of this type should be exclusively created and used by catalogReader
// methods.
type catalogQuery struct {
	codec                keys.SQLCodec
	isDescriptorRequired bool
	expectedType         catalog.DescriptorType
}

// query the catalog to retrieve data from the descriptor and namespace tables.
//
// Any results pertaining to the system database are passed to the system
// database cache to potentially update it with them.
func (cq catalogQuery) query(
	ctx context.Context,
	txn *kv.Txn,
	out *nstree.MutableCatalog,
	in func(codec keys.SQLCodec, b *kv.Batch),
) error {
	if txn == nil {
		return errors.AssertionFailedf("nil txn for catalog query")
	}
	b := txn.NewBatch()
	in(cq.codec, b)
	if err := txn.Run(ctx, b); err != nil {
		return err
	}
	for _, result := range b.Results {
		if result.Err != nil {
			return result.Err
		}
		for _, row := range result.Rows {
			_, catTableID, err := cq.codec.DecodeTablePrefix(row.Key)
			if err != nil {
				return err
			}
			switch catTableID {
			case keys.NamespaceTableID:
				err = cq.processNamespaceResultRow(row, out)
			case keys.DescriptorTableID:
				err = cq.processDescriptorResultRow(row, out)
			case keys.CommentsTableID:
				err = cq.processCommentsResultRow(row, out)
			case keys.ZonesTableID:
				err = cq.processZonesResultRow(row, out)
			default:
				err = errors.AssertionFailedf("unexpected catalog key %s", row.Key.String())
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (cq catalogQuery) processNamespaceResultRow(row kv.KeyValue, cb *nstree.MutableCatalog) error {
	nameInfo, err := catalogkeys.DecodeNameMetadataKey(cq.codec, row.Key)
	if err != nil {
		return err
	}
	if row.Exists() {
		id, err := decodeNamespaceValue(row.Value)
		if err != nil {
			return err
		}
		cb.UpsertNamespaceEntry(nameInfo, id, row.Value.Timestamp)
	}
	return nil
}

func (cq catalogQuery) processDescriptorResultRow(
	row kv.KeyValue, cb *nstree.MutableCatalog,
) error {
	if !row.Exists() {
		return nil
	}
	u32ID, err := cq.codec.DecodeDescMetadataID(row.Key)
	if err != nil {
		return err
	}
	id := descpb.ID(u32ID)
	expectedType := cq.expectedType
	if expectedType == "" {
		expectedType = catalog.Any
	}
	rawBytesInStorage := row.Value.TagAndDataBytes()
	isLegacyDescriptorRow := false
	if rowGroupID, err := keys.DecodeFamilyKey(row.Key); err == nil &&
		rowGroupID != keys.DescriptorTableDescriptorColFamID {
		isLegacyDescriptorRow = true
		rawBytesInStorage = nil
	}
	if isLegacyDescriptorRow && cb.LookupDescriptor(id) != nil {
		return nil
	}
	desc, err := build(expectedType, id, row.Value, rawBytesInStorage, cq.isDescriptorRequired)
	if err != nil {
		return wrapError(expectedType, id, err)
	}
	cb.UpsertDescriptor(desc)
	return nil
}

func (cq catalogQuery) processCommentsResultRow(row kv.KeyValue, cb *nstree.MutableCatalog) error {
	remaining, cmtKey, err := catalogkeys.DecodeCommentMetadataID(cq.codec, row.Key)
	if err != nil {
		return err
	}
	_, famID, err := encoding.DecodeUvarintAscending(remaining)
	if err != nil {
		return err
	}

	// Skip row groups which cannot contain the comment string.
	if famID != 0 && famID != keys.CommentsTableCommentColFamID {
		return nil
	}
	comment, err := decodeCommentValue(row.Value)
	if err != nil {
		return err
	}
	if comment == "" {
		return nil
	}
	return cb.UpsertComment(cmtKey, comment)
}

func (cq catalogQuery) processZonesResultRow(row kv.KeyValue, cb *nstree.MutableCatalog) error {
	remaining, id, err := cq.codec.DecodeZoneConfigMetadataID(row.Key)
	if err != nil {
		return err
	}
	_, famID, err := encoding.DecodeUvarintAscending(remaining)
	if err != nil {
		return err
	}

	// Skip not interested row groups or non-existing keys.
	if famID != 0 {
		return nil
	}

	zoneBytes, err := decodeZoneConfigValue(row)
	if err != nil {
		return err
	}
	if len(zoneBytes) == 0 {
		return nil
	}

	var zoneConfig zonepb.ZoneConfig
	if err := protoutil.Unmarshal(zoneBytes, &zoneConfig); err != nil {
		return errors.Wrapf(err, "decoding zone config for id %d", id)
	}
	cb.UpsertZoneConfig(descpb.ID(id), &zoneConfig, zoneBytes)
	return nil
}

func decodeZoneConfigValue(row kv.KeyValue) ([]byte, error) {
	if row.Value == nil {
		return nil, nil
	}
	switch row.Value.GetTag() {
	case roachpb.ValueType_BYTES:
		return row.Value.GetBytes()
	case roachpb.ValueType_TUPLE:
		valueBytes, err := row.Value.GetTuple()
		if err != nil {
			return nil, err
		}
		var alloc tree.DatumAlloc
		var lastColID descpb.ColumnID
		for len(valueBytes) > 0 {
			_, dataOffset, colIDDiff, typ, err := encoding.DecodeValueTag(valueBytes)
			if err != nil {
				return nil, err
			}
			colID := lastColID + descpb.ColumnID(colIDDiff)
			lastColID = colID
			if colID != keys.ZonesTableConfigColumnID {
				length, err := encoding.PeekValueLengthWithOffsetsAndType(valueBytes, dataOffset, typ)
				if err != nil {
					return nil, err
				}
				valueBytes = valueBytes[length:]
				continue
			}
			datum, _, err := valueside.Decode(&alloc, types.Bytes, valueBytes)
			if err != nil {
				return nil, err
			}
			if datum == tree.DNull {
				return nil, nil
			}
			return []byte(*datum.(*tree.DBytes)), nil
		}
		return nil, nil
	default:
		return nil, errors.AssertionFailedf("unexpected zone config value type %s", row.Value.GetTag())
	}
}

func decodeNamespaceValue(value *roachpb.Value) (descpb.ID, error) {
	const namespaceIDColumnID = descpb.ColumnID(4)
	if value == nil {
		return descpb.InvalidID, nil
	}
	switch value.GetTag() {
	case roachpb.ValueType_INT:
		id, err := value.GetInt()
		return descpb.ID(id), err
	case roachpb.ValueType_TUPLE:
		datum, err := decodeTupleColumn(value, namespaceIDColumnID, types.Int)
		if err != nil || datum == tree.DNull {
			return descpb.InvalidID, err
		}
		return descpb.ID(tree.MustBeDInt(datum)), nil
	default:
		return descpb.InvalidID, errors.AssertionFailedf("unexpected namespace value type %s", value.GetTag())
	}
}

func decodeCommentValue(value *roachpb.Value) (string, error) {
	const commentColumnID = descpb.ColumnID(4)
	if value == nil {
		return "", nil
	}
	switch value.GetTag() {
	case roachpb.ValueType_BYTES:
		b, err := value.GetBytes()
		return string(b), err
	case roachpb.ValueType_TUPLE:
		datum, err := decodeTupleColumn(value, commentColumnID, types.String)
		if err != nil || datum == tree.DNull {
			return "", err
		}
		return string(tree.MustBeDString(datum)), nil
	default:
		return "", errors.AssertionFailedf("unexpected comment value type %s", value.GetTag())
	}
}

func decodeTupleColumn(
	value *roachpb.Value, targetColumnID descpb.ColumnID, typ *types.T,
) (tree.Datum, error) {
	valueBytes, err := value.GetTuple()
	if err != nil {
		return nil, err
	}
	var alloc tree.DatumAlloc
	var lastColID descpb.ColumnID
	for len(valueBytes) > 0 {
		_, dataOffset, colIDDiff, valueType, err := encoding.DecodeValueTag(valueBytes)
		if err != nil {
			return nil, err
		}
		colID := lastColID + descpb.ColumnID(colIDDiff)
		lastColID = colID
		length, err := encoding.PeekValueLengthWithOffsetsAndType(valueBytes, dataOffset, valueType)
		if err != nil {
			return nil, err
		}
		if colID == targetColumnID {
			datum, remaining, err := valueside.Decode(&alloc, typ, valueBytes[:length])
			if err != nil {
				return nil, err
			}
			if len(remaining) != 0 {
				return nil, errors.AssertionFailedf("unexpected trailing bytes decoding column %d", targetColumnID)
			}
			return datum, nil
		}
		valueBytes = valueBytes[length:]
	}
	return tree.DNull, nil
}

func wrapError(expectedType catalog.DescriptorType, id descpb.ID, err error) error {
	switch expectedType {
	case catalog.Table:
		return catalog.WrapTableDescRefErr(id, err)
	case catalog.Database:
		return catalog.WrapDatabaseDescRefErr(id, err)
	case catalog.Schema:
		return catalog.WrapSchemaDescRefErr(id, err)
	case catalog.Type:
		return catalog.WrapTypeDescRefErr(id, err)
	}
	return errors.Wrapf(err, "referenced descriptor ID %d", id)
}

// build unmarshals and builds a descriptor from its value in the descriptor
// table.
func build(
	expectedType catalog.DescriptorType,
	id descpb.ID,
	rowValue *roachpb.Value,
	rawBytesInStorage []byte,
	isRequired bool,
) (catalog.Descriptor, error) {
	b, err := descbuilder.FromSerializedValue(rowValue)
	if err != nil {
		return nil, err
	}
	if b == nil {
		if isRequired {
			return nil, requiredError(expectedType, id)
		}
		return nil, nil
	}
	if expectedType != catalog.Any && b.DescriptorType() != expectedType {
		return nil, pgerror.Newf(pgcode.WrongObjectType, "descriptor is a %s", b.DescriptorType())
	}
	b.SetRawBytesInStorage(rawBytesInStorage)
	desc := b.BuildImmutable()
	if id != desc.GetID() {
		return nil, errors.AssertionFailedf("unexpected ID %d in descriptor", desc.GetID())
	}
	return desc, nil
}

// requiredError returns an appropriate error when a descriptor which was
// required was not found.
func requiredError(expectedType catalog.DescriptorType, id descpb.ID) (err error) {
	switch expectedType {
	case catalog.Table:
		err = sqlerrors.NewUndefinedRelationError(&tree.TableRef{TableID: int64(id)})
	case catalog.Database:
		err = sqlerrors.NewUndefinedDatabaseError(fmt.Sprintf("[%d]", id))
	case catalog.Schema:
		err = sqlerrors.NewUndefinedSchemaError(fmt.Sprintf("[%d]", id))
	case catalog.Type:
		err = sqlerrors.NewUndefinedTypeError(tree.NewUnqualifiedTypeName(fmt.Sprintf("[%d]", id)))
	default:
		err = errors.Errorf("failed to find descriptor [%d]", id)
	}
	return errors.CombineErrors(catalog.NewDescriptorNotFoundError(id), err)
}
