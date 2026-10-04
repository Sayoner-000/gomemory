package codegraph

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"mem/adapters/secondary/codegraph/codebasememory"
	"mem/adapters/secondary/codegraph/codegraphcli"
	"mem/application/ports"
)

// NewProviders conserva el orden de prioridad configurado. Sin candidatos
// explícitos mantiene la autodetección histórica de codebase-memory-mcp.
func NewProviders(root, memDir string, commands []string) []ports.CodeGraphProvider {
	if len(commands) == 0 {
		return []ports.CodeGraphProvider{codebasememory.New(root, memDir, "")}
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

// RefreshAll se usa únicamente en el proceso detached `mem code-refresh`.
// El lock del SO evita que hooks concurrentes sincronicen el mismo proyecto a
// la vez; se libera incluso si el proceso muere abruptamente.
func RefreshAll(ctx context.Context, memDir string, providers []ports.CodeGraphProvider) {
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
}
