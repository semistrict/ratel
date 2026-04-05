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

package udfruntime

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Wat2Wasm converts WebAssembly Text format (WAT) to binary WASM.
// It supports a practical subset of WAT: module, func (with inline export,
// params, results, locals), and numeric instructions (i32/i64/f64).
func Wat2Wasm(wat string) ([]byte, error) {
	tokens, err := tokenize(wat)
	if err != nil {
		return nil, fmt.Errorf("wat tokenize: %w", err)
	}
	exprs, err := parseSExprs(tokens)
	if err != nil {
		return nil, fmt.Errorf("wat parse: %w", err)
	}
	if len(exprs) != 1 {
		return nil, fmt.Errorf("wat: expected exactly one top-level module, got %d expressions", len(exprs))
	}
	mod, err := parseModule(exprs[0])
	if err != nil {
		return nil, fmt.Errorf("wat module: %w", err)
	}
	return encodeWasm(mod)
}

// --- Tokenizer ---

type tokenKind int

const (
	tokLParen  tokenKind = iota // (
	tokRParen                   // )
	tokKeyword                  // module, func, i64.add, etc.
	tokString                   // "invoke"
	tokInt                      // 42, -1, 0xFF
	tokFloat                    // 3.14, 1e10
	tokID                       // $name
)

type token struct {
	kind tokenKind
	text string
	pos  int
}

func tokenize(input string) ([]token, error) {
	var tokens []token
	i := 0
	for i < len(input) {
		ch := input[i]

		// Skip whitespace.
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			i++
			continue
		}

		// Line comment: ;; to end of line.
		if i+1 < len(input) && ch == ';' && input[i+1] == ';' {
			for i < len(input) && input[i] != '\n' {
				i++
			}
			continue
		}

		// Block comment: (; ... ;) (nestable).
		if i+1 < len(input) && ch == '(' && input[i+1] == ';' {
			depth := 1
			i += 2
			for i+1 < len(input) && depth > 0 {
				if input[i] == '(' && input[i+1] == ';' {
					depth++
					i += 2
				} else if input[i] == ';' && input[i+1] == ')' {
					depth--
					i += 2
				} else {
					i++
				}
			}
			if depth > 0 {
				return nil, fmt.Errorf("unterminated block comment at position %d", i)
			}
			continue
		}

		if ch == '(' {
			tokens = append(tokens, token{kind: tokLParen, text: "(", pos: i})
			i++
			continue
		}
		if ch == ')' {
			tokens = append(tokens, token{kind: tokRParen, text: ")", pos: i})
			i++
			continue
		}

		// String literal.
		if ch == '"' {
			start := i
			i++ // skip opening quote
			var sb strings.Builder
			for i < len(input) && input[i] != '"' {
				if input[i] == '\\' && i+1 < len(input) {
					i++ // skip backslash
					switch input[i] {
					case 'n':
						sb.WriteByte('\n')
					case 't':
						sb.WriteByte('\t')
					case '\\':
						sb.WriteByte('\\')
					case '"':
						sb.WriteByte('"')
					default:
						sb.WriteByte(input[i])
					}
				} else {
					sb.WriteByte(input[i])
				}
				i++
			}
			if i >= len(input) {
				return nil, fmt.Errorf("unterminated string at position %d", start)
			}
			i++ // skip closing quote
			tokens = append(tokens, token{kind: tokString, text: sb.String(), pos: start})
			continue
		}

		// ID: $name
		if ch == '$' {
			start := i
			i++
			for i < len(input) && isIDChar(input[i]) {
				i++
			}
			tokens = append(tokens, token{kind: tokID, text: input[start:i], pos: start})
			continue
		}

		// Number or keyword.
		if isIDChar(ch) || ch == '+' || ch == '-' {
			start := i
			// Check for sign followed by digit (number) vs keyword.
			if (ch == '+' || ch == '-') && i+1 < len(input) && (input[i+1] >= '0' && input[i+1] <= '9') {
				// Signed number.
				i++
			}
			for i < len(input) && isIDChar(input[i]) {
				i++
			}
			text := input[start:i]
			kind := classifyToken(text)
			tokens = append(tokens, token{kind: kind, text: text, pos: start})
			continue
		}

		return nil, fmt.Errorf("unexpected character %q at position %d", ch, i)
	}
	return tokens, nil
}

