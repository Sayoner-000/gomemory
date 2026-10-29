package hcdfiles

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type SubjectInspector struct{ Root string }

func (s SubjectInspector) Current(ctx context.Context, ref string) (bool, error) {
	if strings.HasPrefix(ref, "commit ") {
		want := strings.TrimSpace(strings.TrimPrefix(ref, "commit "))
		cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
		cmd.Dir = s.Root
		out, e := cmd.Output()
		if e != nil {
			return false, e
		}
		if strings.TrimSpace(string(out)) != want {
			return false, nil
		}
		cmd = exec.CommandContext(ctx, "git", "status", "--porcelain")
		cmd.Dir = s.Root
		out, e = cmd.Output()
		return e == nil && strings.TrimSpace(string(out)) == "", e
	}
	if !strings.HasPrefix(ref, "tree:") {
		return false, nil
	}
	hash, files, ok := strings.Cut(strings.TrimPrefix(ref, "tree:"), " (archivos: ")
	if !ok || !strings.HasSuffix(files, ")") {
		return false, nil
	}
	got, e := s.treeHash(strings.Split(strings.TrimSuffix(files, ")"), ","))
	if e != nil {
		return false, e
	}
	return got == hash, nil
}

// TreeRef calcula el sujeto `tree:` de los archivos dados con el mismo hash que
// Current comprueba, para que el agente no tenga que reproducirlo a mano.
func (s SubjectInspector) TreeRef(_ context.Context, paths []string) (string, error) {
	hash, e := s.treeHash(paths)
	if e != nil {
		return "", e
	}
	clean := make([]string, 0, len(paths))
	for _, p := range paths {
		clean = append(clean, strings.TrimSpace(p))
	}
	return "tree:" + hash + " (archivos: " + strings.Join(clean, ", ") + ")", nil
}

func (s SubjectInspector) treeHash(paths []string) (string, error) {
	sum := sha256.New()
	for _, path := range paths {
		path = strings.TrimSpace(path)
		clean := filepath.Clean(path)
		if path == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("invalid subject path")
		}
		full := s.Root
		for _, part := range strings.Split(clean, string(filepath.Separator)) {
			full = filepath.Join(full, part)
			info, e := os.Lstat(full)
			if e != nil {
				return "", e
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("symlink subject is not allowed")
			}
		}
		data, e := os.ReadFile(full)
		if e != nil {
			return "", e
		}
		fmt.Fprintf(sum, "%s\x00%d\x00", filepath.ToSlash(clean), len(data))
		sum.Write(data)
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("subject requires files")
	}
	return hex.EncodeToString(sum.Sum(nil))[:12], nil
}
