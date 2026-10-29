package hcdfiles

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestHCDSubjectDetectsFileChanges(t *testing.T) {
	root := t.TempDir()
	data := []byte("code")
	if e := os.WriteFile(filepath.Join(root, "code.go"), data, 0600); e != nil {
		t.Fatal(e)
	}
	sum := sha256.New()
	fmt.Fprintf(sum, "code.go\x00%d\x00", len(data))
	sum.Write(data)
	ref := "tree:" + hex.EncodeToString(sum.Sum(nil))[:12] + " (archivos: code.go)"
	s := SubjectInspector{Root: root}
	valid, e := s.Current(context.Background(), ref)
	if e != nil || !valid {
		t.Fatalf("current=%v %v", valid, e)
	}
	if e = os.WriteFile(filepath.Join(root, "code.go"), []byte("changed"), 0600); e != nil {
		t.Fatal(e)
	}
	valid, e = s.Current(context.Background(), ref)
	if e != nil || valid {
		t.Fatalf("changed code accepted=%v %v", valid, e)
	}
}

// El agente no tenía forma de calcular un sujeto tree: válido; TreeRef debe
// producir exactamente lo que Current acepta y dejar de valer tras un cambio.
func TestSubjectTreeRefRoundTrip(t *testing.T) {
	root := t.TempDir()
	if e := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); e != nil {
		t.Fatal(e)
	}
	s := SubjectInspector{Root: root}
	ref, e := s.TreeRef(context.Background(), []string{"a.go"})
	if e != nil {
		t.Fatal(e)
	}
	if ok, e := s.Current(context.Background(), ref); !ok || e != nil {
		t.Fatalf("Current(%q)=%v %v", ref, ok, e)
	}
	if e = os.WriteFile(filepath.Join(root, "a.go"), []byte("package b\n"), 0o644); e != nil {
		t.Fatal(e)
	}
	if ok, _ := s.Current(context.Background(), ref); ok {
		t.Fatal("ref must go stale after the file changes")
	}
	if _, e = s.TreeRef(context.Background(), nil); e == nil {
		t.Fatal("empty file list must fail")
	}
}
