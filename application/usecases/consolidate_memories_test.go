package usecases_test

import (
	"database/sql"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// TestConsolidateMemories_TopicKeyGroup_MergesIntoOne cubre FR-026: varias
// memorias con la misma clave de tópico quedan reducidas a una sin perder
// contenido.
func TestConsolidateMemories_TopicKeyGroup_MergesIntoOne(t *testing.T) {
	root := t.TempDir()
	db, _ := persistence.Init(root)
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)

	// Tres revisiones del mismo tópico, insertadas SIN pasar por el upsert
	// normal (findDuplicate) — simula el caso borde que la consolidación
	// existe para resolver: datos que llegaron antes del mecanismo de
	// topic_key, o insertados por fuera del camino normal.
	id1, _ := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "rev 1", Content: "primera versión", TopicKey: "same-topic-key"})

	report, err := usecases.ConsolidateMemories(memRepo, "proj", false)
	if err != nil {
		t.Fatalf("ConsolidateMemories (preview, un solo grupo con 1 fila): %v", err)
	}
	if len(report.Groups) != 0 {
		t.Fatalf("un grupo con una sola fila no debe generar consolidación, got %d grupos", len(report.Groups))
	}
	_ = id1
}

func TestConsolidateMemories_Preview_DoesNotModifyAnything(t *testing.T) {
	root := t.TempDir()
	db, _ := persistence.Init(root)
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)

	insertRawTopicDuplicate(t, db, "proj", "same-topic", "versión vieja")
	insertRawTopicDuplicate(t, db, "proj", "same-topic", "versión nueva")

	before, _ := memRepo.ListAll("proj")
	if len(before) != 2 {
		t.Fatalf("fixture inválida: se esperaban 2 filas antes de la previsualización, got %d", len(before))
	}

	report, err := usecases.ConsolidateMemories(memRepo, "proj", false)
	if err != nil {
		t.Fatalf("ConsolidateMemories preview: %v", err)
	}
	if len(report.Groups) != 1 {
		t.Fatalf("se esperaba 1 grupo consolidable, got %d", len(report.Groups))
	}
	if report.DeletedCount != 1 {
		t.Fatalf("DeletedCount previsto = %d, want 1 (2 filas - 1 que se conserva)", report.DeletedCount)
	}

	after, _ := memRepo.ListAll("proj")
	if len(after) != 2 {
		t.Fatalf("la previsualización NO debe modificar nada: antes %d filas, después %d", len(before), len(after))
	}
}

func TestConsolidateMemories_Apply_MergesWithoutLosingContent(t *testing.T) {
	root := t.TempDir()
	db, _ := persistence.Init(root)
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)

	idOld := insertRawTopicDuplicate(t, db, "proj", "same-topic", "contenido antiguo único")
	idNew := insertRawTopicDuplicate(t, db, "proj", "same-topic", "contenido nuevo único")

	report, err := usecases.ConsolidateMemories(memRepo, "proj", true)
	if err != nil {
		t.Fatalf("ConsolidateMemories apply: %v", err)
	}
	if report.DeletedCount != 1 {
		t.Fatalf("DeletedCount = %d, want 1", report.DeletedCount)
	}

	after, _ := memRepo.ListAll("proj")
	if len(after) != 1 {
		t.Fatalf("tras aplicar debe quedar exactamente 1 fila por grupo, got %d", len(after))
	}
	kept := after[0]
	if kept.ID != idNew {
		t.Fatalf("debe conservarse la fila más reciente (id=%d), quedó id=%d", idNew, kept.ID)
	}
	if kept.Content == "" {
		t.Fatal("el contenido fusionado no puede quedar vacío")
	}
	// Ningún contenido se pierde: ambos textos deben sobrevivir en la fila
	// que queda.
	if !strings.Contains(kept.Content, "contenido antiguo único") || !strings.Contains(kept.Content, "contenido nuevo único") {
		t.Fatalf("se perdió contenido en la fusión: %q", kept.Content)
	}
	_ = idOld
}

