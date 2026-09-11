package main

import (
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

// subagentStopPayload arma el payload típico que Claude Code envía al hook
// SubagentStop: last_assistant_message con una sección de aprendizajes, y
// files/commands para el checkpoint determinista vigente.
const subagentStopPayload = `{
  "last_assistant_message": "Listo.\n\n## Aprendizajes clave\n1. El caché se invalida al escribir la configuración del proyecto\n2. Los hooks de Codex exigen JSON en el evento SubagentStop\n",
  "files": ["a.go"],
  "commands": ["go test ./..."]
}`

// TestHookSubagentStop_CapturaAprendizajesYMantieneElCheckpoint cubre US3
// (feature 030): la captura pasiva de aprendizajes NO debe desplazar el
// checkpoint determinista vigente (files/commands) — ambos deben ocurrir con
// el MISMO payload, leído una sola vez.
func TestHookSubagentStop_CapturaAprendizajesYMantieneElCheckpoint(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	project := persistence.ProjectKey(target)
	if _, err := persistence.NewSessionRepository(db).Start(project); err != nil {
		t.Fatalf("start session: %v", err)
	}
	_ = db.Close()

	runHookWithStdin(t, bin, target, "subagent-stop", subagentStopPayload)

	db2, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer func() { _ = db2.Close() }()
	all, err := persistence.NewMemoryRepository(db2).ListAll(project)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}

	var learnings, checkpoints int
	for _, m := range all {
		switch {
		case m.Type == "learning" && strings.Contains(m.Content, "Procedencia: subagente"):
			learnings++
		case m.Type == "checkpoint" && strings.Contains(m.Title, "Checkpoint de subagente"):
			checkpoints++
			if !strings.Contains(m.Content, "a.go") || !strings.Contains(m.Content, "go test") {
				t.Errorf("el checkpoint debía conservar files/commands del mismo payload: %q", m.Content)
			}
		}
	}
	if learnings != 2 {
		t.Errorf("esperaba 2 memorias de aprendizaje, got %d (total memorias: %d)", learnings, len(all))
	}
	if checkpoints != 1 {
		t.Errorf("esperaba 1 checkpoint de subagente, got %d", checkpoints)
	}
}

// TestHookSubagentStop_RepetirNoDuplicaAprendizajes cubre SC-004: repetir la
// misma respuesta no debe crear memorias de aprendizaje nuevas (el upsert por
// topic_key consolida sobre la existente).
func TestHookSubagentStop_RepetirNoDuplicaAprendizajes(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	project := persistence.ProjectKey(target)
	if _, err := persistence.NewSessionRepository(db).Start(project); err != nil {
		t.Fatalf("start session: %v", err)
	}
	_ = db.Close()

	runHookWithStdin(t, bin, target, "subagent-stop", subagentStopPayload)
	runHookWithStdin(t, bin, target, "subagent-stop", subagentStopPayload)

	db2, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer func() { _ = db2.Close() }()
	all, err := persistence.NewMemoryRepository(db2).ListAll(project)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	var learnings int
	for _, m := range all {
		if m.Type == "learning" && strings.Contains(m.Content, "Procedencia: subagente") {
			learnings++
		}
	}
	if learnings != 2 {
		t.Errorf("repetir la misma respuesta no debía duplicar: esperaba 2 memorias de aprendizaje, got %d", learnings)
	}
}

// TestHookSubagentStop_SinSeccionNoCreaMemoriasDeAprendizaje cubre el edge
// case: una respuesta sin sección de aprendizajes no debe crear ninguna
// memoria extraída (el checkpoint determinista sigue funcionando igual).
func TestHookSubagentStop_SinSeccionNoCreaMemoriasDeAprendizaje(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	project := persistence.ProjectKey(target)
	if _, err := persistence.NewSessionRepository(db).Start(project); err != nil {
		t.Fatalf("start session: %v", err)
	}
	_ = db.Close()

	runHookWithStdin(t, bin, target, "subagent-stop", `{"last_assistant_message":"Listo, sin nada especial que reportar.","files":["b.go"]}`)

	db2, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer func() { _ = db2.Close() }()
	all, err := persistence.NewMemoryRepository(db2).ListAll(project)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	for _, m := range all {
		if m.Type == "learning" {
			t.Errorf("sin sección de aprendizajes no debía crear ninguna memoria learning: %+v", m)
		}
	}
}

// TestHookSubagentStop_StdinMalformadoNoRompe cubre S-007 (ACR feature 030):
// un JSON roto sale con código 0 (runHookWithStdinArgs falla el test si no),
// Codex sigue recibiendo JSON válido y no se extrae ningún aprendizaje.
func TestHookSubagentStop_StdinMalformadoNoRompe(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	project := persistence.ProjectKey(target)
	if _, err := persistence.NewSessionRepository(db).Start(project); err != nil {
		t.Fatalf("start session: %v", err)
	}
	_ = db.Close()

	out := runHookWithStdinArgs(t, bin, target, []string{"hook", "subagent-stop", "--emit=json"}, `{"last_assistant_message": "## Aprendizajes`)
	if strings.TrimSpace(out) != "{}" {
		t.Errorf("con stdin malformado y --emit=json esperaba \"{}\", got %q", out)
	}

	db2, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer func() { _ = db2.Close() }()
	all, err := persistence.NewMemoryRepository(db2).ListAll(project)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	for _, m := range all {
		if m.Type == "learning" {
			t.Errorf("un payload malformado no debía crear memorias learning: %+v", m)
		}
	}
}

// TestHookSubagentStop_EmitJSONImprimeSoboVacio cubre el contrato de Codex
// (SubagentStop exige JSON válido en su salida).
func TestHookSubagentStop_EmitJSONImprimeSoboVacio(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_ = db.Close()

	out := runHookWithStdinArgs(t, bin, target, []string{"hook", "subagent-stop", "--emit=json"}, subagentStopPayload)
	if strings.TrimSpace(out) != "{}" {
		t.Errorf("con --emit=json esperaba \"{}\", got %q", out)
	}
}
