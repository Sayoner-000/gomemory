package native

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// T019 — código: firmas y declaraciones literales, cuerpos largos con marcador.
func TestCompressGoKeepsSignatures(t *testing.T) {
	in := readCorpus(t, "sample.go")
	out, n, ok := compressGo(in, "abc123abc123", maxTh)
	if !ok || n == 0 {
		t.Fatalf("compressGo ok=%v n=%d", ok, n)
	}
	if err := checkLiteral(in, out); err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", in, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			line := strings.SplitN(in[fset.Position(x.Pos()).Offset:], "\n", 2)[0]
			if !strings.Contains(out, line) {
				t.Errorf("falta la firma: %s", line)
			}
			if x.Doc != nil && !strings.Contains(out, x.Doc.Text()[:min(30, len(x.Doc.Text()))]) {
				// Doc.Text() normaliza; basta con el comienzo.
				t.Errorf("falta el doc de %s", x.Name.Name)
			}
		case *ast.GenDecl:
			if x.Tok == token.TYPE || x.Tok == token.CONST {
				line := strings.SplitN(in[fset.Position(x.Pos()).Offset:], "\n", 2)[0]
				if !strings.Contains(out, line) {
					t.Errorf("falta la declaración: %s", line)
				}
			}
		}
	}
}

func TestCompressGoShortBodyIntact(t *testing.T) {
	in := "package p\n\nfunc Corta() int {\n\treturn 1\n}\n"
	if out, n, _ := compressGo(in, "r", maxTh); n != 0 || out != in {
		t.Errorf("una función corta debe quedar intacta:\n%s", out)
	}
}

func TestCompressHeuristicLanguages(t *testing.T) {
	cases := map[string]func(string, string) (string, int, bool){
		"sample.py":   func(c, r string) (string, int, bool) { return compressPython(c, r, maxTh) },
		"sample.ts":   func(c, r string) (string, int, bool) { return compressBraces(c, r, maxTh) },
		"sample.java": func(c, r string) (string, int, bool) { return compressBraces(c, r, maxTh) },
	}
	for name, fn := range cases {
		in := readCorpus(t, name)
		out, n, ok := fn(in, "abc123abc123")
		if !ok || n == 0 {
			t.Errorf("%s: ok=%v n=%d", name, ok, n)
			continue
		}
		if err := checkLiteral(in, out); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		// Cabeceras y documentación se conservan.
		for _, l := range strings.Split(in, "\n") {
			tl := strings.TrimSpace(l)
			if strings.HasPrefix(tl, "def ") || strings.HasPrefix(tl, "public double ") || strings.HasPrefix(tl, "async ") ||
				strings.HasPrefix(tl, `"""`) || strings.HasPrefix(tl, "/**") {
				if !strings.Contains(out, l) {
					t.Errorf("%s: falta la línea %q", name, tl)
				}
			}
		}
	}
}

func TestCompressBracesUnbalanced(t *testing.T) {
	in := "function f() {\n" + strings.Repeat("  x++;\n", 10) // sin cerrar
	if _, _, ok := compressBraces(in, "r", maxTh); ok {
		t.Error("llaves desbalanceadas deben dar ok=false")
	}
}

func TestCompressSQLInserts(t *testing.T) {
	in := "CREATE TABLE t (id INT, v TEXT);\n" + strings.Repeat("INSERT INTO t VALUES (1, 'x');\n", 20) + "SELECT * FROM t;\n"
	out, n, ok := compressSQL(in, "abc", maxTh)
	if !ok || n != 18 || !strings.Contains(out, "CREATE TABLE t (id INT, v TEXT);") {
		t.Errorf("SQL: n=%d\n%s", n, out)
	}
	if err := checkLiteral(in, out); err != nil {
		t.Error(err)
	}
}
