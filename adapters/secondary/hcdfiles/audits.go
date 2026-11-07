package hcdfiles

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// auditsDir es donde viven los informes de auditoría del proyecto (HCD-046).
const auditsDir = "docs/audits/"

// ListAudits devuelve las rutas relativas de los informes .md de docs/audits,
// sin recorrer subdirectorios ni seguir symlinks. Sin directorio no hay
// informes.
func (w Workspace) ListAudits() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(w.Root, filepath.FromSlash(auditsDir)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, auditsDir+e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// ReadAudit lee un informe; nunca fuera de docs/audits ni por un symlink.
func (w Workspace) ReadAudit(rel string) ([]byte, error) {
	full, err := w.resolve(rel, auditsDir)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}
