package codegraph_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"mem/adapters/secondary/codegraph/codebasememory"
	"mem/adapters/secondary/codegraph/codegraphcli"
	"mem/application/ports"
)

func TestCodeProviders_OptimizeMaintenance(t *testing.T) {
	root := t.TempDir()
	memDir := filepath.Join(root, ".memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 1. Probar codebase-memory-mcp maintainer
	cbmProvider := codebasememory.New(root, memDir, "")
	cbmMaintainer, ok := any(cbmProvider).(ports.CodeGraphMaintainer)
	if !ok {
		t.Fatalf("codebasememory.Provider debe implementar ports.CodeGraphMaintainer")
	}

	// Crear archivo de snapshot corrupto y lock huérfano
	snapFile := filepath.Join(memDir, "code_provider_snapshot.json")
	if err := os.WriteFile(snapFile, []byte("invalid-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	lockFile := filepath.Join(memDir, "codebase_refresh.lock")
	if err := os.WriteFile(lockFile, []byte("stale-pid"), 0o644); err != nil {
		t.Fatal(err)
	}
	staleTime := time.Now().Add(-10 * time.Minute)
	_ = os.Chtimes(lockFile, staleTime, staleTime)

	repCbmDry, err := cbmMaintainer.Optimize(context.Background(), true, false)
	if err != nil {
		t.Fatalf("cbm dry-run error: %v", err)
	}
	if repCbmDry.SnapshotsCleaned != 2 {
		t.Errorf("cbm dry-run: SnapshotsCleaned = %d, want 2", repCbmDry.SnapshotsCleaned)
	}
	// Asegurar que en dry-run no se borraron
	if _, err := os.Stat(snapFile); os.IsNotExist(err) {
		t.Errorf("en dry-run snapFile no debió borrarse")
	}

	// Ejecución real
	repCbmReal, err := cbmMaintainer.Optimize(context.Background(), false, false)
	if err != nil {
		t.Fatalf("cbm real error: %v", err)
	}
	if repCbmReal.SnapshotsCleaned != 2 {
		t.Errorf("cbm real: SnapshotsCleaned = %d, want 2", repCbmReal.SnapshotsCleaned)
	}
	if _, err := os.Stat(snapFile); !os.IsNotExist(err) {
		t.Errorf("en ejecución real snapFile debió eliminarse")
	}

	// 2. Probar codegraph maintainer
	cgProvider := codegraphcli.New(root, memDir, "")
	cgMaintainer, ok := any(cgProvider).(ports.CodeGraphMaintainer)
	if !ok {
		t.Fatalf("codegraphcli.Provider debe implementar ports.CodeGraphMaintainer")
	}

	repCg, err := cgMaintainer.Optimize(context.Background(), false, false)
	if err != nil {
		t.Fatalf("codegraph maintainer error: %v", err)
	}
	if repCg.Provider != "codegraph" {
		t.Errorf("provider = %s, want codegraph", repCg.Provider)
	}
}

func TestCodeProviders_RegressionAuditFindings(t *testing.T) {
	root := t.TempDir()
	memDir := filepath.Join(root, ".memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 1. C-004: Probar que un proveedor no elimina el snapshot válido del otro
	cbmProvider := codebasememory.New(root, memDir, "")
	cgProvider := codegraphcli.New(root, memDir, "")

	cgSnapshotFile := filepath.Join(memDir, "code_provider_snapshot_other.json")
	if err := os.WriteFile(cgSnapshotFile, []byte(`{"provider":"codegraph","available":false,"root_path":"`+root+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	repCbm, err := cbmProvider.Optimize(context.Background(), false, false)
	if err != nil {
		t.Fatalf("cbm optimize error: %v", err)
	}
	if repCbm.SnapshotsCleaned != 0 {
		t.Errorf("C-004: cbm limpió un snapshot ajeno de codegraph: got %d, want 0", repCbm.SnapshotsCleaned)
	}
	if _, err := os.Stat(cgSnapshotFile); os.IsNotExist(err) {
		t.Errorf("C-004: el snapshot de codegraph fue erróneamente borrado por cbm")
	}

	// 2. C-005: Snapshot sin architecture no produce pánico
	incompleteSnap := filepath.Join(memDir, "code_provider_snapshot.json")
	if err := os.WriteFile(incompleteSnap, []byte(`{"provider":"codebase-memory-mcp","available":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("C-005: pánico por architecture nil: %v", r)
		}
	}()

	repIncomplete, err := cbmProvider.Optimize(context.Background(), true, false)
	if err != nil {
		t.Fatalf("optimize incomplete snapshot: %v", err)
	}
	if repIncomplete.SnapshotsCleaned != 1 {
		t.Errorf("C-005: se esperaba detectar 1 snapshot incompleto, got %d", repIncomplete.SnapshotsCleaned)
	}

	repCgIncomplete, err := cgProvider.Optimize(context.Background(), true, false)
	if err != nil {
		t.Fatalf("cg optimize error: %v", err)
	}
	if repCgIncomplete.SnapshotsCleaned != 0 {
		t.Errorf("cg no debía limpiar el snapshot de cbm, got %d", repCgIncomplete.SnapshotsCleaned)
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

	cbmProvider := codebasememory.New(root, memDir, "")

	snapFile := filepath.Join(memDir, "code_provider_snapshot.json")
	if err := os.WriteFile(snapFile, []byte("invalid-json"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Bloquear permisos de escritura en el directorio para forzar fallo en os.Remove
	if err := os.Chmod(memDir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(memDir, 0o755)

	rep, err := cbmProvider.Optimize(context.Background(), false, false)
	if err == nil {
		t.Errorf("se esperaba error al no tener permisos para eliminar el snapshot")
	}
	if rep.SnapshotsCleaned != 0 {
		t.Errorf("SnapshotsCleaned = %d, want 0 ante fallo de eliminación", rep.SnapshotsCleaned)
	}

	// Restaurar permisos
	_ = os.Chmod(memDir, 0o755)
	repRestored, err := cbmProvider.Optimize(context.Background(), false, false)
	if err != nil {
		t.Fatalf("error tras restaurar permisos: %v", err)
	}
	if repRestored.SnapshotsCleaned != 1 {
		t.Errorf("SnapshotsCleaned = %d, want 1", repRestored.SnapshotsCleaned)
	}
}
