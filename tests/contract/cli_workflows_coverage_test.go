package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ejercita comandos públicos con el binario y almacén aislados. También aporta
// cobertura de los comandos que no se ejecutan dentro del paquete cli.
func TestCLIWorkflows_MemoriaSesionYConsulta(t *testing.T) {
	s := newLifecycleSandbox(t)
	project := s.Project("workflow")
	steps := [][]string{
		{"init"},
		{"save", "-t", "primera", "-y", "learning", "contenido primero"},
		{"save", "-t", "segunda", "-y", "decision", "contenido segundo"},
		{"list"},
		{"get", "1"},
		{"search", "primera"},
		{"session", "start"},
		{"session", "list"},
		{"session", "end", "-s", "resumen de prueba"},
		{"compare", "-r", "related", "-m", "relación de prueba", "1", "2"},
		{"compare", "list"},
		{"usage"},
		{"pack", "stats"},
		{"pack", "savings"},
		{"review", "history"},
		{"octopus", "status"},
	}
	for _, args := range steps {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			res := s.Run("", project, args...)
			if strings.Join(args, " ") == "pack stats" {
				if res.ExitCode != 1 || !strings.Contains(res.Stderr, "ContextPack JSON válido") {
					t.Fatalf("pack stats sin entrada debe rechazarla: %+v", res)
				}
				return
			}
			if strings.Join(args, " ") == "octopus status" {
				if res.ExitCode != 1 || !strings.Contains(res.Stderr, "desactivado") {
					t.Fatalf("Octopus apagado debe indicarlo: %+v", res)
				}
				return
			}
			if res.ExitCode != 0 {
				t.Fatalf("mem %v: exit=%d stdout=%q stderr=%q", args, res.ExitCode, res.Stdout, res.Stderr)
			}
		})
	}
}

func TestCLIWorkflows_AjustesOctopusYComandoEnvuelto(t *testing.T) {
	s := newLifecycleSandbox(t)
	project := s.Project("settings-octopus-wrap")
	if res := s.Run("", project, "init"); res.ExitCode != 0 {
		t.Fatalf("init: %+v", res)
	}
	for _, args := range [][]string{
		{"settings", "--show"},
		{"settings", "--auto-approve=true", "--code-graph=false", "--compression-level=max", "--concise-output=true"},
		{"settings", "--auto-approve=false", "--compression-level=structural", "--code-graph-providers=proveedor1,proveedor2"},
		{"settings", "--compression-level=none", "--code-graph-command=proveedor", "--tool-output-compression=true"},
	} {
		res := s.Run("", project, args...)
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Compresión de contexto") {
			t.Fatalf("settings %v: %+v", args, res)
		}
	}
	bad := s.Run("", project, "settings", "--compression-level=desconocido")
	if bad.ExitCode == 0 || !strings.Contains(bad.Stderr, "no válido") {
		t.Fatalf("nivel inválido aceptado: %+v", bad)
	}

	mustWrite(t, filepath.Join(project, ".memory", "settings.json"), `{"octopus_enabled":true}`)
	planPath := filepath.Join(project, "plan.json")
	mustWrite(t, planPath, `{"plan_id":"coverage-plan","budget":{"total_tokens":50000},"capabilities":{"subagents":true,"parallel":true,"isolated_context":true,"max_parallel":3},"tasks":[{"id":"T001","objective":"Investigar expiración","task_class":"investigation","read_only":true,"context_tokens":2200},{"id":"T002","objective":"Investigar refresco","task_class":"investigation","read_only":true,"context_tokens":2100}]}`)
	for _, args := range [][]string{
		{"octopus", "plan", "--file", planPath},
		{"octopus", "plan", "--file", planPath, "--json", "--budget", "40000"},
	} {
		res := s.Run("", project, args...)
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "T001") {
			t.Fatalf("octopus plan %v: %+v", args, res)
		}
	}
	for _, args := range [][]string{
		{"octopus", "status"},
		{"octopus", "status", "--json"},
		{"octopus", "usage"},
		{"octopus", "usage", "--json"},
		{"octopus", "history"},
		{"octopus", "history", "--json"},
	} {
		res := s.Run("", project, args...)
		if res.ExitCode != 0 || strings.TrimSpace(res.Stdout) == "" {
			t.Fatalf("octopus %v: %+v", args, res)
		}
	}

	wrapped := s.RunInput("", project, "n\n", "wrap", s.GlobalBin, "settings", "--show")
	if wrapped.ExitCode != 0 || !strings.Contains(wrapped.Stderr, "¿Guardar?") {
		t.Fatalf("wrap: %+v", wrapped)
	}
	saved := s.RunInput("", project, "s\nGuardada desde wrap\nlearning\nContenido real\n", "wrap", "-s=false", s.GlobalBin, "settings", "--show")
	if saved.ExitCode != 0 || !strings.Contains(saved.Stderr, "Memoria guardada") {
		t.Fatalf("wrap y guardar: %+v", saved)
	}
}

