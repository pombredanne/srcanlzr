// Copyright 2014-2015 The project AUTHORS. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"
)

// kind of AST node
const (
	Expression = iota
	Statement
	Other
)

// go source file header
const header = `// Copyright 2014-2015 The project AUTHORS. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// DO NOT EDIT: This source file has been generated by gen/gen_ast_decoder.go

package src

import (
	"github.com/DevMine/srcanlzr/src/ast"
	"github.com/DevMine/srcanlzr/src/token"
)

// decodeExprs decodes a list of expression objects.
func (dec *decoder) decodeExprs() []ast.Expr {
	if !dec.assertNewArray() {
		return nil
	}

	exprs := []ast.Expr{}

	if dec.isEmptyArray() {
		return exprs
	}
	if dec.err != nil {
		return nil
	}

	for {
		expr := dec.decodeExpr()
		if dec.err != nil {
			return nil
		}

		exprs = append(exprs, expr)

		if dec.isEndArray() {
			break
		}
		if dec.err != nil {
			return nil
		}
	}

	return exprs
}

// decodeStmts decodes a list of expression objects.
func (dec *decoder) decodeStmts() []ast.Stmt{
	if !dec.assertNewArray() {
		return nil
	}

	stmts := []ast.Stmt{}

	if dec.isEmptyArray() {
		return stmts
	}
	if dec.err != nil {
		return nil
	}

	for {
		stmt := dec.decodeStmt()
		if dec.err != nil {
			return nil
		}

		stmts = append(stmts, stmt)

		if dec.isEndArray() {
			break
		}
		if dec.err != nil {
			return nil
		}
	}

	return stmts
}

`

const tmplGenericExpr = `
func (dec *decoder) decodeExpr() ast.Expr {
	if !dec.assertNewObject() {
		return nil
	}

	// Expression cannot be an empty object because ast.Expr is an interface
	// and we need the value corresponding to the "expression_name" to allocate
	// the appropriate type.
	if dec.isEmptyObject() {
		dec.err = errors.New("expression object cannot be empty")
		return nil
	}
	if dec.err != nil {
		return nil
	}
	exprName := dec.extractFirstKey("expression_name")
	if dec.err != nil {
		return nil
	}

	// Since the beginning of the object has already been consumed, we need
	// special methods for only decoding the attributes.

	var expr ast.Expr
	switch exprName {
	{{ range $index, $expr := . }}
		case token.{{ $expr.Name }}Name:
			expr = dec.decode{{ $expr.Name }}Attrs()
	{{ end }}
	default:
		dec.err = fmt.Errorf("unknown expression '%s'", exprName)
		return nil
	}
	if dec.err != nil {
		return nil
	}
	return expr
}
`

const tmplGenericStmt = `
func (dec *decoder) decodeStmt() ast.Stmt {
	if !dec.assertNewObject() {
		return nil
	}

	// Expression cannot be an empty object because ast.Stmt is an interface
	// and we need the value corresponding to the "statement_name" to allocate
	// the appropriate type.
	if dec.isEmptyObject() {
		dec.err = errors.New("statement object cannot be empty")
		return nil
	}
	if dec.err != nil {
		return nil
	}
	stmtName := dec.extractFirstKey("statement_name")
	if dec.err != nil {
		return nil
	}

	// Since the beginning of the object has already been consumed, we need
	// special methods for only decoding the attributes.

	var stmt ast.Stmt
	switch stmtName {
	{{ range $index, $stmt := . }}
		case token.{{ $stmt.Name }}Name:
			stmt = dec.decode{{ $stmt.Name }}Attrs()
	{{ end }}
	default:
		dec.err = fmt.Errorf("unknown expression '%s'", stmtName)
		return nil
	}
	if dec.err != nil {
		return nil
	}
	return stmt
}
`

const tmplArray = `
func (dec *decoder) decode{{ .Name }}s() []*ast.{{ .Name }} {
	if !dec.assertNewArray() {
		return nil
	}

	a := []*ast.{{ .Name }}{}

	if dec.isEmptyArray() {
		return a
	}
	if dec.err != nil {
		return nil
	}

	for {
		elt := dec.decode{{ .Name }}()
		if dec.err != nil {
			return nil
		}

		a = append(a, elt)

		if dec.isEndArray() {
			break
		}
		if dec.err != nil {
			return nil
		}
	}

	return a
}
`