func isIDChar(ch byte) bool {
	return ch >= 'a' && ch <= 'z' ||
		ch >= 'A' && ch <= 'Z' ||
		ch >= '0' && ch <= '9' ||
		ch == '_' || ch == '.' || ch == ':' || ch == '/' || ch == '-' || ch == '+'
}

func classifyToken(text string) tokenKind {
	if len(text) == 0 {
		return tokKeyword
	}
	// Check if it's a number.
	start := 0
	if text[0] == '+' || text[0] == '-' {
		start = 1
	}
	if start < len(text) && text[start] >= '0' && text[start] <= '9' {
		isFloat := false
		for _, ch := range text[start:] {
			if ch == '.' || ch == 'e' || ch == 'E' || ch == 'p' || ch == 'P' {
				isFloat = true
				break
			}
		}
		if isFloat {
			return tokFloat
		}
		return tokInt
	}
	// Special float keywords.
	lower := strings.ToLower(text)
	if lower == "inf" || lower == "nan" {
		return tokFloat
	}
	return tokKeyword
}

// --- S-expression parser ---

// sexpr is either an atom (token) or a list of sexprs.
type sexpr struct {
	// If list is non-nil, this is a list. Otherwise it's an atom.
	list   []sexpr
	atom   token
	isList bool
}

func parseSExprs(tokens []token) ([]sexpr, error) {
	var result []sexpr
	i := 0
	for i < len(tokens) {
		expr, next, err := parseSExpr(tokens, i)
		if err != nil {
			return nil, err
		}
		result = append(result, expr)
		i = next
	}
	return result, nil
}

func parseSExpr(tokens []token, pos int) (sexpr, int, error) {
	if pos >= len(tokens) {
		return sexpr{}, pos, fmt.Errorf("unexpected end of input")
	}
	tok := tokens[pos]
	if tok.kind == tokLParen {
		// Parse list until matching RParen.
		var items []sexpr
		pos++
		for pos < len(tokens) && tokens[pos].kind != tokRParen {
			item, next, err := parseSExpr(tokens, pos)
			if err != nil {
				return sexpr{}, 0, err
			}
			items = append(items, item)
			pos = next
		}
		if pos >= len(tokens) {
			return sexpr{}, 0, fmt.Errorf("unmatched '(' at position %d", tok.pos)
		}
		pos++ // skip RParen
		return sexpr{list: items, isList: true}, pos, nil
	}
	if tok.kind == tokRParen {
		return sexpr{}, pos, fmt.Errorf("unexpected ')' at position %d", tok.pos)
	}
	return sexpr{atom: tok}, pos + 1, nil
}

func (s sexpr) keyword() string {
	if !s.isList && s.atom.kind == tokKeyword {
		return s.atom.text
	}
	return ""
}

func (s sexpr) isKeyword(kw string) bool {
	return s.keyword() == kw
}

func (s sexpr) intVal() (int64, error) {
	text := s.atom.text
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") ||
		strings.HasPrefix(text, "-0x") || strings.HasPrefix(text, "-0X") ||
		strings.HasPrefix(text, "+0x") || strings.HasPrefix(text, "+0X") {
		// Hex integer.
		return strconv.ParseInt(strings.Replace(text, "0x", "", 1), 16, 64)
	}
	return strconv.ParseInt(text, 10, 64)
}

func (s sexpr) floatVal() (float64, error) {
	lower := strings.ToLower(s.atom.text)
	switch lower {
	case "inf":
		return math.Inf(1), nil
	case "-inf":
		return math.Inf(-1), nil
	case "nan":
		return math.NaN(), nil
	}
	return strconv.ParseFloat(s.atom.text, 64)
}