func TestCLIWorkflows_SetupMCPTodosLosAgentesDeProyecto(t *testing.T) {
	s := newLifecycleSandbox(t)
	project := s.Project("mcp-setup")
	for _, args := range [][]string{
		{"setup-mcp", "--scope", "project", "--target", project, "--agents", "all"},
		{"setup-mcp", "--scope", "project", "--target", project, "--agents", "all"},
		{"setup-mcp", "--scope", "project", "--target", project, "--agents", "desconocido"},
	} {
		res := s.Run("", project, args...)
		if res.ExitCode != 0 || strings.TrimSpace(res.Stdout) == "" {
			t.Fatalf("setup-mcp %v: %+v", args, res)
		}
	}
	for _, rel := range []string{"opencode.json", ".mcp.json", ".cursor/mcp.json", ".windsurf/mcp_config.json", ".cline/mcp_settings.json"} {
		if _, err := os.Stat(filepath.Join(project, rel)); err != nil {
			t.Fatalf("falta configuración de %s: %v", rel, err)
		}
	}
}

func TestCLIWorkflows_ContextoDocumentosYAdministracion(t *testing.T) {
	s := newLifecycleSandbox(t)
	project := s.Project("commands")
	for _, args := range [][]string{
		{"init"},
		{"save", "-t", "dato", "-y", "learning", "Contenido administrado"},
		{"context", "--full"},
		{"context", "--write"},
		{"docs", "list"},
		{"rules"},
		{"constitution"},
		{"project"},
		{"mass"},
		{"migrate"},
		{"capture", "-w", "Se probó el comando", "-y", "Para validar la CLI", "-l", "El sandbox aísla los datos"},
		{"index", "--skip-graph"},
		{"forget", "1"},
	} {
		res := s.Run("", project, args...)
		if res.ExitCode != 0 {
			t.Fatalf("mem %v: %+v", args, res)
		}
	}
	if _, err := os.Stat(filepath.Join(project, ".memory", "context.md")); err != nil {
		t.Fatalf("context --write no creó el archivo: %v", err)
	}
	missing := s.Run("", project, "forget", "1")
	if missing.ExitCode == 0 || !strings.Contains(missing.Stderr, "no encontrada") {
		t.Fatalf("forget de memoria ausente: %+v", missing)
	}
}

func TestCLIWorkflows_ContextPackYCompresion(t *testing.T) {
	s := newLifecycleSandbox(t)
	project := s.Project("pack")
	for _, args := range [][]string{
		{"init"},
		{"save", "-t", "regla de contexto", "-y", "decision", "usar el contexto del proyecto"},
	} {
		res := s.Run("", project, args...)
		if res.ExitCode != 0 {
			t.Fatalf("mem %v: %+v", args, res)
		}
	}

	built := s.Run("", project, "pack", "build", "--task", "explicar contexto", "--max-tokens", "4000", "--json")
	if built.ExitCode != 0 || !strings.Contains(built.Stdout, "\"Stats\"") {
		t.Fatalf("pack build JSON: %+v", built)
	}
	for _, args := range [][]string{{"pack", "show"}, {"pack", "stats"}} {
		res := s.RunInput("", project, built.Stdout, args...)
		if res.ExitCode != 0 || strings.TrimSpace(res.Stdout) == "" {
			t.Fatalf("mem %v: %+v", args, res)
		}
	}
	for _, args := range [][]string{
		{"pack", "compress", "--level", "structural", "--json"},
		{"pack", "compress", "--compare"},
	} {
		res := s.RunInput("", project, "# Contexto\nLa regla usa el contexto del proyecto.\n", args...)
		if res.ExitCode != 0 || strings.TrimSpace(res.Stdout) == "" {
			t.Fatalf("mem %v: %+v", args, res)
		}
	}
	for _, args := range [][]string{
		{"pack", "savings", "--json"},
		{"pack", "tune", "--reset", "--type", "prose"},
		{"pack", "purge"},
	} {
		res := s.Run("", project, args...)
		if res.ExitCode != 0 {
			t.Fatalf("mem %v: %+v", args, res)
		}
	}
	missing := s.Run("", project, "pack", "retrieve", "ref-ausente")
	if missing.ExitCode != 2 || !strings.Contains(missing.Stderr, "no encontrada") {
		t.Fatalf("referencia ausente: %+v", missing)
	}
}
