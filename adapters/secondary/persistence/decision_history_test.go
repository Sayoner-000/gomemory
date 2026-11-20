package persistence

import (
	"database/sql"
	"strings"
	"testing"

	"mem/domain"
)

// HCD-046: las fichas del historial se enlazan con los archivos que citan.

func linkedFiles(t *testing.T, db *sql.DB, id int64) string {
	t.Helper()
	rows, err := db.Query(`SELECT path FROM memory_files WHERE memory_id = ? ORDER BY path`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return strings.Join(out, ",")
}

func digestMemory(project, content string) *domain.Memory {
	return &domain.Memory{Project: project, Type: domain.Decision, Title: "HCD-044 · Metadatos (activa)", Content: content, Filepath: "hcd/HCD-044-metadatos.md", TopicKey: "hcd/HCD-044"}
}

func TestDecisionHistory_UpsertReemplazaLosEnlaces(t *testing.T) {
	db := openTestDB(t)
	repo := NewDecisionHistoryRepository(db)
	id, err := repo.UpsertDigest(digestMemory("proj", "v1"), []string{"a.go", "b.go", "a.go", ""}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := linkedFiles(t, db, id); got != "a.go,b.go" {
		t.Fatalf("enlaces = %q", got)
	}
	id2, err := repo.UpsertDigest(digestMemory("proj", "v2"), []string{"b.go", "c.go"}, true)
	if err != nil || id2 != id {
		t.Fatalf("upsert = %d %v, quiero la misma memoria %d", id2, err, id)
	}
	if got := linkedFiles(t, db, id); got != "b.go,c.go" {
		t.Fatalf("enlaces tras el upsert = %q", got)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories WHERE topic_key = 'hcd/HCD-044'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("fichas = %d %v", n, err)
	}
}

func TestDecisionHistory_EnlaceNoGuardaRutasPrivadas(t *testing.T) {
	db := openTestDB(t)
	id, err := NewDecisionHistoryRepository(db).UpsertDigest(digestMemory("proj", "v1"), []string{"a.go", "<private>SYNTH-LINK-0046</private>.go"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := linkedFiles(t, db, id); got != "a.go" {
		t.Fatalf("enlaces = %q", got)
	}
}

func TestDecisionHistory_BorrarLaMemoriaLimpiaSusEnlaces(t *testing.T) {
	db := openTestDB(t)
	id, err := NewDecisionHistoryRepository(db).UpsertDigest(digestMemory("proj", "v1"), []string{"a.go"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := DeleteMemory(db, "proj", id); err != nil || !ok {
		t.Fatalf("DeleteMemory = %v %v", ok, err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_files`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("enlaces huérfanos = %d %v", n, err)
	}
}

func TestListMemoriesByFilepath_IncluyeEnlaces(t *testing.T) {
	db := openTestDB(t)
	repo := NewDecisionHistoryRepository(db)
	if _, err := repo.UpsertDigest(digestMemory("proj", "v1"), []string{"adapters/x.go"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertDigest(digestMemory("otro", "v1"), []string{"adapters/x.go"}, false); err != nil {
		t.Fatal(err)
	}
	direct := domain.Memory{Project: "proj", Type: domain.Bugfix, Title: "anclada", Content: "c", Filepath: "adapters/x.go"}
	if _, err := InsertMemory(db, &direct); err != nil {
		t.Fatal(err)
	}
	mems, err := ListMemoriesByFilepath(db, "proj", []string{"adapters/x.go"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, m := range mems {
		titles = append(titles, m.Title)
	}
	if strings.Join(titles, " | ") != "anclada | HCD-044 · Metadatos (activa)" {
		t.Fatalf("memorias del archivo = %v", titles)
	}
}

// HCD-058: renombrar una unidad reapunta las memorias y los enlaces que
// citaban la ruta vieja, solo en el proyecto indicado.
func TestRetargetPath(t *testing.T) {
	db := openTestDB(t)
	repo := NewDecisionHistoryRepository(db)
	old, nuevo := "hcd/HCD-044-metadatos.md", "hcd/HCD-044-nuevo.md"
	id, err := repo.UpsertDigest(digestMemory("proj", "v1"), []string{old, "a.go"}, false)
	if err != nil {
		t.Fatal(err)
	}
	other := digestMemory("otro", "v1")
	otherID, err := repo.UpsertDigest(other, []string{old}, false)
	if err != nil {
		t.Fatal(err)
	}
	// Una memoria que ya enlaza ambas rutas no debe duplicar la clave.
	if _, err := db.Exec(`INSERT INTO memory_files (project, memory_id, path) VALUES ('proj', ?, ?)`, id, nuevo); err != nil {
		t.Fatal(err)
	}
	if err := repo.RetargetPath("proj", old, nuevo); err != nil {
		t.Fatal(err)
	}
	if got := linkedFiles(t, db, id); got != "a.go,"+nuevo {
		t.Fatalf("enlaces = %q", got)
	}
	var fp string
	if err := db.QueryRow(`SELECT filepath FROM memories WHERE id = ?`, id).Scan(&fp); err != nil || fp != nuevo {
		t.Fatalf("filepath = %q %v", fp, err)
	}
	if got := linkedFiles(t, db, otherID); got != old {
		t.Fatalf("otro proyecto tocado: %q", got)
	}
}