// --- Module parser ---

type ValType byte

const (
	ValI32 ValType = 0x7F
	ValI64 ValType = 0x7E
	ValF64 ValType = 0x7C

	// JS-only types (not valid WASM value types, used for JS UDF marshaling).
	ValString    ValType = 0x01
	ValBytes     ValType = 0x02
	ValTimestamp ValType = 0x03
	ValJSON      ValType = 0x04
)

type watFunc struct {
	name       string // $name if named
	exportName string // inline (export "name")
	params     []watParam
	results    []ValType
	locals     []watLocal
	body       []sexpr // raw instruction sexprs
}

type watParam struct {
	name string // $name or ""
	typ  ValType
}

type watLocal struct {
	name string
	typ  ValType
}

type watModule struct {
	funcs []watFunc
}

func parseValType(s string) (ValType, error) {
	switch s {
	case "i32":
		return ValI32, nil
	case "i64":
		return ValI64, nil
	case "f64":
		return ValF64, nil
	default:
		return 0, fmt.Errorf("unsupported value type %q (supported: i32, i64, f64)", s)
	}
}

func parseModule(s sexpr) (*watModule, error) {
	if !s.isList || len(s.list) == 0 || !s.list[0].isKeyword("module") {
		return nil, fmt.Errorf("expected (module ...), got %v", s)
	}
	mod := &watModule{}
	for _, item := range s.list[1:] {
		if !item.isList || len(item.list) == 0 {
			continue
		}
		kw := item.list[0].keyword()
		switch kw {
		case "func":
			f, err := parseFunc(item.list[1:])
			if err != nil {
				return nil, fmt.Errorf("in func: %w", err)
			}
			mod.funcs = append(mod.funcs, f)
		default:
			return nil, fmt.Errorf("unsupported module item %q", kw)
		}
	}
	return mod, nil
}

func parseFunc(items []sexpr) (watFunc, error) {
	var f watFunc
	i := 0

	// Optional function name ($name).
	if i < len(items) && !items[i].isList && items[i].atom.kind == tokID {
		f.name = items[i].atom.text
		i++
	}

	// Parse structured items: (export ...), (param ...), (result ...), (local ...).
	for i < len(items) {
		if !items[i].isList {
			break // start of instruction body
		}
		inner := items[i].list
		if len(inner) == 0 {
			i++
			continue
		}
		kw := inner[0].keyword()
		switch kw {
		case "export":
			if len(inner) < 2 || inner[1].atom.kind != tokString {
				return f, fmt.Errorf("expected (export \"name\")")
			}
			f.exportName = inner[1].atom.text
		case "param":
			params, err := parseParams(inner[1:])
			if err != nil {
				return f, err
			}
			f.params = append(f.params, params...)
		case "result":
			for _, r := range inner[1:] {
				t, err := parseValType(r.keyword())
				if err != nil {
					return f, err
				}
				f.results = append(f.results, t)
			}
		case "local":
			locals, err := parseLocals(inner[1:])
			if err != nil {
				return f, err
			}
			f.locals = append(f.locals, locals...)
		default:
			// This is a folded instruction, not a structural item.
			// Put it back and break to instruction parsing.
			goto body
		}
		i++
	}
body:
	// Remaining items are instructions.
	f.body = items[i:]
	return f, nil
}

func parseParams(items []sexpr) ([]watParam, error) {
	var params []watParam
	i := 0
	for i < len(items) {
		if items[i].atom.kind == tokID {
			// Named param: $name type
			name := items[i].atom.text
			i++
			if i >= len(items) {
				return nil, fmt.Errorf("expected type after param name %s", name)
			}
			t, err := parseValType(items[i].keyword())
			if err != nil {
				return nil, err
			}
			params = append(params, watParam{name: name, typ: t})
		} else {
			// Anonymous param: type
			t, err := parseValType(items[i].keyword())
			if err != nil {
				return nil, err
			}
			params = append(params, watParam{typ: t})
		}
		i++
	}
	return params, nil
}