// template for expressions
const tmplExpr = `
func (dec *decoder) decode{{ .Name }}() *ast.{{ .Name }} {
	if !dec.assertNewObject() {
		return nil
	}
	if dec.isEmptyObject() {
		dec.err = errors.New("{{ .Name }} object cannot be empty")
		return nil
	}
	if dec.err != nil {
		return nil
	}
	return dec.decode{{ .Name }}Attrs()
}

func (dec *decoder) decode{{ .Name }}Attrs() *ast.{{ .Name }} {
	expr := ast.{{ .Name }}{ExprName: token.{{ .Name }}Name}
	for {
		key, err := dec.scan.nextKey()
		if err != nil {
			if err == io.EOF {
				break
			}
			dec.err = err
			return nil
		}
		if key == "" {
			dec.err = errors.New("empty key")
			return nil
		}

		{{ if .HasBasicType }}
		val, tok, err := dec.scan.nextValue()
		{{ else }}
		_, _, err = dec.scan.nextValue()
		{{ end }}
		if err != nil {
			dec.err = err
			return nil
		}

		switch key {
		{{ range $index, $field := .Fields }}
		case "{{ $field.JSONName }}":
			{{ if $field.BasicType }}
				if tok != scan{{ $field.Type }}Lit {
					dec.err = fmt.Errorf("expected '{{ $field.Type }} literal', found '%v'", tok)
					return nil
				}
				expr.{{ $field.Name }}, dec.err = dec.unmarshal{{ $field.Type }}(val)
			{{ else }}
				dec.scan.back()
				{{ if $field.Array }}
					expr.{{ $field.Name }} = dec.decode{{ $field.Type }}s()
				{{ else }}
					expr.{{ $field.Name }} = dec.decode{{ $field.Type }}()
				{{ end }}
			{{ end }}
		{{ end }}
		default:
			dec.err = fmt.Errorf("unexpected key '%s' for {{.Name}} object", key)
		}

		if dec.err != nil {
			return nil
		}

		if dec.isEndObject() {
			break
		}
		if err != nil {
			return nil
		}
	}
	return &expr
}

`

// template for statements
const tmplStmt = `
func (dec *decoder) decode{{ .Name }}() *ast.{{ .Name }} {
	if !dec.assertNewObject() {
		return nil
	}
	if dec.isEmptyObject() {
		dec.err = errors.New("{{ .Name }} object cannot be empty")
		return nil
	}
	if dec.err != nil {
		return nil
	}
	return dec.decode{{ .Name }}Attrs()
}

func (dec *decoder) decode{{ .Name }}Attrs() *ast.{{ .Name }} {
	stmt := ast.{{ .Name }}{StmtName: token.{{ .Name }}Name}
	for {
		key, err := dec.scan.nextKey()
		if err != nil {
			if err == io.EOF {
				break
			}
			dec.err = err
			return nil
		}
		if key == "" {
			dec.err = errors.New("empty key")
			return nil
		}

		{{ if .HasBasicType }}
		val, tok, err := dec.scan.nextValue()
		{{ else }}
		_, _, err = dec.scan.nextValue()
		{{ end }}
		if err != nil {
			dec.err = err
			return nil
		}

		switch key {
		{{ range $index, $field := .Fields }}
		case "{{ $field.JSONName }}":
			{{ if $field.BasicType }}
				if tok != scan{{ $field.Type }}Lit {
					dec.err = fmt.Errorf("expected '{{ $field.Type }} literal', found '%v'", tok)
					return nil
				}
				stmt.{{ $field.Name }}, dec.err = dec.unmarshal{{ $field.Type }}(val)
			{{ else }}
				dec.scan.back()
				{{ if $field.Array }}
					stmt.{{ $field.Name }} = dec.decode{{ $field.Type }}s()
				{{ else }}
					stmt.{{ $field.Name }} = dec.decode{{ $field.Type }}()
				{{ end }}
			{{ end }}
		{{ end }}
		default:
			dec.err = fmt.Errorf("unexpected key '%s' for {{.Name}} object", key)
		}

		if dec.err != nil {
			return nil
		}

		if dec.isEndObject() {
			break
		}
		if err != nil {
			return nil
		}
	}
	return &stmt
}

`

