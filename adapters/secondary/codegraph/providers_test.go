package codegraph

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"mem/adapters/secondary/codegraph/codebasememory"
	"mem/adapters/secondary/codegraph/codegraphcli"
	"mem/application/ports"
	"mem/domain"
)

func TestNewProviders_SeleccionaAdaptadorPorComando(t *testing.T) {
	root := t.TempDir()
	memDir := filepath.Join(root, ".memory")
	providers := NewProviders(root, memDir, []string{"codegraph", "codebase-memory-mcp"})
	if len(providers) != 2 {
		t.Fatalf("proveedores = %d", len(providers))
	}
	if _, ok := providers[0].(*codegraphcli.Provider); !ok {
		t.Fatalf("codegraph debe usar su propio adaptador, recibido %T", providers[0])
	}
	if _, ok := providers[1].(*codebasememory.Provider); !ok {
		t.Fatalf("codebase-memory-mcp debe conservar su adaptador, recibido %T", providers[1])
	}
	if _, ok := NewProviders(root, memDir, nil)[0].(*codebasememory.Provider); !ok {
		t.Fatal("la autodetección por defecto debe seguir siendo codebase-memory-mcp")
	}
}

// Sin lista configurada, CodeGraph instalado en PATH debe sumarse tras
// codebase-memory-mcp: antes solo aparecía si el settings.json local lo
// listaba a mano, así que otras máquinas nunca lo veían.
func TestNewProviders_AutodetectaCodeGraphEnPath(t *testing.T) {
	root := t.TempDir()
	memDir := filepath.Join(root, ".memory")
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, codegraphcli.ProviderName), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	providers := NewProviders(root, memDir, nil)
	if len(providers) != 2 {
		t.Fatalf("proveedores = %d, se esperaba codebase-memory-mcp + codegraph", len(providers))
	}
	if _, ok := providers[0].(*codebasememory.Provider); !ok {
		t.Fatalf("codebase-memory-mcp conserva la prioridad, recibido %T", providers[0])
	}
	if _, ok := providers[1].(*codegraphcli.Provider); !ok {
		t.Fatalf("codegraph autodetectado debe usar su adaptador, recibido %T", providers[1])
	}

	t.Setenv("PATH", t.TempDir())
	if got := len(NewProviders(root, memDir, nil)); got != 1 {
		t.Fatalf("sin codegraph en PATH: proveedores = %d", got)
	}
	if got := len(NewProviders(root, memDir, []string{"codebase-memory-mcp"})); got != 1 {
		t.Fatalf("una lista explícita se respeta tal cual: proveedores = %d", got)
	}
}

type refreshTestProvider struct {
	calls   atomic.Int32
	started chan struct{}
	finish  chan struct{}
}

func (p *refreshTestProvider) Name() string { return "test" }
func (p *refreshTestProvider) Snapshot() domain.CodeProviderSnapshot {
	return domain.CodeProviderSnapshot{}
}
func (p *refreshTestProvider) MaybeRefresh() {}
func (p *refreshTestProvider) ImpactFor(string) (domain.CodeImpactAnnotation, bool) {
	return domain.CodeImpactAnnotation{}, false
}
func (p *refreshTestProvider) Refresh(context.Context) {
	p.calls.Add(1)
	if p.started != nil {
		close(p.started)
		<-p.finish
	}
}

func TestRefreshAll_EvitaSincronizacionesSimultaneas(t *testing.T) {
	memDir := filepath.Join(t.TempDir(), ".memory")
	first := &refreshTestProvider{started: make(chan struct{}), finish: make(chan struct{})}
	second := &refreshTestProvider{}
	done := make(chan struct{})
	go func() {
		RefreshAll(context.Background(), memDir, []ports.CodeGraphProvider{first})
		close(done)
	}()
	<-first.started
	RefreshAll(context.Background(), memDir, []ports.CodeGraphProvider{second})
	if got := second.calls.Load(); got != 0 {
		t.Fatalf("segundo refresco durante el lock: %d", got)
	}
	close(first.finish)
	<-done
	RefreshAll(context.Background(), memDir, []ports.CodeGraphProvider{second})
	if got := second.calls.Load(); got != 1 {
		t.Fatalf("refresco tras liberar el lock: %d", got)
	}
}
