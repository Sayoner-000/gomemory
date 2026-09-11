package persistence

import (
	"strings"
	"testing"
)

// TestUpdateSessionSummary_NoCierraLaSesion cubre el cierre del defecto
// latente de research.md R4: guardar el resumen compactado NUNCA debe tocar
// ended_at. Antes de esta feature, la única vía para persistir un resumen era
// EndSession, que sí la cierra — y hookPostCompact no abría una sesión
// nueva, así que toda memoria guardada después de compactar quedaba sin
// sesión asociada.
func TestUpdateSessionSummary_NoCierraLaSesion(t *testing.T) {
	db := openTestDB(t)

	sess, err := StartSession(db, "proj")
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	if err := UpdateSessionSummary(db, sess.ID, "resumen compactado v1"); err != nil {
		t.Fatalf("UpdateSessionSummary: %v", err)
	}

	active, err := ActiveSession(db, "proj")
	if err != nil {
		t.Fatalf("active session: %v", err)
	}
	if active == nil {
		t.Fatal("la sesión debía seguir activa tras UpdateSessionSummary")
	}
	if active.Summary != "resumen compactado v1" {
		t.Errorf("summary = %q, se esperaba %q", active.Summary, "resumen compactado v1")
	}
	if active.EndedAt != nil {
		t.Errorf("ended_at debía seguir NULL, got %v", *active.EndedAt)
	}
}

// TestUpdateSessionSummary_Idempotente cubre FR-009: persistir el mismo texto
// dos veces no debe fallar ni cambiar nada más que el summary (que ya es
// idéntico).
func TestUpdateSessionSummary_Idempotente(t *testing.T) {
	db := openTestDB(t)

	sess, err := StartSession(db, "proj")
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	if err := UpdateSessionSummary(db, sess.ID, "mismo resumen"); err != nil {
		t.Fatalf("primera UpdateSessionSummary: %v", err)
	}
	if err := UpdateSessionSummary(db, sess.ID, "mismo resumen"); err != nil {
		t.Fatalf("segunda UpdateSessionSummary (idempotente): %v", err)
	}

	active, err := ActiveSession(db, "proj")
	if err != nil {
		t.Fatalf("active session: %v", err)
	}
	if active.Summary != "mismo resumen" {
		t.Errorf("summary tras repetir = %q, se esperaba %q", active.Summary, "mismo resumen")
	}
}

// TestUpdateSessionSummary_SesionCerradaOInexistenteFalla protege la misma
// semántica que EndSession: actualizar el resumen de una sesión que ya
// terminó, o que no existe, es un error explícito, no un no-op silencioso.
func TestUpdateSessionSummary_SesionCerradaOInexistenteFalla(t *testing.T) {
	db := openTestDB(t)

	sess, err := StartSession(db, "proj")
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if err := EndSession(db, sess.ID, "cierre"); err != nil {
		t.Fatalf("end session: %v", err)
	}

	if err := UpdateSessionSummary(db, sess.ID, "no debería aplicar"); err == nil {
		t.Fatal("UpdateSessionSummary sobre una sesión ya cerrada debía fallar")
	} else if !strings.Contains(err.Error(), sess.ID) {
		t.Errorf("el error debía identificar la sesión: %v", err)
	}

	if err := UpdateSessionSummary(db, "no-existe", "texto"); err == nil {
		t.Fatal("UpdateSessionSummary sobre una sesión inexistente debía fallar")
	}
}
