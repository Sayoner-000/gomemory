package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"mem/domain"
)

// knownAgentNamesForTest devuelve los nombres declarados en el registro
// único de capacidades (domain.KnownAgents), para verificar que el agente
// ficticio de este test no está entre ellos.
func knownAgentNamesForTest() []string {
	names := make([]string, 0, len(domain.KnownAgents))
	for _, a := range domain.KnownAgents {
		names = append(names, a.Name)
	}
	return names
}

// foreignAgentScript reproduce, casi literal, el ejemplo mínimo de
// contracts/agent-integration.md: un agente ficticio que llama a
// `mem hook plan-guard` antes de presentar un plan, usando ÚNICAMENTE el
// dialecto neutral (stdin + código de salida) — ningún parámetro, envoltura
// ni bandera propia de Claude Code.
const foreignAgentScript = `#!/usr/bin/env bash
set -e
plan_file="$1"
if ! reason=$("$MEM_BIN" hook plan-guard < "$plan_file" 2>&1 >/dev/null); then
  echo "PLAN DEVUELTO: $reason"
  exit 1
fi
echo "PLAN ACEPTADO"
exit 0
`

// TestForeignAgentGetsFullGuaranteeWithoutAnyGomemoryChange demuestra SC-A1
// (feature 019): un cliente que imita a un agente que gomemory NO conoce
// —"agente-de-prueba-desconocido" no aparece en domain.KnownAgents ni en
// ningún archivo de este repositorio— obtiene la garantía completa del
// contrato neutral (devolución con plan en prosa, permiso con plan en
// árbol), sin una sola línea de código de gomemory dedicada a él.
func TestForeignAgentGetsFullGuaranteeWithoutAnyGomemoryChange(t *testing.T) {
	const unknownAgentName = "agente-de-prueba-desconocido"
	for _, known := range knownAgentNamesForTest() {
		if known == unknownAgentName {
			t.Fatalf("%q no debía estar en el registro de capacidades — el test perdería su sentido", unknownAgentName)
		}
	}

	bin := buildMemBinary(t)
	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "foreign-agent.sh")
	if err := os.WriteFile(scriptPath, []byte(foreignAgentScript), 0755); err != nil {
		t.Fatalf("escribir script del agente ficticio: %v", err)
	}

	runClient := func(plan string) (stdout string, exitCode int) {
		t.Helper()
		planFile := filepath.Join(t.TempDir(), "plan.txt")
		if err := os.WriteFile(planFile, []byte(plan), 0644); err != nil {
			t.Fatalf("escribir plan.txt: %v", err)
		}

		cmd := exec.Command("bash", scriptPath, planFile)
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(), "MEM_BIN="+bin)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		code := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
			} else {
				t.Fatalf("ejecutar el agente ficticio: %v", err)
			}
		}
		return out.String(), code
	}

	prosaLarga := `Voy a implementar la integración completa revisando todo el código
relevante y haciendo los cambios necesarios en varios archivos del proyecto
para que todo funcione correctamente de principio a fin sin dejar nada
pendiente ni ningún caso sin cubrir en el flujo principal de la aplicación.`

	stdout, code := runClient(prosaLarga)
	if code == 0 {
		t.Errorf("un plan en prosa debe devolverse incluso para un agente desconocido, stdout=%q", stdout)
	}
	if !bytes.Contains([]byte(stdout), []byte("PLAN DEVUELTO")) {
		t.Errorf("el script del agente ficticio debe reportar la devolución, stdout=%q", stdout)
	}

	planEnArbol := `🎯 objetivo de prueba
├─ [1] subtarea
│  └─ [1.1] ✓ verbo + objeto → resultado verificable
└─ [2] ✓ otra subtarea → resultado verificable   (∥)`

	stdout, code = runClient(planEnArbol)
	if code != 0 {
		t.Errorf("un plan en árbol debe permitirse incluso para un agente desconocido, stdout=%q", stdout)
	}
	if !bytes.Contains([]byte(stdout), []byte("PLAN ACEPTADO")) {
		t.Errorf("el script del agente ficticio debe reportar el permiso, stdout=%q", stdout)
	}
}
