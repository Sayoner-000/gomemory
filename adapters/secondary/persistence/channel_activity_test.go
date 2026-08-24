package persistence

import (
	"testing"
	"time"
)

// TestChannelActivity_RegistraYRecupera cubre FR-009 de la 024: el sistema
// registra cuándo se ejerció por última vez cada canal de inyección.
//
// El informe verificaba PRESENCIA y no ACTIVIDAD: un complemento presente y
// muerto era indistinguible de uno sano.
func TestChannelActivity_RegistraYRecupera(t *testing.T) {
	db := openTestDB(t)

	if _, ok := LastChannelActivity(db, "p", "opencode", "user", "plan_entry"); ok {
		t.Fatal("un canal sin actividad no puede reportar ninguna")
	}
	if err := RecordChannelActivity(db, "p", "opencode", "user", "plan_entry"); err != nil {
		t.Fatalf("registrar: %v", err)
	}
	a, ok := LastChannelActivity(db, "p", "opencode", "user", "plan_entry")
	if !ok || a.FiredAt.IsZero() {
		t.Fatalf("esperaba actividad con fecha, obtuve %+v (ok=%v)", a, ok)
	}
}

// TestChannelActivity_RegistraElFallo cubre FR-012: las rutas de error dejan
// rastro donde el informe pueda leerlo, en vez de absorberlo en silencio.
func TestChannelActivity_RegistraElFallo(t *testing.T) {
	db := openTestDB(t)

	if err := RecordChannelError(db, "p", "opencode", "user", "plan_entry", "la operación ya no existe"); err != nil {
		t.Fatalf("registrar fallo: %v", err)
	}
	a, ok := LastChannelActivity(db, "p", "opencode", "user", "plan_entry")
	if !ok || a.LastError != "la operación ya no existe" {
		t.Fatalf("el fallo no quedó registrado: %+v", a)
	}
}

// TestChannelActivity_UnFalloNoBorraLaUltimaActividad: saber cuándo funcionó
// por última vez sigue importando después de un fallo.
func TestChannelActivity_UnFalloNoBorraLaUltimaActividad(t *testing.T) {
	db := openTestDB(t)

	RecordChannelActivity(db, "p", "opencode", "user", "plan_entry")
	RecordChannelError(db, "p", "opencode", "user", "plan_entry", "fallo posterior")

	a, _ := LastChannelActivity(db, "p", "opencode", "user", "plan_entry")
	if a.FiredAt.IsZero() {
		t.Error("el fallo borró la fecha del último uso correcto")
	}
	if a.LastError == "" {
		t.Error("no se conservó el fallo")
	}
}

// TestChannelActivity_AcotadaAlProyecto: la actividad de un proyecto no puede
// leerse como la de otro.
func TestChannelActivity_AcotadaAlProyecto(t *testing.T) {
	db := openTestDB(t)

	RecordChannelActivity(db, "p1", "opencode", "user", "plan_entry")
	if _, ok := LastChannelActivity(db, "p2", "opencode", "user", "plan_entry"); ok {
		t.Error("la actividad de un proyecto se filtró a otro")
	}
}

// TestSessionsSince cubre FR-011: distinguir un canal sin actividad por falta
// de sesiones de uno que no responde habiéndolas.
func TestSessionsSince(t *testing.T) {
	db := openTestDB(t)

	if n := SessionsSince(db, "p", time.Now().Add(-time.Hour)); n != 0 {
		t.Fatalf("sin sesiones esperaba 0, obtuve %d", n)
	}
	if _, err := StartSession(db, "p"); err != nil {
		t.Fatalf("abrir sesión: %v", err)
	}
	if n := SessionsSince(db, "p", time.Now().Add(-time.Hour)); n != 1 {
		t.Errorf("tras abrir una sesión esperaba 1, obtuve %d", n)
	}
}

// TestSessionsSince_ErrorEsDesconocidoNoCero: una consulta fallida no puede
// fingir "cero sesiones" — el informe confundiría un dato ausente con la
// evidencia de que nadie trabajó (FR-011).
func TestSessionsSince_ErrorEsDesconocidoNoCero(t *testing.T) {
	db := openTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatalf("cerrar: %v", err)
	}

	if n := SessionsSince(db, "p", time.Now().Add(-time.Hour)); n != -1 {
		t.Fatalf("consulta sobre BD cerrada debía dar -1 (desconocido), dio %d", n)
	}
}
