package persistence

import (
	"fmt"
	"strings"
	"testing"

	"mem/domain"
)

// TestListMemoriesBySession_FiltraPorSesionYProyecto cubre US1 (feature 030):
// solo deben aparecer las memorias de esa sesión concreta, dentro de ese
// proyecto, de la más reciente a la más antigua, respetando limit.
func TestListMemoriesBySession_FiltraPorSesionYProyecto(t *testing.T) {
	db := openTestDB(t)

	sessA, err := StartSession(db, "proj")
	if err != nil {
		t.Fatalf("start session A: %v", err)
	}
	sessB, err := StartSession(db, "proj")
	if err != nil {
		t.Fatalf("start session B: %v", err)
	}

	insert := func(sessionID, title string) {
		m := &domain.Memory{Project: "proj", SessionID: sessionID, Type: domain.Learning, Title: title, Content: title}
		if _, err := InsertMemory(db, m); err != nil {
			t.Fatalf("insert %s: %v", title, err)
		}
	}
	insert(sessA.ID, "de la sesión A - primera")
	insert(sessA.ID, "de la sesión A - segunda")
	insert(sessB.ID, "de la sesión B")
	// De otro proyecto, misma sesión hipotética: no debe colarse.
	otherProj := &domain.Memory{Project: "otro-proyecto", SessionID: sessA.ID, Type: domain.Learning,
		Title: "de otro proyecto", Content: "x"}
	if _, err := InsertMemory(db, otherProj); err != nil {
		t.Fatalf("insert otro proyecto: %v", err)
	}

	got, err := ListMemoriesBySession(db, "proj", sessA.ID, 100)
	if err != nil {
		t.Fatalf("ListMemoriesBySession: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("esperaba 2 memorias de la sesión A, got %d: %+v", len(got), got)
	}
	// Más reciente primero.
	if got[0].Title != "de la sesión A - segunda" || got[1].Title != "de la sesión A - primera" {
		t.Errorf("orden inesperado: %+v", got)
	}
	for _, m := range got {
		if m.SessionID != sessA.ID {
			t.Errorf("memoria de otra sesión coló: %+v", m)
		}
	}
}

// TestListMemoriesBySession_RespetaLimit verifica que el tope se aplique.
func TestListMemoriesBySession_RespetaLimit(t *testing.T) {
	db := openTestDB(t)

	sess, err := StartSession(db, "proj")
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	for i := 0; i < 5; i++ {
		// Contenido distinto en cada iteración: mismo título+tipo con contenido
		// idéntico dispararía el dedup por identidad (feature 008, FR-013) y
		// las 5 inserciones colapsarían en una sola fila.
		m := &domain.Memory{Project: "proj", SessionID: sess.ID, Type: domain.Learning,
			Title: fmt.Sprintf("entrada %d", i), Content: fmt.Sprintf("contenido %d", i)}
		if _, err := InsertMemory(db, m); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	got, err := ListMemoriesBySession(db, "proj", sess.ID, 2)
	if err != nil {
		t.Fatalf("ListMemoriesBySession: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("esperaba 2 memorias (limit), got %d", len(got))
	}
}

// TestListMemoriesBySession_SessionIDVacioDevuelveVacio cubre el contrato:
// nunca debe interpretarse como "todas las memorias del proyecto".
func TestListMemoriesBySession_SessionIDVacioDevuelveVacio(t *testing.T) {
	db := openTestDB(t)

	m := &domain.Memory{Project: "proj", Type: domain.Learning, Title: "sin sesión", Content: "x"}
	if _, err := InsertMemory(db, m); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := ListMemoriesBySession(db, "proj", "", 100)
	if err != nil {
		t.Fatalf("ListMemoriesBySession: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("sessionID vacío debía devolver lista vacía, got %d: %+v", len(got), got)
	}
}

// TestListMemoriesBySession_UsaIndiceCompuesto cierra S-001 (ACR feature 030):
// el filtro por (project, session_id) y el orden por created_at DESC, id DESC
// deben resolverse con idx_memories_project_session, sin recorrer todas las
// memorias del proyecto ni ordenar en un B-tree temporal.
func TestListMemoriesBySession_UsaIndiceCompuesto(t *testing.T) {
	db := openTestDB(t)

	rows, err := db.Query(
		`EXPLAIN QUERY PLAN SELECT id FROM memories
		 WHERE project = ? AND session_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`,
		"proj", "sess", 200,
	)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var plan []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("scan: %v", err)
		}
		plan = append(plan, detail)
	}
	joined := strings.Join(plan, " | ")
	if !strings.Contains(joined, "idx_memories_project_session") {
		t.Errorf("la query debía usar idx_memories_project_session, plan: %s", joined)
	}
	if strings.Contains(joined, "TEMP B-TREE") {
		t.Errorf("el orden debía salir del índice, sin B-tree temporal, plan: %s", joined)
	}
}
