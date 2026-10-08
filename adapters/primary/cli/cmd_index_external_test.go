package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"mem/adapters/primary/console"
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

func plainFlow(out *bytes.Buffer) *console.Flow {
	return console.NewFlow(out, console.Env{Width: 200, Getenv: func(string) string { return "" }}, "Indexar")
}

func TestIndexExternalGraph_IndexaAmbosAunqueElPrimeroFalle(t *testing.T) {
	var calls []string
	deps := &Deps{CodeProviders: []ports.CodeGraphProvider{
		indexGraphProvider{name: "codebase-memory-mcp", calls: &calls, err: errors.New("falló índice")},
		indexGraphProvider{name: "codegraph", calls: &calls},
	}}
	var out bytes.Buffer
	indexExternalGraph(deps, plainFlow(&out))
	if got := strings.Join(calls, ","); got != "codebase-memory-mcp:full,codegraph:full" {
		t.Fatalf("orden de indexado = %q", got)
	}
	text := out.String()
	if !strings.Contains(text, "⚠ codebase-memory-mcp: falló índice") ||
		!strings.Contains(text, "✓ codegraph · 12 nodos · 34 aristas") {
		t.Fatalf("salida no reporta ambos resultados:\n%s", text)
	}
}

func TestIndexExternalGraph_SenalaCodeGraphAusenteConPista(t *testing.T) {
	var calls []string
	deps := &Deps{CodeProviders: []ports.CodeGraphProvider{
		indexGraphProvider{name: "codebase-memory-mcp", calls: &calls},
	}}
	if got := strings.Join(externalGraphSteps(deps), ","); got != "codebase-memory-mcp,codegraph" {
		t.Fatalf("plan del grafo externo = %q", got)
	}
	var out bytes.Buffer
	indexExternalGraph(deps, plainFlow(&out))
	if text := out.String(); !strings.Contains(text, "− codegraph: no instalado → curl") || !strings.Contains(text, "install.sh") {
		t.Fatalf("falta la pista para instalar codegraph:\n%s", text)
	}
}
