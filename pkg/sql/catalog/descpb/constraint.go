// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package descpb

// ConstraintType is used to identify the type of a constraint.
type ConstraintType string

const (
	// ConstraintTypePK identifies a PRIMARY KEY constraint.
	ConstraintTypePK ConstraintType = "PRIMARY KEY"
	// ConstraintTypeFK identifies a FOREIGN KEY constraint.
	ConstraintTypeFK ConstraintType = "FOREIGN KEY"
	// ConstraintTypeUnique identifies a UNIQUE constraint.
	ConstraintTypeUnique ConstraintType = "UNIQUE"
	// ConstraintTypeCheck identifies a CHECK constraint.
	ConstraintTypeCheck ConstraintType = "CHECK"
)

// ConstraintDetail describes a constraint.
type ConstraintDetail struct {
	Kind         ConstraintType
	ConstraintID ConstraintID
	Columns      []string
	Details      string
	Unvalidated  bool

	// Only populated for PK and Unique Constraints with an index.
	Index *IndexDescriptor

	// Only populated for Unique Constraints without an index.
	UniqueWithoutIndexConstraint *UniqueWithoutIndexConstraint

	// Only populated for FK Constraints.
	FK              *ForeignKeyConstraint
	ReferencedTable *TableDescriptor

	// Only populated for Check Constraints.
	CheckConstraint *TableDescriptor_CheckConstraint
}
