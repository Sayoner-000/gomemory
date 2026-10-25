package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const mcpWithGomemory = `{"mcpServers":{"gomemory":{"command":"mem","args":["mcp"]},"otro":{"command":"x"}}}`

func writeMCP(t *testing.T, root string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(root, ".mcp.json")
	if err := os.WriteFile(path, []byte(mcpWithGomemory), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func gomemoryEntry(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg["mcpServers"].(map[string]any)["gomemory"].(map[string]any)
}

// The list is written atomically and the file keeps its mode (H-01, ADM 2026-10-24).
func TestApplyAutoApprove_WritesAndKeepsTheFileMode(t *testing.T) {
	root := t.TempDir()
	path := writeMCP(t, root, 0o640)
	if err := ApplyAutoApprove(root, Settings{AutoApprove: true, AutoApproveTools: []string{"get_context"}}); err != nil {
		t.Fatal(err)
	}
	if got := gomemoryEntry(t, path)["autoApprove"]; got == nil {
		t.Fatal("autoApprove not written")
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(path); info.Mode().Perm() != 0o640 {
			t.Fatalf("mode = %v, want 0640 kept", info.Mode().Perm())
		}
	}
	if leftovers, _ := filepath.Glob(filepath.Join(root, ".*.tmp-*")); len(leftovers) != 0 {
		t.Fatalf("temporary files left: %v", leftovers)
	}
}

// Turning auto-approve off removes the list; before, an early return made the
// removal unreachable.
func TestApplyAutoApprove_OffRemovesTheList(t *testing.T) {
	root := t.TempDir()
	path := writeMCP(t, root, 0o644)
	if err := ApplyAutoApprove(root, Settings{AutoApprove: true, AutoApproveTools: []string{"get_context"}}); err != nil {
		t.Fatal(err)
	}
	if err := ApplyAutoApprove(root, Settings{AutoApprove: false, AutoApproveTools: []string{"get_context"}}); err != nil {
		t.Fatal(err)
	}
	if _, had := gomemoryEntry(t, path)["autoApprove"]; had {
		t.Fatal("autoApprove kept after turning it off")
	}
}

func TestApplyAutoApprove_MissingFileIsNotAnError(t *testing.T) {
	if err := ApplyAutoApprove(t.TempDir(), Settings{AutoApprove: true, AutoApproveTools: []string{"get_context"}}); err != nil {
		t.Fatalf("missing .mcp.json = %v, want nil", err)
	}
}

func TestApplyAutoApprove_ReportsUnreadableJSONAndWriteFailures(t *testing.T) {
	on := Settings{AutoApprove: true, AutoApproveTools: []string{"get_context"}}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte("{roto"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ApplyAutoApprove(root, on); err == nil {
		t.Error("an unreadable .mcp.json reported success")
	}
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not block writes here")
	}
	root = t.TempDir()
	path := writeMCP(t, root, 0o644)
	if err := os.Chmod(root, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) })
	if err := ApplyAutoApprove(root, on); err == nil {
		t.Fatal("a failed write reported success")
	}
	if _, had := gomemoryEntry(t, path)["autoApprove"]; had {
		t.Fatal("a failed write changed the file")
	}
}

// A linked project config is refused, and the link and its target stay intact:
// replacing with rename would turn the link into a plain copy (H-01, ADM r2).
func TestApplyAutoApprove_RefusesLinkedProjectConfigs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need extra privileges on Windows")
	}
	on := Settings{AutoApprove: true, AutoApproveTools: []string{"get_context"}}
	cases := map[string]func(root, shared string) (link, target string){
		"linked .mcp.json": func(root, shared string) (string, string) {
			target := filepath.Join(shared, "mcp.json")
			return filepath.Join(root, ".mcp.json"), target
		},
		"linked .cursor directory": func(root, shared string) (string, string) {
			return filepath.Join(root, ".cursor"), shared
		},
	}
	for name, layout := range cases {
		t.Run(name, func(t *testing.T) {
			root, shared := t.TempDir(), t.TempDir()
			sharedFile := filepath.Join(shared, "mcp.json")
			if err := os.WriteFile(sharedFile, []byte(mcpWithGomemory), 0o644); err != nil {
				t.Fatal(err)
			}
			link, target := layout(root, shared)
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			if err := ApplyAutoApprove(root, on); err == nil {
				t.Fatal("writing through a linked project config reported success")
			}
			if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("the link was replaced: %v, %v", info, err)
			}
			if data, _ := os.ReadFile(sharedFile); string(data) != mcpWithGomemory {
				t.Fatalf("the shared target changed: %s", data)
			}
		})
	}
}