func parseLocals(items []sexpr) ([]watLocal, error) {
	var locals []watLocal
	i := 0
	for i < len(items) {
		if items[i].atom.kind == tokID {
			name := items[i].atom.text
			i++
			if i >= len(items) {
				return nil, fmt.Errorf("expected type after local name %s", name)
			}
			t, err := parseValType(items[i].keyword())
			if err != nil {
				return nil, err
			}
			locals = append(locals, watLocal{name: name, typ: t})
		} else {
			t, err := parseValType(items[i].keyword())
			if err != nil {
				return nil, err
			}
			locals = append(locals, watLocal{typ: t})
		}
		i++
	}
	return locals, nil
}

// --- Instruction encoding ---

type operandKind int

const (
	opNone  operandKind = iota // no immediate operand
	opU32                      // unsigned LEB128 index (local.get, call, br, etc.)
	opI32                      // signed LEB128 (i32.const)
	opI64                      // signed LEB128 (i64.const)
	opF64                      // 8-byte little-endian float (f64.const)
	opBlock                    // block type byte
)

type instrInfo struct {
	opcode  byte
	operand operandKind
}

//nolint:unused
var instrTable = map[string]instrInfo{
	// Control flow
	"unreachable": {0x00, opNone},
	"nop":         {0x01, opNone},
	"block":       {0x02, opBlock},
	"loop":        {0x03, opBlock},
	"if":          {0x04, opBlock},
	"else":        {0x05, opNone},
	"end":         {0x0B, opNone},
	"br":          {0x0C, opU32},
	"br_if":       {0x0D, opU32},
	"return":      {0x0F, opNone},
	"call":        {0x10, opU32},

	// Parametric
	"drop":   {0x1A, opNone},
	"select": {0x1B, opNone},

	// Variable access
	"local.get":  {0x20, opU32},
	"local.set":  {0x21, opU32},
	"local.tee":  {0x22, opU32},
	"global.get": {0x23, opU32},
	"global.set": {0x24, opU32},

	// Constants
	"i32.const": {0x41, opI32},
	"i64.const": {0x42, opI64},
	"f64.const": {0x44, opF64},

	// i32 comparison
	"i32.eqz":  {0x45, opNone},
	"i32.eq":   {0x46, opNone},
	"i32.ne":   {0x47, opNone},
	"i32.lt_s": {0x48, opNone},
	"i32.lt_u": {0x49, opNone},
	"i32.gt_s": {0x4A, opNone},
	"i32.gt_u": {0x4B, opNone},
	"i32.le_s": {0x4C, opNone},
	"i32.le_u": {0x4D, opNone},
	"i32.ge_s": {0x4E, opNone},
	"i32.ge_u": {0x4F, opNone},

	// i64 comparison
	"i64.eqz":  {0x50, opNone},
	"i64.eq":   {0x51, opNone},
	"i64.ne":   {0x52, opNone},
	"i64.lt_s": {0x53, opNone},
	"i64.lt_u": {0x54, opNone},
	"i64.gt_s": {0x55, opNone},
	"i64.gt_u": {0x56, opNone},
	"i64.le_s": {0x57, opNone},
	"i64.le_u": {0x58, opNone},
	"i64.ge_s": {0x59, opNone},
	"i64.ge_u": {0x5A, opNone},

	// f64 comparison
	"f64.eq": {0x61, opNone},
	"f64.ne": {0x62, opNone},
	"f64.lt": {0x63, opNone},
	"f64.gt": {0x64, opNone},
	"f64.le": {0x65, opNone},
	"f64.ge": {0x66, opNone},

	// i32 arithmetic
	"i32.clz":    {0x67, opNone},
	"i32.ctz":    {0x68, opNone},
	"i32.popcnt": {0x69, opNone},
	"i32.add":    {0x6A, opNone},
	"i32.sub":    {0x6B, opNone},
	"i32.mul":    {0x6C, opNone},
	"i32.div_s":  {0x6D, opNone},
	"i32.div_u":  {0x6E, opNone},
	"i32.rem_s":  {0x6F, opNone},
	"i32.rem_u":  {0x70, opNone},
	"i32.and":    {0x71, opNone},
	"i32.or":     {0x72, opNone},
	"i32.xor":    {0x73, opNone},
	"i32.shl":    {0x74, opNone},
	"i32.shr_s":  {0x75, opNone},
	"i32.shr_u":  {0x76, opNone},
	"i32.rotl":   {0x77, opNone},
	"i32.rotr":   {0x78, opNone},

	// i64 arithmetic
	"i64.clz":    {0x79, opNone},
	"i64.ctz":    {0x7A, opNone},
	"i64.popcnt": {0x7B, opNone},
	"i64.add":    {0x7C, opNone},
	"i64.sub":    {0x7D, opNone},
	"i64.mul":    {0x7E, opNone},
	"i64.div_s":  {0x7F, opNone},
	"i64.div_u":  {0x80, opNone},
	"i64.rem_s":  {0x81, opNone},
	"i64.rem_u":  {0x82, opNone},
	"i64.and":    {0x83, opNone},
	"i64.or":     {0x84, opNone},
	"i64.xor":    {0x85, opNone},
	"i64.shl":    {0x86, opNone},
	"i64.shr_s":  {0x87, opNone},
	"i64.shr_u":  {0x88, opNone},
	"i64.rotl":   {0x89, opNone},
	"i64.rotr":   {0x8A, opNone},

	// f64 arithmetic
	"f64.abs":      {0x99, opNone},
	"f64.neg":      {0x9A, opNone},
	"f64.ceil":     {0x9B, opNone},
	"f64.floor":    {0x9C, opNone},
	"f64.trunc":    {0x9D, opNone},
	"f64.nearest":  {0x9E, opNone},
	"f64.sqrt":     {0x9F, opNone},
	"f64.add":      {0xA0, opNone},
	"f64.sub":      {0xA1, opNone},
	"f64.mul":      {0xA2, opNone},
	"f64.div":      {0xA3, opNone},
	"f64.min":      {0xA4, opNone},
	"f64.max":      {0xA5, opNone},
	"f64.copysign": {0xA6, opNone},

	// Conversions
	"i32.wrap_i64":        {0xA7, opNone},
	"i32.trunc_f64_s":     {0xAA, opNone},
	"i32.trunc_f64_u":     {0xAB, opNone},
	"i64.extend_i32_s":    {0xAC, opNone},
	"i64.extend_i32_u":    {0xAD, opNone},
	"i64.trunc_f64_s":     {0xB0, opNone},
	"i64.trunc_f64_u":     {0xB1, opNone},
	"f64.convert_i32_s":   {0xB7, opNone},
	"f64.convert_i32_u":   {0xB8, opNone},
	"f64.convert_i64_s":   {0xB9, opNone},
	"f64.convert_i64_u":   {0xBA, opNone},
	"i64.reinterpret_f64": {0xBD, opNone},
	"f64.reinterpret_i64": {0xBF, opNone},
}

