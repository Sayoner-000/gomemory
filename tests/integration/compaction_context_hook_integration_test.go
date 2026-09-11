package main

import (
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

// TestHookCompactionContext_ConSesionActivaIncluyeOrdenYPuntero cubre US1,
// capacidad C1 (feature 030): `mem hook compaction-context` es el subcomando
// que usan las integraciones que aportan texto al compresor ANTES de
// compactar (p. ej. el plugin de OpenCode en experimental.session.compacting).
// Debe emitir el contexto de la sesión activa seguido de la orden de
// persistir el resumen.
func TestHookCompactionContext_ConSesionActivaIncluyeOrdenYPuntero(t *testing.T) {
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
	sessRepo := persistence.NewSessionRepository(db)
	if _, err := sessRepo.Start(project); err != nil {
		t.Fatalf("start session: %v", err)
	}
	mem := &domain.Memory{Project: project, SessionID: "", Type: domain.Decision,
		Title: "Decisión de prueba", Content: "contenido de prueba"}
	// La sesión activa ya la abrimos arriba; InsertMemory adjunta el
	// session_id de la sesión activa del proyecto automáticamente cuando
	// SessionID viene vacío... no: InsertMemory solo hereda origin_prompt de
	// la sesión activa, no session_id. Se fija explícitamente al valor real.
	active, err := sessRepo.Active(project)
	if err != nil || active == nil {
		t.Fatalf("sesión activa: %v", err)
	}
	mem.SessionID = active.ID
	if _, err := persistence.NewMemoryRepository(db).Insert(mem); err != nil {
		t.Fatalf("insert memory: %v", err)
	}
	_ = db.Close()

	out := runHook(t, bin, target, "compaction-context")

	if !strings.Contains(out, "Memoria de esta sesión") {
		t.Fatalf("debía incluir el encabezado de memoria de sesión: %q", out)
	}
	if !strings.Contains(out, "Decisión de prueba") {
		t.Fatalf("debía incluir la memoria guardada en la sesión: %q", out)
	}
	if !strings.Contains(out, "get_memory") {
		t.Fatalf("debía incluir el puntero al detalle íntegro: %q", out)
	}
	if !strings.Contains(out, "PRIMERA ACCIÓN") || !strings.Contains(out, "save_session_summary") {
		t.Fatalf("debía incluir la orden de persistir el resumen: %q", out)
	}
}

// TestHookCompactionContext_SinSesionActivaSoloEmiteLaOrden cubre el edge
// case: sin sesión activa no hay contexto de sesión que mostrar, pero la
// orden de persistir el resumen se emite igual.
func TestHookCompactionContext_SinSesionActivaSoloEmiteLaOrden(t *testing.T) {
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

	out := runHook(t, bin, target, "compaction-context")

	if strings.Contains(out, "## Memoria de esta sesión") {
		t.Fatalf("sin sesión activa no debía incluir el encabezado de sesión: %q", out)
	}
	if !strings.Contains(out, "save_session_summary") {
		t.Fatalf("debía incluir la orden de persistir el resumen aun sin sesión activa: %q", out)
	}
}
