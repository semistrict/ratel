// Copyright 2024 Oxide Computer Company
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

package tree

// FuncParam represents a parameter in a CREATE FUNCTION statement.
type FuncParam struct {
	Name string
	Type ResolvableTypeReference
}

// CreateFunction represents a CREATE FUNCTION statement.
type CreateFunction struct {
	Name       Name
	Params     []FuncParam
	ReturnType ResolvableTypeReference
	Language   string
	Body       string // WAT source text
	Volatility Volatility
}

// Format implements the NodeFormatter interface.
func (n *CreateFunction) Format(ctx *FmtCtx) {
	ctx.WriteString("CREATE FUNCTION ")
	ctx.FormatNode(&n.Name)
	ctx.WriteString("(")
	for i, p := range n.Params {
		if i > 0 {
			ctx.WriteString(", ")
		}
		if p.Name != "" {
			ctx.WriteString(p.Name)
			ctx.WriteString(" ")
		}
		ctx.FormatTypeReference(p.Type)
	}
	ctx.WriteString(") RETURNS ")
	ctx.FormatTypeReference(n.ReturnType)
	ctx.WriteString(" LANGUAGE ")
	ctx.WriteString(n.Language)
	ctx.WriteString(" AS ")
	ctx.WriteString("'")
	ctx.WriteString(n.Body)
	ctx.WriteString("'")
	if n.Volatility == VolatilityImmutable {
		ctx.WriteString(" IMMUTABLE")
	}
}

func (n *CreateFunction) String() string { return AsString(n) }

// StatementReturnType implements the Statement interface.
func (*CreateFunction) StatementReturnType() StatementReturnType { return DDL }

// StatementType implements the Statement interface.
func (*CreateFunction) StatementType() StatementType { return TypeDDL }

// StatementTag returns a short string identifying the type of statement.
func (*CreateFunction) StatementTag() string { return "CREATE FUNCTION" }

// DropFunction represents a DROP FUNCTION statement.
type DropFunction struct {
	Name     Name
	Params   []FuncParam
	IfExists bool
}

// Format implements the NodeFormatter interface.
func (n *DropFunction) Format(ctx *FmtCtx) {
	ctx.WriteString("DROP FUNCTION ")
	if n.IfExists {
		ctx.WriteString("IF EXISTS ")
	}
	ctx.FormatNode(&n.Name)
	ctx.WriteString("(")
	for i, p := range n.Params {
		if i > 0 {
			ctx.WriteString(", ")
		}
		if p.Name != "" {
			ctx.WriteString(p.Name)
			ctx.WriteString(" ")
		}
		ctx.FormatTypeReference(p.Type)
	}
	ctx.WriteString(")")
}

func (n *DropFunction) String() string { return AsString(n) }

// StatementReturnType implements the Statement interface.
func (*DropFunction) StatementReturnType() StatementReturnType { return DDL }

// StatementType implements the Statement interface.
func (*DropFunction) StatementType() StatementType { return TypeDDL }

// StatementTag returns a short string identifying the type of statement.
func (*DropFunction) StatementTag() string { return "DROP FUNCTION" }
