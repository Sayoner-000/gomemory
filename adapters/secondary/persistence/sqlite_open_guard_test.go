package persistence

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestSQLiteOpen_OnlyThroughSqliteDSN(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	count := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		alias := ""
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if path == "database/sql" {
				alias = "sql"
				if imp.Name != nil {
					alias = imp.Name.Name
				}
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Open" {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != alias || len(call.Args) < 2 {
				return true
			}
			driver, ok := call.Args[0].(*ast.BasicLit)
			if !ok {
				return true
			}
			value, err := strconv.Unquote(driver.Value)
			if err != nil || value != "sqlite" {
				return true
			}
			count++
			dsn, ok := call.Args[1].(*ast.CallExpr)
			valid := false
			if ok {
				fn, ok := dsn.Fun.(*ast.Ident)
				valid = ok && fn.Name == "sqliteDSN"
			}
			if !valid {
				t.Errorf("%s: SQLite opens must call sqliteDSN directly", fset.Position(call.Pos()))
			}
			return true
		})
	}
	if count == 0 {
		t.Fatal("guard found no SQLite opens")
	}
}
