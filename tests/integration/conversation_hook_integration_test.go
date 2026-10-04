package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

const avisoAgente = "AVISO DE MEMORIA"

// T053 (feature 035, US3): el «checkpoint pegado». En Codex, turn-end deja el
// aviso de compactación pendiente; la conversación termina; antes, el segundo
// prompt de la conversación SIGUIENTE lo recibía.
func TestConversation_AvisoPendienteNoCruzaDeConversacion(t *testing.T) {
	bin := buildMemBinary(t)
	target := setUpAgentNoticeProject(t, true)

	runHookWithStdin(t, bin, target, "session-start", `{"session_id":"A"}`)
	runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit", "--emit=json"}, `{"prompt":"uno"}`)
	_ = os.WriteFile(filepath.Join(target, ".memory", ".footprint"), []byte("60000"), 0o644)
	runHookWithStdinArgs(t, bin, target, []string{"hook", "turn-end", "--emit=json"}, `{}`)
	if _, err := os.Stat(filepath.Join(target, ".memory", ".pending-agent-notice")); err != nil {
		t.Fatalf("precondición: la conversación A debía dejar el aviso pendiente: %v", err)
	}

	runHookWithStdin(t, bin, target, "session-start", `{"session_id":"B"}`)
	for i, p := range []string{`{"prompt":"b1"}`, `{"prompt":"b2"}`, `{"prompt":"b3"}`} {
		if out := runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit", "--emit=json"}, p); strings.Contains(out, avisoAgente) {
			t.Errorf("prompt %d de la conversación B recibió el aviso de A: %q", i+1, out)
		}
	}
}

func TestConversation_AvisoDeLaMismaConversacionLlegaEnElPrimerPrompt(t *testing.T) {
	bin := buildMemBinary(t)
	target := setUpAgentNoticeProject(t, true)

	runHookWithStdin(t, bin, target, "session-start", `{"session_id":"A"}`)
	_ = os.WriteFile(filepath.Join(target, ".memory", ".footprint"), []byte("60000"), 0o644)
	runHookWithStdinArgs(t, bin, target, []string{"hook", "turn-end", "--emit=json"}, `{}`)
	// Reanudación: mismo session_id. El primer prompt tras el arranque es el
	// que debe entregar el aviso, y una sola vez.
	runHookWithStdin(t, bin, target, "session-start", `{"session_id":"A"}`)
	primero := runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit", "--emit=json"}, `{"prompt":"x"}`)
	if !strings.Contains(primero, avisoAgente) {
		t.Errorf("el primer prompt tras el arranque debe entregar el aviso pendiente de su conversación: %q", firstN(primero, 300))
	}
	if segundo := runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit", "--emit=json"}, `{"prompt":"y"}`); strings.Contains(segundo, avisoAgente) {
		t.Error("el aviso se entrega una sola vez")
	}
}

func TestConversation_ReanudacionNoReiniciaElEstado(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	runHookWithStdin(t, bin, target, "session-start", `{"session_id":"A"}`)
	huella := filepath.Join(target, ".memory", ".footprint")
	_ = os.WriteFile(huella, []byte("123"), 0o644)

	runHookWithStdin(t, bin, target, "session-start", `{"session_id":"A"}`)
	if _, err := os.Stat(huella); err != nil {
		t.Error("una reanudación (mismo session_id) no reinicia la huella")
	}
	runHookWithStdin(t, bin, target, "session-start", `{"session_id":"B"}`)
	if _, err := os.Stat(huella); !os.IsNotExist(err) {
		t.Error("una conversación nueva reinicia la huella")
	}
}

func firstN(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// openProjectStore abre la base del proyecto en el almacén aislado de la suite.
func openProjectStore(t *testing.T, target string) *sql.DB {
	t.Helper()
	db, err := persistence.Open(target)
	if err != nil || db == nil {
		t.Fatalf("abrir el almacén del proyecto: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// T055 (feature 035, FR-021): Codex no tiene fin de sesión; una sesión sin
// actividad durante más de 4 h se cierra al empezar una conversación nueva.
func TestConversation_RotaLaSesionInactiva(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	runMem(t, bin, target, "session", "start")
	runMem(t, bin, target, "save", "-t", "decisión vieja", "-y", "decision", "una decisión de hace horas")
	db := openProjectStore(t, target)
	var oldID string
	if err := db.QueryRow(`SELECT id FROM sessions WHERE ended_at IS NULL`).Scan(&oldID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE memories SET created_at = datetime('now','-5 hours','-5 hours'), updated_at = datetime('now','-5 hours','-5 hours');
		UPDATE sessions SET created_at = datetime('now','-5 hours','-6 hours')`); err != nil {
		t.Fatal(err)
	}

	runHookWithStdin(t, bin, target, "session-start", `{"session_id":"nueva"}`)

	var ended sql.NullString
	var summary string
	if err := db.QueryRow(`SELECT ended_at, summary FROM sessions WHERE id = ?`, oldID).Scan(&ended, &summary); err != nil {
		t.Fatal(err)
	}
	if !ended.Valid || !strings.Contains(summary, "inactividad") {
		t.Errorf("la sesión inactiva debía cerrarse con el resumen automático: ended=%v summary=%q", ended, summary)
	}
	var activas int
	_ = db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE ended_at IS NULL`).Scan(&activas)
	if activas != 1 {
		t.Errorf("debía quedar exactamente una sesión activa nueva, hay %d", activas)
	}
}

func TestConversation_SinIdRotaTrasInactividad(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	runMem(t, bin, target, "session", "start")
	before, ok := readConversationFile(t, target)
	if !ok {
		t.Fatal("la primera sesión debe abrir conversación local")
	}
	runMem(t, bin, target, "save", "-t", "actividad vieja", "-y", "decision", "contenido")
	db := openProjectStore(t, target)
	if _, err := db.Exec(`UPDATE memories SET created_at = datetime('now','-5 hours','-5 hours'), updated_at = datetime('now','-5 hours','-5 hours');
		UPDATE sessions SET created_at = datetime('now','-5 hours','-6 hours')`); err != nil {
		t.Fatal(err)
	}
	huella := filepath.Join(target, ".memory", ".footprint")
	_ = os.WriteFile(huella, []byte("123"), 0o600)
	runHookWithStdin(t, bin, target, "session-start", `{}`)
	after, ok := readConversationFile(t, target)
	if !ok || after["id"] == before["id"] {
		t.Fatalf("la inactividad debe abrir otra conversación local: %v, %v", before, after)
	}
	if _, err := os.Stat(huella); !os.IsNotExist(err) {
		t.Fatal("el estado por turno de la conversación vieja debe limpiarse")
	}
}

func TestConversation_NoRotaConActividadReciente(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	runMem(t, bin, target, "session", "start")
	runMem(t, bin, target, "save", "-t", "decisión reciente", "-y", "decision", "una decisión de hace minutos")
	db := openProjectStore(t, target)
	var id string
	_ = db.QueryRow(`SELECT id FROM sessions WHERE ended_at IS NULL`).Scan(&id)

	runHookWithStdin(t, bin, target, "session-start", `{"session_id":"otra"}`)
	var ended sql.NullString
	_ = db.QueryRow(`SELECT ended_at FROM sessions WHERE id = ?`, id).Scan(&ended)
	if ended.Valid {
		t.Error("con actividad reciente no se rota: otro agente puede estar trabajando en el proyecto")
	}
}
