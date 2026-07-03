package usecases

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// skippedDirs son directorios que el indexador nunca recorre: control de
// versiones, estado de gomemory, vendoring y artefactos de build. No hay
// soporte de .gitignore completo en Fase 1 — esta lista cubre los casos
// comunes de un proyecto Go.
var skippedDirs = map[string]bool{
	".git":         true,
	".memory":      true,
	".claude":      true,
	"vendor":       true,
	"node_modules": true,
	"dist":         true,
	"build":        true,
}

// IndexReport resume el resultado de una corrida del indexador.
type IndexReport struct {
	Scanned  int
	Parsed   int
	Skipped  int
	Deleted  int
	Nodes    int
	Edges    int
	Duration time.Duration
}

// Indexer construye y mantiene el grafo de código de un proyecto Go a partir
// de sus fuentes .go, usando go/parser (sin cgo, sin type-checking completo:
// la resolución de llamadas es best-effort).
type Indexer struct {
	Repo    ports.CodeGraphRepository
	Root    string
	Project string
}

func NewIndexer(repo ports.CodeGraphRepository, root, project string) *Indexer {
	return &Indexer{Repo: repo, Root: root, Project: project}
}

// IndexProject recorre todo el proyecto, indexa los archivos .go cambiados
// (o todos si force=true) y borra del grafo los que ya no existen en disco.
func (ix *Indexer) IndexProject(force bool) (IndexReport, error) {
	start := time.Now()

	paths, err := ix.walkGoFiles()
	if err != nil {
		return IndexReport{}, err
	}

	knownHashes, err := ix.Repo.FileHashes(ix.Project)
	if err != nil {
		return IndexReport{}, err
	}

	seen := make(map[string]bool, len(paths))
	var toIndex []string
	report := IndexReport{Scanned: len(paths)}

	for _, rel := range paths {
		seen[rel] = true
		hash, err := hashFile(filepath.Join(ix.Root, rel))
		if err != nil {
			report.Skipped++
			continue
		}
		if !force && knownHashes[rel] == hash {
			report.Skipped++
			continue
		}
		toIndex = append(toIndex, rel)
	}

	for known := range knownHashes {
		if !seen[known] {
			if err := ix.Repo.DeleteFile(ix.Project, known); err == nil {
				report.Deleted++
			}
		}
	}

	sub, err := ix.indexFilesInternal(toIndex)
	if err != nil {
		return report, err
	}
	report.Parsed = sub.Parsed
	report.Nodes = sub.Nodes
	report.Edges = sub.Edges
	report.Duration = time.Since(start)
	return report, nil
}

// IndexFiles reindexa incrementalmente solo los archivos dados (relativos a
// Root), usado por el hook turn-end tras cada turno del agente. A diferencia
// de IndexProject, no recorre el árbol ni borra archivos ausentes.
func (ix *Indexer) IndexFiles(relPaths []string) (IndexReport, error) {
	start := time.Now()
	report, err := ix.indexFilesInternal(relPaths)
	report.Duration = time.Since(start)
	return report, err
}

func (ix *Indexer) indexFilesInternal(relPaths []string) (IndexReport, error) {
	var report IndexReport
	if len(relPaths) == 0 {
		return report, nil
	}

	type fileResult struct {
		path    string
		nodes   []domain.CodeNode
		parsed  parsedFile
	}
	var results []fileResult

	for _, rel := range relPaths {
		abs := filepath.Join(ix.Root, rel)
		src, err := os.ReadFile(abs)
		if err != nil {
			continue // archivo desapareció entre el walk y el parseo: se ignora, IndexProject lo detecta luego
		}
		hash := sha256Hex(src)

		pf, err := parseGoFile(rel, src)
		if err != nil {
			continue // Go inválido/no parseable: se omite sin abortar el resto del índice
		}

		inserted, err := ix.Repo.ReplaceFile(ix.Project, rel, hash, pf.Nodes)
		if err != nil {
			return report, err
		}
		report.Parsed++
		report.Nodes += len(inserted)
		results = append(results, fileResult{path: rel, nodes: inserted, parsed: pf})
	}

	// Segunda pasada: recién ahora TODOS los archivos tocados están
	// persistidos, así que se puede resolver llamadas/imports cruzados entre
	// archivos consultando el grafo ya escrito.
	for _, r := range results {
		edges := ix.resolveEdges(r.path, r.nodes, r.parsed)
		if len(edges) == 0 {
			continue
		}
		if err := ix.Repo.InsertEdges(ix.Project, r.path, edges); err != nil {
			return report, err
		}
		report.Edges += len(edges)
	}

	return report, nil
}

