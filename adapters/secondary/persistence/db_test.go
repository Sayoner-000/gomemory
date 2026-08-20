package persistence

import (
	"testing"

	"mem/domain"
)

// TestMigrate_IsIdempotent cubre SC-011 de la feature 020: correr la
// migración dos veces sobre una base existente no debe fallar ni alterar las
// tablas previas. Open() ya llama a migrate() en cada apertura, así que abrir
// dos veces el mismo directorio ES correr la migración dos veces.
func TestMigrate_IsIdempotent(t *testing.T) {
	dir := t.TempDir()

	db1, err := Open(dir)
	if err != nil {
		t.Fatalf("primera apertura: %v", err)
	}
	if _, err := InsertMemory(db1, &domain.Memory{
		Project: "proj", Type: domain.Learning, Title: "t", Content: "c",
	}); err != nil {
		t.Fatalf("insertar memoria previa: %v", err)
	}
	db1.Close()

	// Segunda apertura sobre el MISMO directorio: vuelve a correr migrate().
	db2, err := Open(dir)
	if err != nil {
		t.Fatalf("segunda apertura (migración repetida) no debe fallar: %v", err)
	}
	defer db2.Close()

	var count int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM memories WHERE project = 'proj'`).Scan(&count); err != nil {
		t.Fatalf("consultar memories tras segunda migración: %v", err)
	}
	if count != 1 {
		t.Fatalf("la segunda migración alteró los datos previos: count=%d, want 1", count)
	}

	// usage_records debe existir y estar operativa tras la migración repetida.
	if _, err := db2.Exec(
		`INSERT INTO usage_records (project, session_id, operation, channel, baseline_tokens, emitted_tokens)
		 VALUES ('proj', 'sess', 'build_context', 'cli', 10, 5)`,
	); err != nil {
		t.Fatalf("usage_records no operativa tras migración repetida: %v", err)
	}
}
