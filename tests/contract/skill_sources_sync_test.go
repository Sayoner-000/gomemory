package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSkillSourcesInSync(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
	entries, err := os.ReadDir(filepath.Join(root, "infrastructure/templates"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		canonical, err := os.ReadFile(filepath.Join(root, "infrastructure/templates", name, "SKILL.md"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		published, err := os.ReadFile(filepath.Join(root, "skills", name, "SKILL.md"))
		if os.IsNotExist(err) && name != "socratica" {
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !bytes.Equal(canonical, published) {
			t.Errorf("%s: published skill differs from canonical template", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "skills/socratica/SKILL.md")); err != nil {
		t.Errorf("socratica: %v", err)
	}
}