// funcEncoder encodes instructions for a single function.
type funcEncoder struct {
	nameMap map[string]uint32 // $name -> local/func index
	buf     []byte
}

func newFuncEncoder(f *watFunc, mod *watModule) *funcEncoder {
	enc := &funcEncoder{
		nameMap: make(map[string]uint32),
	}
	// Build name map: params first, then locals.
	idx := uint32(0)
	for _, p := range f.params {
		if p.name != "" {
			enc.nameMap[p.name] = idx
		}
		idx++
	}
	for _, l := range f.locals {
		if l.name != "" {
			enc.nameMap[l.name] = idx
		}
		idx++
	}
	// Map function names.
	for i, fn := range mod.funcs {
		if fn.name != "" {
			enc.nameMap[fn.name] = uint32(i)
		}
	}
	return enc
}

func (e *funcEncoder) resolveIndex(s sexpr) (uint32, error) {
	if s.atom.kind == tokID {
		idx, ok := e.nameMap[s.atom.text]
		if !ok {
			return 0, fmt.Errorf("undefined name %s", s.atom.text)
		}
		return idx, nil
	}
	if s.atom.kind == tokInt {
		v, err := s.intVal()
		if err != nil {
			return 0, err
		}
		return uint32(v), nil
	}
	return 0, fmt.Errorf("expected index or $name, got %q", s.atom.text)
}

