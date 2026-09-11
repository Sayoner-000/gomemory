package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

// runHookWithStdin ejecuta `mem hook <event>` pasando payload por stdin.
// Complementa a runHook (hook_marker_integration_test.go), que no pasa stdin.
func runHookWithStdin(t *testing.T, bin, dir, event, payload string) string {
	t.Helper()
	return runHookWithStdinArgs(t, bin, dir, []string{"hook", event}, payload)
}

// runHookWithStdinArgs es la variante que acepta argumentos extra (p. ej.
// --emit=json), para los casos que necesitan fijar el dialecto explícito.
func runHookWithStdinArgs(t *testing.T, bin, dir string, args []string, payload string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(payload)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("mem %v: %v (%s)", args, err, out.String())
	}
	return out.String()
}

// TestHookCompactSummary_PersisteSinCerrarLaSesion cubre US2 (feature 030,
// capacidad C6): `mem hook compact-summary` persiste el resumen que produjo
// la compactación en la sesión activa, SIN cerrarla.
func TestHookCompactSummary_PersisteSinCerrarLaSesion(t *testing.T) {
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
	sess, err := sessRepo.Start(project)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	_ = db.Close()

	out := runHookWithStdin(t, bin, target, "compact-summary", `{"compact_summary":"Resumen de la compactación"}`)
	if strings.TrimSpace(out) != "" {
		t.Errorf("compact-summary (sin --emit) no debía imprimir nada, got %q", out)
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
		t.Fatal("la sesión debía seguir activa")
	}
	if active.ID != sess.ID {
		t.Fatalf("no debía abrir una sesión nueva teniendo una activa: original=%s got=%s", sess.ID, active.ID)
	}
	if active.Summary != "Resumen de la compactación" {
		t.Errorf("summary = %q", active.Summary)
	}
}

// TestHookCompactSummary_AceptaClaveSummary cubre la forma alternativa del
// payload ({"summary": ...}), para integraciones que no usan el campo del
// cliente (contracts/hooks.md).
func TestHookCompactSummary_AceptaClaveSummary(t *testing.T) {
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

	runHookWithStdin(t, bin, target, "compact-summary", `{"summary":"vía campo alternativo"}`)

	db2, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer func() { _ = db2.Close() }()
	active, err := persistence.NewSessionRepository(db2).Active(project)
	if err != nil {
		t.Fatalf("active session: %v", err)
	}
	if active == nil || active.Summary != "vía campo alternativo" {
		t.Errorf("summary no se persistió por la clave alternativa: %+v", active)
	}
}

// activeSessionAfterCompactSummary prepara un proyecto (con sesión activa si
// withSession), envía payload a `mem hook compact-summary` y devuelve la
// sesión activa resultante (nil si no hay ninguna).
func activeSessionAfterCompactSummary(t *testing.T, payload string, withSession bool) *domain.Session {
	t.Helper()
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
	if withSession {
		if _, err := persistence.NewSessionRepository(db).Start(project); err != nil {
			t.Fatalf("start session: %v", err)
		}
	}
	_ = db.Close()

	runHookWithStdin(t, bin, target, "compact-summary", payload)

	db2, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer func() { _ = db2.Close() }()
	active, err := persistence.NewSessionRepository(db2).Active(project)
	if err != nil {
		t.Fatalf("active session: %v", err)
	}
	return active
}

// TestHookCompactSummary_AmbasClavesGanaCompactSummary fija la precedencia de
// contracts/hooks.md (S-003, ACR feature 030): con las dos claves presentes y
// distintas, se persiste la del cliente.
func TestHookCompactSummary_AmbasClavesGanaCompactSummary(t *testing.T) {
	active := activeSessionAfterCompactSummary(t, `{"compact_summary":"del cliente","summary":"neutral"}`, true)
	if active == nil || active.Summary != "del cliente" {
		t.Errorf("con ambas claves debía ganar compact_summary: %+v", active)
	}
}

// TestHookCompactSummary_CompactSummaryEnBlancoUsaSummary cubre la otra mitad
// de la precedencia: un compact_summary en blanco no debe descartar un
// summary con contenido.
func TestHookCompactSummary_CompactSummaryEnBlancoUsaSummary(t *testing.T) {
	active := activeSessionAfterCompactSummary(t, `{"compact_summary":"  ","summary":"neutral"}`, true)
	if active == nil || active.Summary != "neutral" {
		t.Errorf("compact_summary en blanco debía ceder a summary: %+v", active)
	}
}

// TestHookCompactSummary_StdinMalformadoNoRompe cubre S-007 (ACR feature 030):
// el hook es best-effort — un JSON roto sale con código 0 (runHookWithStdin
// falla el test si no) y no abre ni toca ninguna sesión.
func TestHookCompactSummary_StdinMalformadoNoRompe(t *testing.T) {
	active := activeSessionAfterCompactSummary(t, `{"compact_summary": "sin cerrar`, false)
	if active != nil {
		t.Errorf("un payload malformado no debía abrir ninguna sesión, got %+v", active)
	}
}

// TestHookCompactSummary_SinSesionActivaAbreUna cubre R4: nunca debe perderse
// un resumen por no haber sesión previa.
func TestHookCompactSummary_SinSesionActivaAbreUna(t *testing.T) {
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
	_ = db.Close()

	runHookWithStdin(t, bin, target, "compact-summary", `{"compact_summary":"primer resumen sin sesión previa"}`)

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
		t.Fatal("compact-summary debía abrir una sesión cuando no había ninguna activa")
	}
	if active.Summary != "primer resumen sin sesión previa" {
		t.Errorf("summary = %q", active.Summary)
	}
}

// TestHookCompactSummary_VacioNoHaceNada cubre el edge case: un resumen vacío
// no debe crear una sesión ni cambiar nada.
func TestHookCompactSummary_VacioNoHaceNada(t *testing.T) {
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
	_ = db.Close()

	runHookWithStdin(t, bin, target, "compact-summary", `{"compact_summary":""}`)

	db2, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer func() { _ = db2.Close() }()
	active, err := persistence.NewSessionRepository(db2).Active(project)
	if err != nil {
		t.Fatalf("active session: %v", err)
	}
	if active != nil {
		t.Fatalf("un resumen vacío no debía abrir ni tocar ninguna sesión, got %+v", active)
	}
}

// TestHookCompactSummary_EmitJSONImprimeSoboVacio cubre el contrato de
// dialecto: con --emit=json debe imprimir "{}" (Codex valida JSON aun sin
// contenido que aportar al modelo).
func TestHookCompactSummary_EmitJSONImprimeSoboVacio(t *testing.T) {
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

	out := runHookWithStdinArgs(t, bin, target, []string{"hook", "compact-summary", "--emit=json"}, `{"compact_summary":"x"}`)
	if strings.TrimSpace(out) != "{}" {
		t.Errorf("con --emit=json esperaba \"{}\", got %q", out)
	}
}