// template for other structures
const tmplOther = `
func (dec *decoder) decode{{ .Name }}() *ast.{{ .Name }} {
	if !dec.assertNewObject() {
		return nil
	}
	if dec.isEmptyObject() {
		return nil
	}
	if dec.err != nil {
		return nil
	}

	any := ast.{{ .Name }}{}
	for {
		key, err := dec.scan.nextKey()
		if err != nil {
			if err == io.EOF {
				break
			}
			dec.err = err
			return nil
		}
		if key == "" {
			dec.err = errors.New("empty key")
			return nil
		}

		{{ if .HasBasicType }}
		val, tok, err := dec.scan.nextValue()
		{{ else }}
		_, _, err = dec.scan.nextValue()
		{{ end }}
		if err != nil {
			dec.err = err
			return nil
		}

		switch key {
		{{ range $index, $field := .Fields }}
		case "{{ $field.JSONName }}":
			{{ if $field.BasicType }}
				if tok != scan{{ $field.Type }}Lit {
					dec.err = fmt.Errorf("expected '{{ $field.Type }} literal', found '%v'", tok)
					return nil
				}
				any.{{ $field.Name }}, dec.err = dec.unmarshal{{ $field.Type }}(val)
			{{ else }}
				dec.scan.back()
				{{ if $field.Array }}
					any.{{ $field.Name }} = dec.decode{{ $field.Type }}s()
				{{ else }}
					any.{{ $field.Name }} = dec.decode{{ $field.Type }}()
				{{ end }}
			{{ end }}
		{{ end }}
		default:
			dec.err = fmt.Errorf("unexpected key '%s' for {{.Name}} object", key)
		}

		if dec.err != nil {
			return nil
		}

		if dec.isEndObject() {
			break
		}
		if err != nil {
			return nil
		}
	}
	return &any
}

`

const outputPath = "decode_ast.gen.go"

type DecoderTmpl struct {
	Name   string
	Fields []Field
}

func (dt DecoderTmpl) HasBasicType() bool {
	for _, field := range dt.Fields {
		if field.BasicType {
			return true
		}
	}
	return false
}

type Field struct {
	Name      string
	JSONName  string
	Type      string
	BasicType bool
	Array     bool
}

func genArray(w io.Writer, dec []DecoderTmpl) error {
	if len(dec) == 0 {
		return nil
	}

	t := template.Must(template.New("array").Parse(tmplArray))

	for _, elt := range dec {
		if err := t.Execute(w, elt); err != nil {
			return err
		}
	}
	return nil
}

func genGenericExpr(w io.Writer, exprs []DecoderTmpl) error {
	if len(exprs) == 0 {
		return nil
	}

	t := template.Must(template.New("expression").Parse(tmplGenericExpr))
	return t.Execute(w, exprs)
}

func genExprs(w io.Writer, exprs []DecoderTmpl) error {
	if len(exprs) == 0 {
		return nil
	}

	if err := genGenericExpr(w, exprs); err != nil {
		return err
	}

	t := template.Must(template.New("expressions").Parse(tmplExpr))
	for _, expr := range exprs {
		if err := t.Execute(w, expr); err != nil {
			return err
		}
	}
	return genArray(w, exprs)
}

func genGenericStmt(w io.Writer, stmts []DecoderTmpl) error {
	if len(stmts) == 0 {
		return nil
	}

	t := template.Must(template.New("statement").Parse(tmplGenericStmt))
	return t.Execute(w, stmts)
}

func genStmts(w io.Writer, stmts []DecoderTmpl) error {
	if len(stmts) == 0 {
		return nil
	}

	if err := genGenericStmt(w, stmts); err != nil {
		return err
	}

	t := template.Must(template.New("statements").Parse(tmplStmt))
	for _, stmt := range stmts {
		if err := t.Execute(w, stmt); err != nil {
			return err
		}
	}
	return genArray(w, stmts)
}

