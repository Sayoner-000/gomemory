package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHCD062SinCRLF: las plantillas y skills que gomemory distribuye usan solo
// LF, como el resto del repositorio (.gitattributes eol=lf). Una skill con CR
// mezclados llega así a cada agente y ensucia todos sus diffs (HCD-062).
func TestHCD062SinCRLF(t *testing.T) {
	root := filepath.Join("..", "..")
	var offenders []string
	for _, dir := range []string{"infrastructure/templates", "skills"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if ext := filepath.Ext(path); ext != ".md" && ext != ".tmpl" && ext != ".json" && ext != ".yaml" && ext != ".yml" && ext != ".toml" {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if bytes.IndexByte(b, '\r') >= 0 {
				rel, _ := filepath.Rel(root, path)
				offenders = append(offenders, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("recorrer %s: %v", dir, err)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("contienen CR (usa solo LF): %s", strings.Join(offenders, ", "))
	}
}
