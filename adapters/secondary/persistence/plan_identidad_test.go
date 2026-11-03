package persistence

import (
	"testing"

	"mem/domain"
)

// ADM-02: dos decisiones con el mismo título y distinto topic_key coexisten; el
// contenido del primer plan no se reescribe.
func TestPlanIdentidad_MismoTituloDistintoTopicKeyCoexisten(t *testing.T) {
	db, _ := abrirBaseDeBorrado(t)
	a, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Decision, Title: "Plan aprobado: Plan", Content: "plan A", TopicKey: "plan:aaaaaaaaaaaaaaaa"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Decision, Title: "Plan aprobado: Plan", Content: "plan B", TopicKey: "plan:bbbbbbbbbbbbbbbb"})
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("el segundo plan reescribió al primero (id %d)", a)
	}
	var contenido string
	if err := db.QueryRow(`SELECT content FROM memories WHERE id = ?`, a).Scan(&contenido); err != nil {
		t.Fatal(err)
	}
	if contenido != "plan A" {
		t.Fatalf("el plan A quedó con %q", contenido)
	}
}

// ADM-03: con el dedup por identidad desactivado, el reenvío exacto de un plan
// no crea una fila nueva gracias al topic_key.
func TestPlanIdentidad_ReenvioExactoNoDuplicaSinDedup(t *testing.T) {
	SetDedupWindowDays(-1)
	t.Cleanup(func() { SetDedupWindowDays(DefaultDedupWindowDays) })
	db, _ := abrirBaseDeBorrado(t)
	m := domain.Memory{Project: "proj", Type: domain.Decision, Title: "Plan aprobado: Plan", Content: "plan", TopicKey: "plan:cccccccccccccccc"}
	primero, err := InsertMemory(db, &m)
	if err != nil {
		t.Fatal(err)
	}
	reenvio := m
	segundo, err := InsertMemory(db, &reenvio)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM memories WHERE title = 'Plan aprobado: Plan'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if primero != segundo || n != 1 {
		t.Fatalf("reenvío duplicado: ids %d/%d, filas %d", primero, segundo, n)
	}
}
