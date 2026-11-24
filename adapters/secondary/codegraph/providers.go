package codegraph

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"mem/adapters/secondary/codegraph/codebasememory"
	"mem/adapters/secondary/codegraph/codegraphcli"
	"mem/application/ports"
)

// NewProviders conserva el orden de prioridad configurado. Sin candidatos
// explícitos autodetecta únicamente CodeGraph cuando su binario está en PATH,
// evitando fricción con codebase-memory-mcp salvo configuración explícita.
func NewProviders(root, memDir string, commands []string) []ports.CodeGraphProvider {
	if len(commands) == 0 {
		if _, err := exec.LookPath(codegraphcli.ProviderName); err == nil {
			return []ports.CodeGraphProvider{codegraphcli.New(root, memDir, "")}
		}
		return nil
	}
	providers := make([]ports.CodeGraphProvider, 0, len(commands))
	for _, command := range commands {
		base := filepath.Base(command)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		if strings.EqualFold(name, codegraphcli.ProviderName) {
			providers = append(providers, codegraphcli.New(root, memDir, command))
		} else {
			providers = append(providers, codebasememory.New(root, memDir, command))
		}
	}
	return providers
}

// RefreshAll se usa en el proceso detached `mem code-refresh`.
func RefreshAll(ctx context.Context, memDir string, providers []ports.CodeGraphProvider) {
	RefreshAllWithInternal(ctx, memDir, providers, nil)
}

// RefreshAllWithInternal ejecuta bajo el mismo lock del SO el refresco de los
// proveedores externos (CodeGraph) y el refresco incremental del grafo interno
// de gomemory, evitando sincronizaciones simultáneas desde hooks o heartbeats.
func RefreshAllWithInternal(ctx context.Context, memDir string, providers []ports.CodeGraphProvider, internal func(context.Context)) {
	if err := os.MkdirAll(memDir, 0o700); err != nil {
		return
	}
	lock, err := os.OpenFile(filepath.Join(memDir, ".code-refresh.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return
	}
	defer lock.Close()
	acquired, err := tryLockRefreshFile(lock)
	if err != nil || !acquired {
		return
	}
	defer unlockRefreshFile(lock)
	for _, provider := range providers {
		if refresher, ok := provider.(interface{ Refresh(context.Context) }); ok {
			refresher.Refresh(ctx)
		}
	}
	if internal != nil {
		internal(ctx)
	}
}
