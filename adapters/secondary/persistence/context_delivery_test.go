package persistence

import "testing"

// TestContextDelivery_RegistraYRecupera cubre FR-013 de la especificación 023:
// el sistema registra qué entregó cada canal en una sesión.
func TestContextDelivery_RegistraYRecupera(t *testing.T) {
	db := openTestDB(t)

	if _, ok := LastContextDelivery(db, "sesion-1", "context"); ok {
		t.Fatal("una sesión sin entregas previas no puede reportar ninguna")
	}

	if err := RecordContextDelivery(db, "sesion-1", "context", "hash-a"); err != nil {
		t.Fatalf("registrar entrega: %v", err)
	}

	h, ok := LastContextDelivery(db, "sesion-1", "context")
	if !ok || h != "hash-a" {
		t.Fatalf("esperaba hash-a, obtuve %q (ok=%v)", h, ok)
	}
}

// TestContextDelivery_AcotadaALaSesion cubre FR-012: lo entregado en una sesión
// no puede suprimir material en otra.
//
// Sin esta separación, abrir una sesión nueva heredaría la creencia de que el
// contexto ya se entregó, y el agente arrancaría sin él.
func TestContextDelivery_AcotadaALaSesion(t *testing.T) {
	db := openTestDB(t)

	if err := RecordContextDelivery(db, "sesion-1", "context", "hash-a"); err != nil {
		t.Fatalf("registrar: %v", err)
	}
	if _, ok := LastContextDelivery(db, "sesion-2", "context"); ok {
		t.Error("la entrega de una sesión se filtró a otra")
	}
}

// TestContextDelivery_LaUltimaEntregaGana: si el contenido cambia dentro de la
// misma sesión, lo que cuenta es la última entrega.
func TestContextDelivery_LaUltimaEntregaGana(t *testing.T) {
	db := openTestDB(t)

	RecordContextDelivery(db, "sesion-1", "context", "hash-a")
	RecordContextDelivery(db, "sesion-1", "context", "hash-b")

	if h, _ := LastContextDelivery(db, "sesion-1", "context"); h != "hash-b" {
		t.Errorf("esperaba hash-b, obtuve %q", h)
	}
}

// TestContextDelivery_CanalesIndependientes: el contexto general y el de
// planificación son canales distintos y no se pisan.
func TestContextDelivery_CanalesIndependientes(t *testing.T) {
	db := openTestDB(t)

	RecordContextDelivery(db, "sesion-1", "context", "hash-a")
	if _, ok := LastContextDelivery(db, "sesion-1", "plan_context"); ok {
		t.Error("el registro de un canal se leyó como el de otro")
	}
}
