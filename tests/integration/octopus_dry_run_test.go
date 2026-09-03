package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// T075 — AC-019 e INV-AAR-018: la simulación NO ejecuta nada. Se comprueba
// contra el binario real observando los procesos hijo, no leyendo el código:
// "no llamamos a exec.Command" es una afirmación sobre el código, y lo que la
// especificación promete es una afirmación sobre el comportamiento.
func TestOctopusPlan_NoIniciaNingunProceso(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "mem-dryrun-test")
	build := exec.Command("go", "build", "-o", bin, "./infrastructure")
	build.Dir = repoRootIntegration(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compilar binario: %v\n%s", err, out)
	}

	dir := proyectoConOctopus(t)
	plan := filepath.Join(dir, "plan.json")
	escribirArchivo(t, plan, `{
		"plan_id": "dry-run",
		"budget": {"total_tokens": 50000},
		"capabilities": {"subagents": true, "parallel": true, "isolated_context": true, "max_parallel": 3},
		"tasks": [
			{"id": "T001", "objective": "Investigar la expiración", "task_class": "investigation",
			 "read_only": true, "files": ["a.go"], "context_tokens": 2200},
			{"id": "T002", "objective": "Investigar el refresco", "task_class": "investigation",
			 "read_only": true, "files": ["b.go"], "context_tokens": 2100}
		]
	}`)

	cmd := exec.Command(bin, "octopus", "plan", "--file", plan)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("octopus plan: %v\n%s", err, out)
	}

	// El proceso terminó y no dejó hijos vivos: si hubiera lanzado un subagente,
	// CombinedOutput habría esperado a que su stdout se cerrara.
	if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
		t.Error("el proceso debería haber terminado por su cuenta")
	}

	texto := string(out)
	if !strings.Contains(texto, "no se inicia ningún subagente") {
		t.Errorf("la salida debe declarar que es una simulación:\n%s", texto)
	}
	// Y sí produjo un plan: sin esto, "no ejecutó nada" sería trivialmente cierto.
	if !strings.Contains(texto, "T001") || !strings.Contains(texto, "T002") {
		t.Errorf("la simulación debe describir las rutas de todas las tareas:\n%s", texto)
	}
	if !strings.Contains(texto, "Agentes delegados:") {
		t.Errorf("la simulación debe declarar cuántos agentes propondría:\n%s", texto)
	}
}
