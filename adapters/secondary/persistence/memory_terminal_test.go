package persistence

import (
	"strings"
	"testing"

	"mem/domain"
)

// TestMemoryRead_NeutralizaSecuenciasDeTerminal cierra H-01 (auditoría
// 2026-10-17, ronda 2): una memoria con secuencias de control, guardada antes
// de la guarda o por cualquier vía, se lee sin ellas en todas las consultas.
func TestMemoryRead_NeutralizaSecuenciasDeTerminal(t *testing.T) {
	db := openTestDB(t)
	// Fila escrita directamente, como la dejaría una versión anterior.
	res, err := db.Exec(`INSERT INTO memories (project, type, title, content, filepath, created_at, updated_at)
		VALUES ('proj', 'learning', ?, ?, ?, '2026-10-17 10:00:00', '2026-10-17 10:00:00')`,
		"titulo \x1b]0;PWNED\x07", "copia \x1b]52;c;aGFjaw==\x07 y \x1b[2J borrado", "a\x1b[31m.go")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	clean := func(m domain.Memory) {
		t.Helper()
		for _, field := range []string{m.Title, m.Content, m.Filepath} {
			if strings.ContainsAny(field, "\x1b\x07") {
				t.Fatalf("secuencia de control en la lectura: %q", field)
			}
		}
		if !strings.Contains(m.Title, "]0;PWNED") {
			t.Fatalf("el texto visible no se conserva: %q", m.Title)
		}
	}
	got, err := GetMemoryByID(db, "proj", id)
	if err != nil || got == nil {
		t.Fatal(got, err)
	}
	clean(*got)
	list, err := ListMemories(db, "proj", 10)
	if err != nil || len(list) != 1 {
		t.Fatal(list, err)
	}
	clean(list[0])
}
