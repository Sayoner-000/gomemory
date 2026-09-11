package main

import (
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

// TestHookPostCompact_AbreSesionSiNoHayActiva cubre el cierre del defecto
// latente de research.md R4 (feature 030): antes de este cambio, la
// recuperación tras compactar pedía end_session (que cierra la sesión) y
// hookPostCompact no abría una nueva, así que toda memoria guardada después
// de la primera compactación quedaba sin sesión asociada. Este test simula
// exactamente ese estado previo (sesión ya cerrada) y verifica que
// post-compact la reabre.
func TestHookPostCompact_AbreSesionSiNoHayActiva(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sessRepo := persistence.NewSessionRepository(db)
	project := persistence.ProjectKey(target)
	sess, err := sessRepo.Start(project)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if err := sessRepo.End(sess.ID, "sesión ya cerrada antes de compactar"); err != nil {
		t.Fatalf("end session: %v", err)
	}
	_ = db.Close()

	out := runHook(t, bin, target, "post-compact")

	if !strings.Contains(out, "PRIMERA ACCIÓN REQUERIDA") {
		t.Fatalf("post-compact debía emitir los pasos de recuperación: %q", out)
	}
	if strings.Contains(out, "end_session") {
		t.Fatalf("post-compact NO debe pedir end_session (cerraría la sesión de nuevo): %q", out)
	}
	if !strings.Contains(out, "save_session_summary") {
		t.Fatalf("post-compact debe pedir save_session_summary como primer paso: %q", out)
	}

	db2, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer func() { _ = db2.Close() }()
	active, err := persistence.NewSessionRepository(db2).Active(project)
	if err != nil {
		t.Fatalf("active session: %v", err)
	}
	if active == nil {
		t.Fatal("post-compact debía dejar una sesión activa aunque no hubiera ninguna al empezar")
	}
}

// TestHookPostCompact_ConSesionActivaNoAbreOtra protege el caso normal: si ya
// hay una sesión activa (el caso más común), post-compact no debe abrir una
// segunda.
func TestHookPostCompact_ConSesionActivaNoAbreOtra(t *testing.T) {
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
	sess, err := persistence.NewSessionRepository(db).Start(project)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	_ = db.Close()

	runHook(t, bin, target, "post-compact")

	db2, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer func() { _ = db2.Close() }()
	active, err := persistence.NewSessionRepository(db2).Active(project)
	if err != nil {
		t.Fatalf("active session: %v", err)
	}
	if active == nil {
		t.Fatal("debía seguir habiendo una sesión activa")
	}
	if active.ID != sess.ID {
		t.Fatalf("post-compact no debía abrir una sesión nueva teniendo una activa: original=%s got=%s",
			sess.ID, active.ID)
	}
}

// TestHookPostCompact_SinMemoriaDeSesionMuestraNota cubre el edge case de
// US1 (spec.md): una sesión activa sin memorias propias debe mostrar una
// nota explícita, no un bloque vacío.
func TestHookPostCompact_SinMemoriaDeSesionMuestraNota(t *testing.T) {
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

	out := runHook(t, bin, target, "post-compact")
	if !strings.Contains(out, "Memoria de esta sesión") {
		t.Fatalf("debía incluir el encabezado de memoria de sesión: %q", out)
	}
	if !strings.Contains(out, "aún no guardó memorias propias") {
		t.Fatalf("sesión sin memorias propias debía mostrar la nota explícita: %q", out)
	}
}
