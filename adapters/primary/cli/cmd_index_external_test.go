package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mem/application/ports"
	"mem/domain"
)

type indexGraphProvider struct {
	name  string
	calls *[]string
	err   error
}

func (p indexGraphProvider) Name() string { return p.name }
func (p indexGraphProvider) Snapshot() domain.CodeProviderSnapshot {
	return domain.CodeProviderSnapshot{}
}
func (p indexGraphProvider) MaybeRefresh() {}
func (p indexGraphProvider) ImpactFor(string) (domain.CodeImpactAnnotation, bool) {
	return domain.CodeImpactAnnotation{}, false
}
func (p indexGraphProvider) IndexRepository(_ context.Context, mode string) (int, int, error) {
	*p.calls = append(*p.calls, p.name+":"+mode)
	return 12, 34, p.err
}

func TestIndexExternalGraph_IndexaAmbosAunqueElPrimeroFalle(t *testing.T) {
	var calls []string
	deps := &Deps{CodeProviders: []ports.CodeGraphProvider{
		indexGraphProvider{name: "codebase-memory-mcp", calls: &calls, err: errors.New("falló índice")},
		indexGraphProvider{name: "codegraph", calls: &calls},
	}}
	out := captureStdout(t, func() { indexExternalGraph(deps) })
	if got := strings.Join(calls, ","); got != "codebase-memory-mcp:full,codegraph:full" {
		t.Fatalf("orden de indexado = %q", got)
	}
	if !strings.Contains(out, "grafo externo (codebase-memory-mcp): falló índice") ||
		!strings.Contains(out, "Indexando grafo externo (codegraph)") ||
		!strings.Contains(out, "Nodos: 12, aristas: 34") {
		t.Fatalf("salida no reporta ambos resultados:\n%s", out)
	}
}
