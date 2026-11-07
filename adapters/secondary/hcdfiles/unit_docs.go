package hcdfiles

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// UnitDocs devuelve las rutas relativas de los documentos hcd/HCD-*.md, sin
// seguir symlinks (HCD-046, Q7). Sin directorio hcd/ no hay documentos.
func (w Workspace) UnitDocs() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(w.Root, "hcd"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasPrefix(e.Name(), "HCD-") && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, "hcd/"+e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}