// encodeInstrs encodes a sequence of instructions (flat or folded).
func (e *funcEncoder) encodeInstrs(items []sexpr) error {
	i := 0
	for i < len(items) {
		item := items[i]
		if item.isList {
			// Folded instruction: (op args...)
			if err := e.encodeFolded(item); err != nil {
				return err
			}
			i++
			continue
		}

		kw := item.keyword()
		if kw == "" {
			return fmt.Errorf("expected instruction, got %q", item.atom.text)
		}

		info, ok := instrTable[kw]
		if !ok {
			return fmt.Errorf("unknown instruction %q", kw)
		}

		e.buf = append(e.buf, info.opcode)

		switch info.operand {
		case opNone:
			i++
		case opU32:
			i++
			if i >= len(items) {
				return fmt.Errorf("instruction %s requires an index operand", kw)
			}
			idx, err := e.resolveIndex(items[i])
			if err != nil {
				return fmt.Errorf("instruction %s: %w", kw, err)
			}
			e.buf = appendULEB128(e.buf, idx)
			i++
		case opI32:
			i++
			if i >= len(items) {
				return fmt.Errorf("instruction %s requires an i32 operand", kw)
			}
			v, err := items[i].intVal()
			if err != nil {
				return fmt.Errorf("instruction %s: %w", kw, err)
			}
			e.buf = appendSLEB128(e.buf, int64(int32(v)))
			i++
		case opI64:
			i++
			if i >= len(items) {
				return fmt.Errorf("instruction %s requires an i64 operand", kw)
			}
			v, err := items[i].intVal()
			if err != nil {
				return fmt.Errorf("instruction %s: %w", kw, err)
			}
			e.buf = appendSLEB128(e.buf, v)
			i++
		case opF64:
			i++
			if i >= len(items) {
				return fmt.Errorf("instruction %s requires an f64 operand", kw)
			}
			v, err := items[i].floatVal()
			if err != nil {
				return fmt.Errorf("instruction %s: %w", kw, err)
			}
			var b [8]byte
			binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
			e.buf = append(e.buf, b[:]...)
			i++
		case opBlock:
			// Block type: empty (0x40) for now. We don't support typed blocks.
			e.buf = append(e.buf, 0x40)
			i++
		}
	}
	return nil
}

// encodeFolded encodes a folded instruction like (i64.add (local.get 0) (local.get 1)).
func (e *funcEncoder) encodeFolded(s sexpr) error {
	if !s.isList || len(s.list) == 0 {
		return fmt.Errorf("empty folded instruction")
	}
	kw := s.list[0].keyword()
	if kw == "" {
		return fmt.Errorf("expected instruction keyword in folded form")
	}

	// Special handling for block-structured instructions.
	switch kw {
	case "if":
		return e.encodeFoldedIf(s.list[1:])
	case "block", "loop":
		return e.encodeFoldedBlock(kw, s.list[1:])
	}

	info, ok := instrTable[kw]
	if !ok {
		return fmt.Errorf("unknown instruction %q", kw)
	}

	// Separate immediate operand from sub-expressions.
	rest := s.list[1:]
	var immediate []sexpr
	var subExprs []sexpr

	switch info.operand {
	case opNone:
		subExprs = rest
	case opU32, opI32, opI64, opF64:
		// The first non-list item is the immediate operand.
		if len(rest) > 0 && !rest[0].isList {
			immediate = rest[:1]
			subExprs = rest[1:]
		} else {
			subExprs = rest
		}
	case opBlock:
		subExprs = rest
	}

	// Encode sub-expressions first (they push values onto the stack).
	for _, sub := range subExprs {
		if sub.isList {
			if err := e.encodeFolded(sub); err != nil {
				return err
			}
		} else {
			// Flat instruction within a folded context.
			if err := e.encodeInstrs([]sexpr{sub}); err != nil {
				return err
			}
		}
	}

	// Encode the instruction itself.
	e.buf = append(e.buf, info.opcode)
	if len(immediate) > 0 {
		switch info.operand {
		case opU32:
			idx, err := e.resolveIndex(immediate[0])
			if err != nil {
				return err
			}
			e.buf = appendULEB128(e.buf, idx)
		case opI32:
			v, err := immediate[0].intVal()
			if err != nil {
				return err
			}
			e.buf = appendSLEB128(e.buf, int64(int32(v)))
		case opI64:
			v, err := immediate[0].intVal()
			if err != nil {
				return err
			}
			e.buf = appendSLEB128(e.buf, v)
		case opF64:
			v, err := immediate[0].floatVal()
			if err != nil {
				return err
			}
			var b [8]byte
			binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
			e.buf = append(e.buf, b[:]...)
		}
	} else if info.operand == opBlock {
		e.buf = append(e.buf, 0x40)
	}

	return nil
}