func genOthers(w io.Writer, others []DecoderTmpl) error {
	if len(others) == 0 {
		return nil
	}

	t := template.Must(template.New("others").Parse(tmplOther))
	for _, other := range others {
		if err := t.Execute(w, other); err != nil {
			return err
		}
	}
	return genArray(w, others)
}

func extractType(field *ast.Field) (typ string, array bool, basicType bool) {
	switch s := field.Type.(type) {
	case *ast.StarExpr:
		if ident, ok := s.X.(*ast.Ident); ok {
			typ = ident.String()
		}
	case *ast.ArrayType:
		switch elt := s.Elt.(type) {
		case *ast.StarExpr:
			if ident, ok := elt.X.(*ast.Ident); ok {
				typ = ident.String()
				array = true
			}
		case *ast.Ident:
			tmp := elt.String()
			typ = strings.ToUpper(string(tmp[0])) + tmp[1:]
			array = true
		}
	case *ast.Ident:
		tmp := s.String()
		typ = strings.ToUpper(string(tmp[0])) + tmp[1:]

		switch typ {
		case "String", "Int64", "Float64", "Bool":
			basicType = true
		}
	}
	return
}

func extractTag(tag *ast.BasicLit) string {
	// extract JSON tag
	re := regexp.MustCompile("`json:\"([a-zA-Z0-9_]+)(,omitempty)?\"`")
	m := re.FindStringSubmatch(tag.Value)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func warn(a ...interface{}) {
	fmt.Fprintln(os.Stderr, a...)
}

func warnf(format string, a ...interface{}) {
	warn(fmt.Sprintf(format, a...) + "\n")
}

func fatal(a ...interface{}) {
	warn(a...)
	os.Exit(1)
}

func fatalf(format string, a ...interface{}) {
	warnf(format, a...)
	os.Exit(1)
}

func main() {
	flag.Parse()

	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, "./ast/ast.go", nil, 0)
	if err != nil {
		fatal(err)
	}

	exprs := []DecoderTmpl{}
	stmts := []DecoderTmpl{}
	others := []DecoderTmpl{}

	for _, decl := range f.Decls {
		var genDecl *ast.GenDecl
		var ok bool
		if genDecl, ok = decl.(*ast.GenDecl); !ok {
			continue
		}

		for _, spec := range genDecl.Specs {
			var typeSpec *ast.TypeSpec
			var ok bool
			if typeSpec, ok = spec.(*ast.TypeSpec); !ok {
				continue
			}

			dec := DecoderTmpl{Name: typeSpec.Name.String(), Fields: []Field{}}

			var structType *ast.StructType
			if structType, ok = typeSpec.Type.(*ast.StructType); !ok {
				continue
			}
			kind := Other
			for _, field := range structType.Fields.List {
				fieldTmpl := Field{}
				// XXX: handle compositions
				if len(field.Names) == 0 {
					continue
				}
				fieldTmpl.Name = field.Names[0].String()
				fieldTmpl.JSONName = extractTag(field.Tag)
				switch fieldTmpl.JSONName {
				case "expression_name":
					kind = Expression
				case "statement_name":
					kind = Statement
				}
				if fieldTmpl.Type, fieldTmpl.Array, fieldTmpl.BasicType = extractType(field); fieldTmpl.Type == "" {
					warnf("invalid type for %s.%s", dec.Name, fieldTmpl.Name)
					continue
				}
				dec.Fields = append(dec.Fields, fieldTmpl)
			}

			switch kind {
			case Expression:
				exprs = append(exprs, dec)
			case Statement:
				stmts = append(stmts, dec)
			default:
				others = append(others, dec)
			}
		}
	}

	buf := bytes.NewBufferString(header)

	// generate code
	if err := genExprs(buf, exprs); err != nil {
		fatal(err)
	}
	if err := genStmts(buf, stmts); err != nil {
		fatal(err)
	}
	if err := genOthers(buf, others); err != nil {
		fatal(err)
	}

	// format and write final source file
	bs, err := imports.Process(outputPath, buf.Bytes(), nil)
	if err != nil {
		fatal(err)
	}
	if err := ioutil.WriteFile(outputPath, bs, 0644); err != nil {
		fatal(err)
	}
}
