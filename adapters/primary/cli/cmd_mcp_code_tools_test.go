package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/persistence"
)

// TestMCPCodeTools_IndexaYConsultaElGrafo ejercita las herramientas MCP del
// grafo de código propio sobre un proyecto Go real y pequeño: índice vacío,
// indexación, estado, búsqueda, definición con llamadores/llamados,
// recorrido de dependencias en ambas direcciones y símbolos inexistentes.
func TestMCPCodeTools_IndexaYConsultaElGrafo(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module demo\n\ngo 1.22\n")
	write("app/app.go", `package app

import "strings"

// Saludo arma un saludo.
func Saludo(nombre string) string { return prefijo() + strings.TrimSpace(nombre) }

func prefijo() string { return "hola " }

// Servicio agrupa operaciones.
type Servicio struct{}

// Ejecutar usa Saludo.
func (Servicio) Ejecutar() string { return Saludo("mundo") }
`)

	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	deps := &Deps{Root: root, Project: "proj", CodeGraphRepo: persistence.NewCodeGraphRepository(db)}
	cs := connectMCPTestSession(t, deps, root, "proj")
	call := func(tool string, args map[string]any) string {
		t.Helper()
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("%s: err=%v res=%s", tool, err, toolText(res))
		}
		return toolText(res)
	}

	if got := call("graph_status", map[string]any{}); !strings.Contains(got, "Sin código indexado") {
		t.Fatalf("estado sin índice: %q", got)
	}
	if got := call("index_project", map[string]any{"force": true}); !strings.Contains(got, "Índice actualizado") || !strings.Contains(got, "parseados: 1") {
		t.Fatalf("index_project: %q", got)
	}
	status := call("graph_status", map[string]any{})
	for _, want := range []string{"Archivos: 1", "Símbolos:", "Paquetes principales", "app"} {
		if !strings.Contains(status, want) {
			t.Fatalf("graph_status sin %q:\n%s", want, status)
		}
	}
	if got := call("search_code", map[string]any{"query": "Saludo"}); !strings.Contains(got, "Saludo") {
		t.Fatalf("search_code: %q", got)
	}
	if got := call("search_code", map[string]any{"query": "NoExisteNada037", "limit": 3}); !strings.Contains(got, "Sin resultados") {
		t.Fatalf("search_code sin resultados: %q", got)
	}

	def := call("get_symbol", map[string]any{"name": "Saludo"})
	for _, want := range []string{"Saludo", "Paquete: app", "Ubicación: app/app.go", "Llama a:", "prefijo", "Llamado por:"} {
		if !strings.Contains(def, want) {
			t.Fatalf("get_symbol sin %q:\n%s", want, def)
		}
	}
	if got := call("get_symbol", map[string]any{"name": "Fantasma"}); !strings.Contains(got, "no encontrado") {
		t.Fatalf("get_symbol inexistente: %q", got)
	}

	out := call("list_dependencies", map[string]any{"name": "Saludo"})
	if !strings.Contains(out, "relaciones out, profundidad 1") || !strings.Contains(out, "prefijo") {
		t.Fatalf("list_dependencies out:\n%s", out)
	}
	in := call("list_dependencies", map[string]any{"name": "Saludo", "direction": "in", "kind": "calls", "depth": 2})
	if !strings.Contains(in, "relaciones in, profundidad 2") || !strings.Contains(in, "Ejecutar") {
		t.Fatalf("list_dependencies in:\n%s", in)
	}
	if got := call("list_dependencies", map[string]any{"name": "Saludo", "kind": "desconocido"}); !strings.Contains(got, "(calls)") {
		t.Fatalf("un kind inválido cae a calls: %q", got)
	}
	if got := call("list_dependencies", map[string]any{"name": "Fantasma"}); !strings.Contains(got, "no encontrado") {
		t.Fatalf("list_dependencies inexistente: %q", got)
	}
}