func (e *funcEncoder) encodeFoldedIf(items []sexpr) error {
	// (if (condition) (then ...) (else ...))
	// Encode condition first.
	for _, item := range items {
		if item.isList && len(item.list) > 0 {
			kw := item.list[0].keyword()
			if kw == "then" || kw == "else" {
				continue
			}
		}
		// This is the condition.
		if item.isList {
			if err := e.encodeFolded(item); err != nil {
				return err
			}
		} else {
			if err := e.encodeInstrs([]sexpr{item}); err != nil {
				return err
			}
		}
	}
	// Emit if opcode with empty block type.
	e.buf = append(e.buf, 0x04, 0x40)

	// Encode then and else branches.
	for _, item := range items {
		if !item.isList || len(item.list) == 0 {
			continue
		}
		kw := item.list[0].keyword()
		if kw == "then" {
			if err := e.encodeInstrs(item.list[1:]); err != nil {
				return err
			}
		} else if kw == "else" {
			e.buf = append(e.buf, 0x05) // else
			if err := e.encodeInstrs(item.list[1:]); err != nil {
				return err
			}
		}
	}
	e.buf = append(e.buf, 0x0B) // end
	return nil
}

func (e *funcEncoder) encodeFoldedBlock(kw string, items []sexpr) error {
	info := instrTable[kw]
	e.buf = append(e.buf, info.opcode, 0x40) // block type: empty
	if err := e.encodeInstrs(items); err != nil {
		return err
	}
	e.buf = append(e.buf, 0x0B) // end
	return nil
}

// --- WASM binary encoder ---

// WASM section IDs.
const (
	sectionType     byte = 1
	sectionFunction byte = 3
	sectionExport   byte = 7
	sectionCode     byte = 10
)

func encodeWasm(mod *watModule) ([]byte, error) {
	var buf []byte

	// Magic number and version.
	buf = append(buf, 0x00, 0x61, 0x73, 0x6D) // \0asm
	buf = append(buf, 0x01, 0x00, 0x00, 0x00) // version 1

	// Type section: one type per function.
	typeSection := encodeTypeSection(mod)
	buf = appendSection(buf, sectionType, typeSection)

	// Function section: maps function index to type index.
	funcSection := encodeFuncSection(mod)
	buf = appendSection(buf, sectionFunction, funcSection)

	// Export section.
	exportSection := encodeExportSection(mod)
	if len(exportSection) > 0 {
		buf = appendSection(buf, sectionExport, exportSection)
	}

	// Code section.
	codeSection, err := encodeCodeSection(mod)
	if err != nil {
		return nil, err
	}
	buf = appendSection(buf, sectionCode, codeSection)

	return buf, nil
}

