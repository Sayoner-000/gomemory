package usecases

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"

	"mem/domain"
)

// parsedCall es una llamada pendiente de resolver contra el resto del
// proyecto en la segunda pasada del indexador (necesita que TODOS los
// archivos ya estén persistidos para resolver llamadas cruzadas entre
// archivos/paquetes).
type parsedCall struct {
	// CallerIndex indexa parsedFile.Nodes: el nodo función/método que
	// contiene la llamada.
	CallerIndex int
	// Ident es el identificador llamado directamente (ej. "Bar()") — vacío
	// si la llamada es un selector (pkg.Func()).
	Ident string
	// SelPkgAlias/SelName son el alias de paquete y el nombre del selector
	// para llamadas tipo pkg.Func() — ambos vacíos si no es un selector.
	SelPkgAlias string
	SelName     string
}

// parsedFile es el resultado de parsear un archivo .go: los nodos que
// declara (el nodo de archivo en el índice 0, luego funciones/métodos/tipos
// en el mismo orden en que deben insertarse), sus imports, y las llamadas
// pendientes de resolver.
type parsedFile struct {
	Nodes   []domain.CodeNode
	Package string
	Imports map[string]string // alias -> import path
	Calls   []parsedCall
}

// parseGoFile extrae la estructura de un archivo .go: package, imports,
// símbolos top-level (funciones, métodos, tipos) y referencias de llamadas
// dentro de sus cuerpos. No hace type-checking: la resolución de llamadas es
// best-effort (mismo paquete o vía el import map del archivo).
func parseGoFile(relPath string, src []byte) (parsedFile, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, relPath, src, parser.ParseComments)
	if err != nil {
		return parsedFile{}, err
	}

	pf := parsedFile{
		Package: file.Name.Name,
		Imports: map[string]string{},
	}
	pf.Nodes = append(pf.Nodes, domain.CodeNode{
		Kind: domain.NodeFile,
		Name: relPath,
	})

	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		alias := importAlias(imp, path)
		pf.Imports[alias] = path
	}

	// Índice del nodo función/método actual mientras se recorren los cuerpos,
	// para asociar cada llamada encontrada con quién la hace.
	funcNodeIndex := map[*ast.FuncDecl]int{}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			node := funcDeclNode(fset, pf.Package, d)
			pf.Nodes = append(pf.Nodes, node)
			funcNodeIndex[d] = len(pf.Nodes) - 1
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				pf.Nodes = append(pf.Nodes, typeSpecNode(fset, pf.Package, ts))
			}
		}
	}

	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		callerIdx, ok := funcNodeIndex[fd]
		if !ok {
			continue
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				pf.Calls = append(pf.Calls, parsedCall{CallerIndex: callerIdx, Ident: fn.Name})
			case *ast.SelectorExpr:
				if pkgIdent, ok := fn.X.(*ast.Ident); ok {
					pf.Calls = append(pf.Calls, parsedCall{
						CallerIndex: callerIdx,
						SelPkgAlias: pkgIdent.Name,
						SelName:     fn.Sel.Name,
					})
				}
			}
			return true
		})
	}

	return pf, nil
}

func importAlias(imp *ast.ImportSpec, path string) string {
	if imp.Name != nil {
		return imp.Name.Name
	}
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func funcDeclNode(fset *token.FileSet, pkg string, fd *ast.FuncDecl) domain.CodeNode {
	name := fd.Name.Name
	kind := domain.NodeFunction
	receiver := ""
	if fd.Recv != nil && len(fd.Recv.List) > 0 {
		kind = domain.NodeMethod
		receiver = receiverTypeName(fd.Recv.List[0].Type)
		if receiver != "" {
			name = receiver + "." + name
		}
	}
	return domain.CodeNode{
		Kind:      kind,
		Name:      name,
		Package:   pkg,
		Receiver:  receiver,
		Signature: printNode(fset, fd.Type),
		StartLine: fset.Position(fd.Pos()).Line,
		EndLine:   fset.Position(fd.End()).Line,
		Exported:  fd.Name.IsExported(),
	}
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return ""
	}
}

func typeSpecNode(fset *token.FileSet, pkg string, ts *ast.TypeSpec) domain.CodeNode {
	return domain.CodeNode{
		Kind:      domain.NodeType,
		Name:      ts.Name.Name,
		Package:   pkg,
		Signature: printNode(fset, ts.Type),
		StartLine: fset.Position(ts.Pos()).Line,
		EndLine:   fset.Position(ts.End()).Line,
		Exported:  ts.Name.IsExported(),
	}
}

func printNode(fset *token.FileSet, n ast.Node) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, n); err != nil {
		return ""
	}
	s := buf.String()
	// Firmas largas (structs/interfaces grandes) se truncan: solo sirven de
	// referencia rápida, el detalle real está en el archivo.
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}
