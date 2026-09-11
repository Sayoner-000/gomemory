package persistence

import (
	"fmt"
	"sync"
	"testing"

	"mem/domain"
)

// TestFormSynapse_CheckpointNoSeVuelveAncla: el ancla de una sesión es el
// engrama sustantivo más reciente, nunca un checkpoint. En un proceso largo
// (servidor MCP) la caché de anclas guardaba cualquier ID recién insertado, así
// que el segundo checkpoint se enlazaba al primero en vez de a la decisión.
func TestFormSynapse_CheckpointNoSeVuelveAncla(t *testing.T) {
	db := openTestDB(t)
	SetSynapseEnabled(true)
	t.Cleanup(func() { SetSynapseEnabled(true) })

	sess, err := StartSession(db, "proj")
	if err != nil {
		t.Fatalf("iniciar sesión: %v", err)
	}
	insertar := func(tipo domain.MemoryType, titulo, contenido string) int64 {
		t.Helper()
		id, err := InsertMemory(db, &domain.Memory{
			Project: "proj", SessionID: sess.ID, Type: tipo, Title: titulo, Content: contenido,
		})
		if err != nil {
			t.Fatalf("insert %s: %v", titulo, err)
		}
		return id
	}

	decision := insertar(domain.Decision, "decisión", "la decisión que gobierna la sesión")
	cp1 := insertar(domain.Checkpoint, "Checkpoint automático", "editó a.go")
	cp2 := insertar(domain.Checkpoint, "Checkpoint automático", "editó b.go")

	enlazadas := func(a, b int64) bool {
		var n int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM memory_relations
			 WHERE (memory_id_a = ? AND memory_id_b = ?) OR (memory_id_a = ? AND memory_id_b = ?)`,
			a, b, b, a,
		).Scan(&n); err != nil {
			t.Fatalf("contar relación: %v", err)
		}
		return n > 0
	}

	if !enlazadas(cp1, decision) {
		t.Errorf("el primer checkpoint (%d) debe enlazarse a la decisión (%d)", cp1, decision)
	}
	if !enlazadas(cp2, decision) {
		t.Errorf("el segundo checkpoint (%d) debe enlazarse a la decisión (%d), no al checkpoint anterior", cp2, decision)
	}
	if enlazadas(cp2, cp1) {
		t.Errorf("hay arista checkpoint↔checkpoint (%d↔%d): un checkpoint se usó como ancla", cp2, cp1)
	}
}

// TestFormSynapse_InsercionesConcurrentes: el servidor MCP atiende llamadas en
// paralelo y todas pasan por InsertMemory. Con -race, un acceso sin sincronizar
// a la caché de anclas hace fallar este test.
func TestFormSynapse_InsercionesConcurrentes(t *testing.T) {
	db := openTestDB(t)
	SetSynapseEnabled(true)
	t.Cleanup(func() { SetSynapseEnabled(true) })

	sess, err := StartSession(db, "proj")
	if err != nil {
		t.Fatalf("iniciar sesión: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Best-effort: un SQLITE_BUSY no invalida el test; lo que se mide es
			// la ausencia de carrera en la caché.
			_, _ = InsertMemory(db, &domain.Memory{
				Project: "proj", SessionID: sess.ID, Type: domain.Learning,
				Title: fmt.Sprintf("aprendizaje %d", i), Content: fmt.Sprintf("contenido %d", i),
			})
		}(i)
	}
	wg.Wait()
}