func encodeTypeSection(mod *watModule) []byte {
	var buf []byte
	buf = appendULEB128(buf, uint32(len(mod.funcs))) // count
	for _, f := range mod.funcs {
		buf = append(buf, 0x60) // func type
		// Params.
		buf = appendULEB128(buf, uint32(len(f.params)))
		for _, p := range f.params {
			buf = append(buf, byte(p.typ))
		}
		// Results.
		buf = appendULEB128(buf, uint32(len(f.results)))
		for _, r := range f.results {
			buf = append(buf, byte(r))
		}
	}
	return buf
}

func encodeFuncSection(mod *watModule) []byte {
	var buf []byte
	buf = appendULEB128(buf, uint32(len(mod.funcs)))
	for i := range mod.funcs {
		buf = appendULEB128(buf, uint32(i)) // type index = func index
	}
	return buf
}

func encodeExportSection(mod *watModule) []byte {
	// Count exports.
	var exports []struct {
		name string
		idx  int
	}
	for i, f := range mod.funcs {
		if f.exportName != "" {
			exports = append(exports, struct {
				name string
				idx  int
			}{f.exportName, i})
		}
	}
	if len(exports) == 0 {
		return nil
	}

	var buf []byte
	buf = appendULEB128(buf, uint32(len(exports)))
	for _, exp := range exports {
		buf = appendULEB128(buf, uint32(len(exp.name)))
		buf = append(buf, []byte(exp.name)...)
		buf = append(buf, 0x00) // func export
		buf = appendULEB128(buf, uint32(exp.idx))
	}
	return buf
}

func encodeCodeSection(mod *watModule) ([]byte, error) {
	var buf []byte
	buf = appendULEB128(buf, uint32(len(mod.funcs)))
	for _, f := range mod.funcs {
		body, err := encodeFuncBody(&f, mod)
		if err != nil {
			return nil, fmt.Errorf("encoding function %q: %w", funcName(&f), err)
		}
		buf = appendULEB128(buf, uint32(len(body)))
		buf = append(buf, body...)
	}
	return buf, nil
}

func funcName(f *watFunc) string {
	if f.exportName != "" {
		return f.exportName
	}
	if f.name != "" {
		return f.name
	}
	return "<anonymous>"
}

func encodeFuncBody(f *watFunc, mod *watModule) ([]byte, error) {
	var body []byte

	// Local declarations (compressed: count + type pairs).
	if len(f.locals) == 0 {
		body = appendULEB128(body, 0)
	} else {
		// Compress runs of same type.
		type localRun struct {
			count uint32
			typ   ValType
		}
		var runs []localRun
		for _, l := range f.locals {
			if len(runs) > 0 && runs[len(runs)-1].typ == l.typ {
				runs[len(runs)-1].count++
			} else {
				runs = append(runs, localRun{1, l.typ})
			}
		}
		body = appendULEB128(body, uint32(len(runs)))
		for _, r := range runs {
			body = appendULEB128(body, r.count)
			body = append(body, byte(r.typ))
		}
	}

	// Encode instructions.
	enc := newFuncEncoder(f, mod)
	if err := enc.encodeInstrs(f.body); err != nil {
		return nil, err
	}
	body = append(body, enc.buf...)

	// Implicit end.
	body = append(body, 0x0B)

	return body, nil
}

// --- LEB128 encoding ---

func appendULEB128(buf []byte, v uint32) []byte {
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if v == 0 {
			break
		}
	}
	return buf
}

func appendSLEB128(buf []byte, v int64) []byte {
	for {
		b := byte(v & 0x7F)
		v >>= 7
		// Check if we're done: the sign bit of the current byte matches the
		// remaining value.
		if (v == 0 && b&0x40 == 0) || (v == -1 && b&0x40 != 0) {
			buf = append(buf, b)
			break
		}
		buf = append(buf, b|0x80)
	}
	return buf
}

func appendSection(buf []byte, id byte, content []byte) []byte {
	buf = append(buf, id)
	buf = appendULEB128(buf, uint32(len(content)))
	buf = append(buf, content...)
	return buf
}