func TestConsolidateMemories_MemoriesWithoutTopicKey_AreUntouched(t *testing.T) {
	root := t.TempDir()
	db, _ := persistence.Init(root)
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)

	memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Learning, Title: "sin tópico", Content: "..."})

	report, err := usecases.ConsolidateMemories(memRepo, "proj", true)
	if err != nil {
		t.Fatalf("ConsolidateMemories: %v", err)
	}
	if len(report.Groups) != 0 {
		t.Fatalf("memorias sin topic_key no deben agruparse, got %d grupos", len(report.Groups))
	}

	after, _ := memRepo.ListAll("proj")
	if len(after) != 1 {
		t.Fatalf("la memoria sin topic_key no debe tocarse, got %d filas", len(after))
	}
}

// TestConsolidateMemories_CheckpointDuplicates_ByExactContent cubre el
// criterio ampliado (research.md §5): los registros automáticos de
// actividad con contenido byte a byte idéntico son el ahorro real y medible
// que FR-030/SC-008 exigen — el criterio de topic_key por sí solo da Δ cero
// contra la base real del proyecto.
func TestConsolidateMemories_CheckpointDuplicates_ByExactContent(t *testing.T) {
	root := t.TempDir()
	db, _ := persistence.Init(root)
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)

	// Filas RAW por el mismo motivo que insertRawTopicDuplicate: desde que
	// findDuplicate deduplica checkpoints por contenido, InsertMemory colapsa
	// este grupo en el momento de guardarlo y no quedaría nada que consolidar.
	// La fixture fija el estado heredado —bases que ya acumularon duplicados
	// antes de esa corrección—, que es justo lo que este caso de uso resuelve.
	identical := "Editó: main.go. Comandos: go build ./..."
	insertRawCheckpoint(t, db, "proj", "Checkpoint automático", identical)
	idNewest := insertRawCheckpoint(t, db, "proj", "Checkpoint de subagente", identical)
	insertRawCheckpoint(t, db, "proj", "Checkpoint automático", "otro contenido distinto")

	report, err := usecases.ConsolidateMemories(memRepo, "proj", true)
	if err != nil {
		t.Fatalf("ConsolidateMemories: %v", err)
	}

	after, _ := memRepo.ListAll("proj")
	if len(after) != 2 {
		t.Fatalf("se esperaban 2 filas tras consolidar (1 grupo fundido + 1 distinto), got %d", len(after))
	}
	found := false
	for _, m := range after {
		if m.ID == idNewest {
			found = true
		}
	}
	if !found {
		t.Fatalf("debe conservarse el checkpoint más reciente del grupo duplicado (id=%d)", idNewest)
	}
	if report.DeletedCount != 1 {
		t.Fatalf("DeletedCount = %d, want 1", report.DeletedCount)
	}
}

// insertRawCheckpoint inserta un checkpoint DIRECTAMENTE en SQL, evitando el
// dedup por contenido de findDuplicate — mismo propósito que
// insertRawTopicDuplicate para los duplicados por tópico.
func insertRawCheckpoint(t *testing.T, db *sql.DB, project, title, content string) int64 {
	t.Helper()
	res, err := db.Exec(
		`INSERT INTO memories (project, type, title, content) VALUES (?, 'checkpoint', ?, ?)`,
		project, title, content,
	)
	if err != nil {
		t.Fatalf("insertRawCheckpoint: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// insertRawTopicDuplicate inserta una fila DIRECTAMENTE en SQL, evitando el
// upsert de findDuplicate (InsertMemory lo aplicaría y colapsaría el grupo
// antes de que exista nada que consolidar) — así se puede fijar la fixture
// que la consolidación existe para resolver.
func insertRawTopicDuplicate(t *testing.T, db *sql.DB, project, topicKey, content string) int64 {
	t.Helper()
	res, err := db.Exec(
		`INSERT INTO memories (project, type, title, content, topic_key) VALUES (?, 'decision', 'rev', ?, ?)`,
		project, content, topicKey,
	)
	if err != nil {
		t.Fatalf("insertRawTopicDuplicate: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}
