package hcdfiles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"mem/domain"
)

// TestAuditC004DeterministicMoveDoc instrumenta el MoveDoc actual en un overlay
// aislado para forzar la intercalación: resolve termina -> otra goroutine crea
// el destino con O_EXCL -> se ejecuta el renameNoReplace real. Verifica que
// MoveDoc rechace con domain.ErrNotAllowed sin sobrescribir el destino ni
// perder el origen.
func TestAuditC004DeterministicMoveDoc(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("overlay interleaving test scoped to Darwin/Linux")
	}
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate workspace source")
	}
	dir := filepath.Dir(here)
	sourcePath := filepath.Join(dir, "workspace.go")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	patched := string(source)
	for _, edit := range [][2]string{
		{"\tTimeout time.Duration\n", "\tTimeout time.Duration\n\tc004BeforeRename func(string, string)\n"},
		{"\tif err := renameNoReplace(src, dst); err != nil {\n", "\tif w.c004BeforeRename != nil {\n\t\tw.c004BeforeRename(src, dst)\n\t}\n\tif err := renameNoReplace(src, dst); err != nil {\n"},
	} {
		if strings.Count(patched, edit[0]) != 1 {
			t.Fatalf("overlay seam changed; inspect MoveDoc before updating: %q", edit[0])
		}
		patched = strings.Replace(patched, edit[0], edit[1], 1)
	}
	tmp := t.TempDir()
	workspaceOverlay := filepath.Join(tmp, "workspace.go")
	childOverlay := filepath.Join(tmp, "c004_child_test.go")
	for path, data := range map[string]string{workspaceOverlay: patched, childOverlay: c004ChildSource} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{
		sourcePath: workspaceOverlay,
		filepath.Join(dir, "c004_overlay_child_test.go"): childOverlay,
	}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(tmp, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	args := []string{"test", "-race", "-overlay", overlayPath, ".", "-run", "^TestC004OverlayAtomicNoOverwrite$", "-count=20", "-timeout=30s", "-v"}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	t.Logf("go %s\n%s", strings.Join(args, " "), output)
	if err != nil {
		t.Fatalf("atomic no-overwrite regression failed: %v", err)
	}
}

const c004ChildSource = `package hcdfiles

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mem/domain"
)

func TestC004OverlayAtomicNoOverwrite(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "hcd"), 0700); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "hcd/source.md")
	dst := filepath.Join(root, "hcd/target.md")
	if err := os.WriteFile(src, []byte("source document"), 0600); err != nil {
		t.Fatal(err)
	}
	reachedRename := make(chan struct{})
	writerDone := make(chan error, 1)
	go func() {
		<-reachedRename
		f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			_, err = f.WriteString("exclusive destination marker")
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
		}
		writerDone <- err
	}()
	calls := 0
	w := Workspace{Root: root, c004BeforeRename: func(from, to string) {
		calls++
		if from != src || to != dst {
			t.Fatalf("unexpected rename: %s -> %s", from, to)
		}
		if _, err := os.Lstat(dst); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("destination must still be absent before releasing writer: %v", err)
		}
		close(reachedRename)
		if err := <-writerDone; err != nil {
			t.Fatalf("exclusive competing create: %v", err)
		}
	}}
	err := w.MoveDoc("hcd/source.md", "hcd/target.md")
	if calls != 1 {
		t.Fatalf("MoveDoc did not reach renameNoReplace once: %d", calls)
	}
	if !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("MoveDoc must reject intervening destination with ErrNotAllowed, got %v", err)
	}
	afterDst, readErr := os.ReadFile(dst)
	if readErr != nil || string(afterDst) != "exclusive destination marker" {
		t.Fatalf("destination was altered: %q %v", afterDst, readErr)
	}
	afterSrc, srcErr := os.ReadFile(src)
	if srcErr != nil || string(afterSrc) != "source document" {
		t.Fatalf("source was lost or altered: %q %v", afterSrc, srcErr)
	}
}
`

func TestMoveDocConcurrentSingleWinner(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "hcd"), 0700); err != nil {
		t.Fatal(err)
	}
	const contenders = 8
	for i := range contenders {
		name := fmt.Sprintf("hcd/src-%d.md", i)
		if err := os.WriteFile(filepath.Join(root, name), []byte(fmt.Sprintf("doc-%d", i)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	w := Workspace{Root: root}
	start := make(chan struct{})
	errs := make([]error, contenders)
	var wg sync.WaitGroup
	for i := range contenders {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			errs[idx] = w.MoveDoc(fmt.Sprintf("hcd/src-%d.md", idx), "hcd/target.md")
		}(i)
	}
	close(start)
	wg.Wait()

	winner := -1
	for i, err := range errs {
		if err == nil {
			if winner != -1 {
				t.Fatalf("multiple MoveDoc winners: %d and %d", winner, i)
			}
			winner = i
			continue
		}
		if !errors.Is(err, domain.ErrNotAllowed) {
			t.Fatalf("contender %d unexpected error: %v", i, err)
		}
	}
	if winner == -1 {
		t.Fatal("expected exactly one MoveDoc winner")
	}
	got, err := os.ReadFile(filepath.Join(root, "hcd/target.md"))
	if err != nil || string(got) != fmt.Sprintf("doc-%d", winner) {
		t.Fatalf("target content = %q (%v), want doc-%d", got, err, winner)
	}
	for i := range contenders {
		srcPath := filepath.Join(root, fmt.Sprintf("hcd/src-%d.md", i))
		if i == winner {
			if _, err := os.Lstat(srcPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("winner source still exists: %v", err)
			}
			continue
		}
		data, err := os.ReadFile(srcPath)
		if err != nil || string(data) != fmt.Sprintf("doc-%d", i) {
			t.Fatalf("loser %d source corrupted: %q %v", i, data, err)
		}
	}
}

func TestMoveDocEdgeCases(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "hcd"), 0700); err != nil {
		t.Fatal(err)
	}
	w := Workspace{Root: root}
	if err := w.MoveDoc("hcd/missing.md", "hcd/dst.md"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing source = %v, want os.ErrNotExist", err)
	}
	if err := os.WriteFile(filepath.Join(root, "hcd/same.md"), []byte("same"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := w.MoveDoc("hcd/same.md", "hcd/same.md"); !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("same path = %v, want domain.ErrNotAllowed", err)
	}
}
