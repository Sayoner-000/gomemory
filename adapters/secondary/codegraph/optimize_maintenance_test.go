package codegraph_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"mem/adapters/secondary/codegraph/codegraphcli"
	"mem/application/ports"
)

func codegraphSnapshotPath(memDir, identity string) string {
	sum := sha256.Sum256([]byte(codegraphcli.ProviderName + ":" + identity))
	return filepath.Join(memDir, fmt.Sprintf("code_provider_snapshot_%x.json", sum[:6]))
}

func TestCodeProviders_OptimizeMaintenance(t *testing.T) {
	root := t.TempDir()
	memDir := filepath.Join(root, ".memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cgProvider := codegraphcli.New(root, memDir, "")
	cgMaintainer, ok := any(cgProvider).(ports.CodeGraphMaintainer)
	if !ok {
		t.Fatalf("codegraphcli.Provider debe implementar ports.CodeGraphMaintainer")
	}

	snapFile := codegraphSnapshotPath(memDir, "")
	if err := os.WriteFile(snapFile, []byte("invalid-json"), 0o644); err != nil {
		t.Fatal(err)
	}

	repDry, err := cgMaintainer.Optimize(context.Background(), true, false)
	if err != nil {
		t.Fatalf("cg dry-run error: %v", err)
	}
	if repDry.Provider != codegraphcli.ProviderName {
		t.Errorf("provider = %s, want %s", repDry.Provider, codegraphcli.ProviderName)
	}
	if repDry.SnapshotsCleaned != 1 {
		t.Errorf("cg dry-run: SnapshotsCleaned = %d, want 1", repDry.SnapshotsCleaned)
	}
	if _, err := os.Stat(snapFile); os.IsNotExist(err) {
		t.Errorf("en dry-run snapFile no debió borrarse")
	}

	repReal, err := cgMaintainer.Optimize(context.Background(), false, false)
	if err != nil {
		t.Fatalf("cg real error: %v", err)
	}
	if repReal.SnapshotsCleaned != 1 {
		t.Errorf("cg real: SnapshotsCleaned = %d, want 1", repReal.SnapshotsCleaned)
	}
	if _, err := os.Stat(snapFile); !os.IsNotExist(err) {
		t.Errorf("en ejecución real snapFile debió eliminarse")
	}
}

func TestCodeProviders_RegressionAuditFindings(t *testing.T) {
	root := t.TempDir()
	memDir := filepath.Join(root, ".memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cgProvider := codegraphcli.New(root, memDir, "")

	// 1. C-004: No eliminar un snapshot de otro proveedor
	otherSnapshotFile := filepath.Join(memDir, "code_provider_snapshot_other.json")
	if err := os.WriteFile(otherSnapshotFile, []byte(`{"provider":"other-provider","available":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	// 2. C-005: Snapshot incompleto propio (available=true sin architecture) no produce pánico y se detecta
	incompleteSnap := codegraphSnapshotPath(memDir, "")
	if err := os.WriteFile(incompleteSnap, []byte(`{"provider":"codegraph","available":true,"root_path":"`+root+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("C-005: pánico por architecture nil: %v", r)
		}
	}()

	rep, err := cgProvider.Optimize(context.Background(), false, false)
	if err != nil {
		t.Fatalf("cg optimize error: %v", err)
	}
	if rep.SnapshotsCleaned != 1 {
		t.Errorf("C-005: se esperaba limpiar 1 snapshot incompleto, got %d", rep.SnapshotsCleaned)
	}
	if _, err := os.Stat(otherSnapshotFile); os.IsNotExist(err) {
		t.Errorf("C-004: el snapshot de otro proveedor fue erróneamente borrado")
	}
}

func TestCodeProviders_FailedRemoval(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod 0500 no impide borrado en Windows o bajo root")
	}
	root := t.TempDir()
	memDir := filepath.Join(root, ".memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cgProvider := codegraphcli.New(root, memDir, "")

	snapFile := codegraphSnapshotPath(memDir, "")
	if err := os.WriteFile(snapFile, []byte("invalid-json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(memDir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(memDir, 0o755)

	rep, err := cgProvider.Optimize(context.Background(), false, false)
	if err == nil {
		t.Errorf("se esperaba error al no tener permisos para eliminar el snapshot")
	}
	if rep.SnapshotsCleaned != 0 {
		t.Errorf("SnapshotsCleaned = %d, want 0 ante fallo de eliminación", rep.SnapshotsCleaned)
	}

	_ = os.Chmod(memDir, 0o755)
	repRestored, err := cgProvider.Optimize(context.Background(), false, false)
	if err != nil {
		t.Fatalf("error tras restaurar permisos: %v", err)
	}
	if repRestored.SnapshotsCleaned != 1 {
		t.Errorf("SnapshotsCleaned = %d, want 1", repRestored.SnapshotsCleaned)
	}
}
