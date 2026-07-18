package persistence

import (
	"database/sql"
	"strings"
	"testing"

	"mem/domain"
)

// TestSearchMemories_RanksByRelevanceWhenFTSAvailable cubre la Historia de
// Usuario 3 (specs/009-mitigacion-riesgos): el resultado más relevante para el
// término buscado debe aparecer primero, incluso si es MÁS ANTIGUO que uno de
// menor densidad. Ambos títulos son neutros (sin el término) para que el
// ranking por baldes título/contenido de hoy no pueda "acertar por
// coincidencia" — solo un ranking real (BM25) distingue este caso; bajo el
// mecanismo actual (empate de balde + orden por recencia) el más reciente
// ganaría aunque sea menos relevante, así que este test solo pasa con FTS5.
func TestSearchMemories_RanksByRelevanceWhenFTSAvailable(t *testing.T) {
	db := openTestDB(t)

	// Alta densidad, insertada PRIMERO (más antigua). Títulos DISTINTOS entre sí
	// (para no disparar el dedup por identidad de InsertMemory) pero ninguno
	// contiene el término buscado (para no favorecer el balde de título).
	highID, err := InsertMemory(db, &domain.Memory{
		Project: "proj", Type: domain.Decision, Title: "memoria alpha",
		Content: "gomemory gomemory gomemory: todo este contenido trata sobre gomemory",
	})
	if err != nil {
		t.Fatalf("insert high: %v", err)
	}
	// Se fuerza que highID quede claramente MÁS ANTIGUA (no solo "primero
	// insertada": los timestamps tienen granularidad de segundo y ambos
	// inserts corren en el mismo segundo, así que sin esto el desempate por
	// recencia del mecanismo actual sería un empate accidental, no una prueba
	// real). Con esto, el mecanismo actual (empate de balde + ORDER BY
	// created_at DESC) SIEMPRE preferiría a lowID — solo un ranking real
	// (BM25) puede hacer ganar a highID pese a ser más vieja.
	if _, err := db.Exec(`UPDATE memories SET created_at = datetime('now', '-5 hours', '-1 hour') WHERE id = ?`, highID); err != nil {
		t.Fatalf("backdate high: %v", err)
	}
	// Baja densidad, insertada DESPUÉS (más reciente) — pero menos relevante.
	lowID, err := InsertMemory(db, &domain.Memory{
		Project: "proj", Type: domain.Decision, Title: "memoria beta",
		Content: "esta nota trata de varias cosas y menciona gomemory una sola vez de pasada",
	})
	if err != nil {
		t.Fatalf("insert low: %v", err)
	}

	mems, err := SearchMemories(db, "proj", "gomemory", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(mems) != 2 {
		t.Fatalf("esperaba 2 resultados, got %d: %+v", len(mems), mems)
	}
	if mems[0].ID != highID {
		t.Fatalf("esperaba la memoria de mayor densidad (%d) primero, got orden %d,%d", highID, mems[0].ID, mems[1].ID)
	}
	if mems[1].ID != lowID {
		t.Fatalf("esperaba la memoria de menor densidad (%d) segunda, got %d", lowID, mems[1].ID)
	}
}

// TestSearchMemories_FallsBackToLikeWhenFTSTableMissing simula un build sin
// soporte FTS5 (o cualquier corrupción de la tabla auxiliar) borrando
// memory_search directamente, y confirma que la búsqueda sigue devolviendo
// resultados correctos vía el mecanismo LIKE existente, sin error visible.
func TestSearchMemories_FallsBackToLikeWhenFTSTableMissing(t *testing.T) {
	db := openTestDB(t)

	id, err := InsertMemory(db, &domain.Memory{
		Project: "proj", Type: domain.Learning, Title: "fallback", Content: "contenido de prueba sobre gomemory",
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	if _, err := db.Exec(`DROP TABLE IF EXISTS memory_search`); err != nil {
		t.Fatalf("drop memory_search: %v", err)
	}

	mems, err := SearchMemories(db, "proj", "gomemory", 10)
	if err != nil {
		t.Fatalf("search sin memory_search no debía fallar: %v", err)
	}
	if len(mems) != 1 || mems[0].ID != id {
		t.Fatalf("esperaba 1 resultado (fallback LIKE), got %+v", mems)
	}
}

// TestMemorySearchIndex_StaysInSyncWithMemories cubre la Historia de Usuario 3:
// insertar, actualizar (vía dedup por topic_key) y borrar una memoria debe
// mantener memory_search 1:1 con memories, sin filas huérfanas ni faltantes.
func TestMemorySearchIndex_StaysInSyncWithMemories(t *testing.T) {
	db := openTestDB(t)

	id, err := InsertMemory(db, &domain.Memory{
		Project: "proj", Type: domain.Decision, Title: "v1", Content: "contenido inicial", TopicKey: "tema-x",
	})
	if err != nil {
		t.Fatalf("insert v1: %v", err)
	}
	assertMemorySearchRow(t, db, id, "contenido inicial")

	// Misma topic_key: findDuplicate consolida (UPDATE), no crea una fila nueva.
	id2, err := InsertMemory(db, &domain.Memory{
		Project: "proj", Type: domain.Decision, Title: "v2", Content: "contenido actualizado", TopicKey: "tema-x",
	})
	if err != nil {
		t.Fatalf("insert v2 (dedup): %v", err)
	}
	if id2 != id {
		t.Fatalf("esperaba consolidación en el mismo id %d, got %d", id, id2)
	}
	assertMemorySearchRow(t, db, id, "contenido actualizado")
	assertMemorySearchRowCount(t, db, id, 1) // no debe duplicarse la fila de índice

	if _, err := DeleteMemory(db, "proj", id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	assertMemorySearchRowCount(t, db, id, 0) // no debe quedar huérfana tras borrar
}

func assertMemorySearchRow(t *testing.T, db *sql.DB, memoryID int64, wantContentSubstr string) {
	t.Helper()
	var content string
	err := db.QueryRow(`SELECT content FROM memory_search WHERE memory_id = ?`, memoryID).Scan(&content)
	if err != nil {
		t.Fatalf("memory_search sin fila para memory_id=%d: %v", memoryID, err)
	}
	if !strings.Contains(content, wantContentSubstr) {
		t.Fatalf("memory_search desincronizada: content=%q, esperaba contener %q", content, wantContentSubstr)
	}
}

func assertMemorySearchRowCount(t *testing.T, db *sql.DB, memoryID int64, want int) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM memory_search WHERE memory_id = ?`, memoryID).Scan(&count); err != nil {
		t.Fatalf("contar filas de memory_search: %v", err)
	}
	if count != want {
		t.Fatalf("esperaba %d filas en memory_search para memory_id=%d, got %d", want, memoryID, count)
	}
}
