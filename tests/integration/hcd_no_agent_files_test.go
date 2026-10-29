package main

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"mem/adapters/secondary/persistence"
	"os"
	"path/filepath"
	"testing"
)

func TestHCDPreservesFilesOutsideArtifactsAndDatabase(t *testing.T) {
	runHCDBinaryMinimalFlow(t, true)
}

// FR-041a explicitly permits the GoMemory database outside hcd/.
func hcdProjectSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if rel == "hcd" || rel == persistence.MemDir {
				return filepath.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = fmt.Sprintf("%o:%x", info.Mode(), sha256.Sum256(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
