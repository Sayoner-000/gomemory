package native

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"

	"mem/domain"
)

// compressGo conserva paquete, importaciones, tipos, constantes, variables,
// firmas y comentarios de documentación, y sustituye cada cuerpo de función de
// al menos CodeMinBodyLines líneas por un marcador (research.md R4). Usa
// go/parser de la biblioteca estándar. Un fragmento sin cláusula package se
// parsea envuelto y los desplazamientos se corrigen.
//
// ok=false significa que no es Go parseable: el llamador prueba la heurística
// genérica de llaves.
func compressGo(content, ref string, th domain.Thresholds) (string, int, bool) {
	src := content
	offset := 0
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		const wrap = "package p\n"
		file, err = parser.ParseFile(fset, "", wrap+src, parser.ParseComments)
		if err != nil {
			return "", 0, false
		}
		offset = len(wrap)
	}
	tf := fset.File(file.Pos())

	type cut struct{ from, to, lines int }
	var cuts []cut
	for _, d := range file.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		lb := tf.Offset(fn.Body.Lbrace) - offset
		rb := tf.Offset(fn.Body.Rbrace) - offset
		lines := tf.Line(fn.Body.Rbrace) - tf.Line(fn.Body.Lbrace) - 1
		if lines < th.CodeMinBodyLines {
			continue
		}
		cuts = append(cuts, cut{lb + 1, rb, lines})
	}
	if len(cuts) == 0 {
		return content, 0, true
	}
	sort.Slice(cuts, func(a, b int) bool { return cuts[a].from < cuts[b].from })

	var out strings.Builder
	prev := 0
	omitted := 0
	for _, c := range cuts {
		out.WriteString(content[prev:c.from])
		out.WriteString(" /* " + domain.RenderTextMarker(domain.Omission{Ref: ref, Summary: fmt.Sprintf("cuerpo omitido: %d líneas", c.lines)}) + " */ ")
		prev = c.to
		omitted += c.lines
	}
	out.WriteString(content[prev:])
	return out.String(), omitted, true
}