func (ix *Indexer) resolveEdges(path string, nodes []domain.CodeNode, pf parsedFile) []domain.CodeEdge {
	if len(nodes) == 0 {
		return nil
	}
	fileNode := nodes[0] // parseGoFile siempre emite el nodo de archivo primero
	var edges []domain.CodeEdge

	for _, n := range nodes[1:] {
		edges = append(edges, domain.CodeEdge{FromID: fileNode.ID, ToID: n.ID, Kind: domain.EdgeDefines, Confidence: 1.0})
	}

	for _, importPath := range pf.Imports {
		pkgNode, err := ix.Repo.UpsertPackageNode(ix.Project, importPath)
		if err != nil {
			continue
		}
		edges = append(edges, domain.CodeEdge{FromID: fileNode.ID, ToID: pkgNode.ID, Kind: domain.EdgeImports, Confidence: 1.0})
	}

	callerPkg := pf.Package
	for _, call := range pf.Calls {
		if call.CallerIndex < 0 || call.CallerIndex >= len(nodes) {
			continue
		}
		callerID := nodes[call.CallerIndex].ID

		if call.Ident != "" {
			if edge, ok := ix.resolveIdentCall(callerID, call.Ident, callerPkg); ok {
				edges = append(edges, edge)
			}
			continue
		}
		if call.SelName != "" {
			if edge, ok := ix.resolveSelectorCall(callerID, pf, call); ok {
				edges = append(edges, edge)
			}
		}
	}

	return edges
}

func (ix *Indexer) resolveIdentCall(callerID int64, ident, callerPkg string) (domain.CodeEdge, bool) {
	candidates, err := ix.Repo.NodesByName(ix.Project, ident)
	if err != nil || len(candidates) == 0 {
		return domain.CodeEdge{}, false
	}
	candidates = filterCallable(candidates)
	if len(candidates) == 0 {
		return domain.CodeEdge{}, false
	}
	if len(candidates) == 1 {
		return domain.CodeEdge{FromID: callerID, ToID: candidates[0].ID, Kind: domain.EdgeCalls, Confidence: 0.9}, true
	}
	for _, c := range candidates {
		if c.Package == callerPkg {
			return domain.CodeEdge{FromID: callerID, ToID: c.ID, Kind: domain.EdgeCalls, Confidence: 0.8}, true
		}
	}
	return domain.CodeEdge{FromID: callerID, ToID: candidates[0].ID, Kind: domain.EdgeCalls, Confidence: 0.4}, true
}

func (ix *Indexer) resolveSelectorCall(callerID int64, pf parsedFile, call parsedCall) (domain.CodeEdge, bool) {
	importPath, ok := pf.Imports[call.SelPkgAlias]
	if !ok {
		return domain.CodeEdge{}, false
	}
	candidates, err := ix.Repo.NodesByName(ix.Project, call.SelName)
	if err != nil {
		return domain.CodeEdge{}, false
	}
	candidates = filterCallable(candidates)
	for _, c := range candidates {
		if c.Package == importPath || packageBaseName(c.Package) == call.SelPkgAlias {
			return domain.CodeEdge{FromID: callerID, ToID: c.ID, Kind: domain.EdgeCalls, Confidence: 0.7}, true
		}
	}
	return domain.CodeEdge{}, false
}

func filterCallable(nodes []domain.CodeNode) []domain.CodeNode {
	out := nodes[:0]
	for _, n := range nodes {
		if n.Kind == domain.NodeFunction || n.Kind == domain.NodeMethod {
			out = append(out, n)
		}
	}
	return out
}

func packageBaseName(pkg string) string {
	return filepath.Base(pkg)
}

func (ix *Indexer) walkGoFiles() ([]string, error) {
	var paths []string
	err := filepath.WalkDir(ix.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(ix.Root, path)
		if relErr != nil {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if rel != "." && (skippedDirs[base] || (len(base) > 1 && base[0] == '.')) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".go" {
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	return paths, err
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
