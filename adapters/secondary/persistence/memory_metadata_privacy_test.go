package persistence

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"mem/application/ports"
	"mem/domain"
)

// HCD-044: filepath y topic_key entran en el contrato de privacidad.

const (
	pathCanary  = "SYNTH-PATH-0044"
	topicCanary = "SYNTH-TOPIC-0044"
)

// metadataLeaks dice si alguna fila de memories conserva un canario en sus
// metadatos.
func metadataLeaks(t *testing.T, db *sql.DB) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories WHERE COALESCE(filepath,'') || COALESCE(topic_key,'') LIKE '%SYNTH-%'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

func TestMetadatosPrivados_SeRechazanAlGuardar(t *testing.T) {
	db := openTestDB(t)
	cases := map[string]domain.Memory{
		"filepath":  {Filepath: "src/<private>" + pathCanary + "</private>a.go"},
		"topic_key": {TopicKey: "tema/<private>" + topicCanary + "</private>"},
	}
	for name, meta := range cases {
		t.Run(name, func(t *testing.T) {
			m := domain.Memory{Project: "proj", Type: domain.Decision, Title: "público", Content: "contenido público", Filepath: meta.Filepath, TopicKey: meta.TopicKey}
			if _, err := InsertMemory(db, &m); !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("InsertMemory = %v, quiero ErrValidation", err)
			}
			if err := func() error { _, err := InsertSeedMemory(db, &m); return err }(); !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("InsertSeedMemory = %v, quiero ErrValidation", err)
			}
		})
	}

	// Deduplicación por clave pública: un filepath privado no pisa la memoria.
	base := domain.Memory{Project: "proj", Type: domain.Decision, Title: "tema", Content: "v1", Filepath: "src/a.go", TopicKey: "tema-publico"}
	id, err := InsertMemory(db, &base)
	if err != nil {
		t.Fatal(err)
	}
	dup := base
	dup.Content, dup.Filepath = "v2", "src/<private>"+pathCanary+"</private>"
	if _, err := InsertMemory(db, &dup); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("dedup con filepath privado = %v, quiero ErrValidation", err)
	}
	got, err := GetMemoryByID(db, "proj", id)
	if err != nil || got == nil || got.Content != "v1" || got.Filepath != "src/a.go" {
		t.Fatalf("la memoria existente cambió: %+v %v", got, err)
	}

	// El upsert con clave pública sigue estable.
	again := base
	again.Content = "v3"
	id2, err := InsertMemory(db, &again)
	if err != nil || id2 != id {
		t.Fatalf("upsert público = %d %v, quiero %d", id2, err, id)
	}
	if metadataLeaks(t, db) {
		t.Fatal("la base conserva un canario en los metadatos")
	}
}

func TestImportMemory_RechazaMetadatosPrivados(t *testing.T) {
	db := openTestDB(t)
	for _, m := range []domain.Memory{
		{Project: "proj", Type: domain.Decision, Title: "a", Content: "c", Filepath: "<private>" + pathCanary + "</private>"},
		{Project: "proj", Type: domain.Decision, Title: "b", Content: "c", TopicKey: "<private>" + topicCanary + "</private>"},
	} {
		if _, err := ImportMemory(db, &m); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("ImportMemory = %v, quiero ErrValidation", err)
		}
	}
	if metadataLeaks(t, db) {
		t.Fatal("la importación guardó un canario en los metadatos")
	}
}

// insertHistoricalMetadata simula una fila guardada antes de HCD-044.
func insertHistoricalMetadata(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO memories (id, project, type, title, content, filepath, topic_key) VALUES (20, 'proj', 'decision', 'título público', 'contenido público', ?, ?)`,
		"src/<private>"+pathCanary+"</private>a.go", "tema/<private>"+topicCanary+"</private>"); err != nil {
		t.Fatal(err)
	}
}

func TestMetadatosPrivados_HistoricosSeProtegenAlLeer(t *testing.T) {
	db := openTestDB(t)
	insertHistoricalMetadata(t, db)
	byID, err := GetMemoryByID(db, "proj", 20)
	if err != nil || byID == nil {
		t.Fatalf("GetMemoryByID = %+v %v", byID, err)
	}
	all, err := ListAllMemories(db, "proj")
	if err != nil || len(all) != 1 {
		t.Fatalf("ListAllMemories = %d %v", len(all), err)
	}
	list, err := ListMemories(db, "proj", 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListMemories = %d %v", len(list), err)
	}
	for _, m := range []domain.Memory{*byID, all[0], list[0]} {
		if strings.Contains(m.Filepath+m.TopicKey, "SYNTH-") {
			t.Fatalf("la lectura devuelve un canario: filepath=%q topic_key=%q", m.Filepath, m.TopicKey)
		}
		if m.Filepath != "src/a.go" {
			t.Fatalf("la lectura perdió la parte pública de la ruta: %q", m.Filepath)
		}
	}
}

func TestPrivacyStore_SaneaMetadatosDeMemorias(t *testing.T) {
	db := openTestDB(t)
	insertHistoricalMetadata(t, db)
	if _, err := db.Exec(`INSERT INTO memories (id, project, type, title, content, filepath, topic_key) VALUES (21, 'proj', 'decision', 'limpia', 'contenido', 'src/b.go', 'tema-limpio')`); err != nil {
		t.Fatal(err)
	}
	store, ctx := NewPrivacyMaintenanceStore(db), context.Background()

	cands, err := store.Candidates(ctx, "proj")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"memory_filepath:20", "memory_topic_key:20"}
	if got := candidateIDs(cands); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("candidatos = %v, quiero %v", got, want)
	}
	for _, c := range cands {
		if err := c.Validate("proj"); err != nil || !c.Writable() {
			t.Fatalf("candidato no aplicable %+v: %v", c, err)
		}
	}

	err = store.WithTransaction(ctx, "proj", func(tx ports.PrivacyTransaction) error {
		cs, err := tx.Candidates(ctx, "proj")
		if err != nil {
			return err
		}
		for _, c := range cs {
			if changed, err := tx.Apply(ctx, c, domain.ProtectText(c.Text)); err != nil || !changed {
				return errors.Join(err, errors.New("Apply no cambió "+string(c.Category)))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	var title, content, path string
	var topic sql.NullString
	if err := db.QueryRow(`SELECT title, content, filepath, topic_key FROM memories WHERE id = 20`).Scan(&title, &content, &path, &topic); err != nil {
		t.Fatal(err)
	}
	if path != "src/a.go" || topic.Valid || title != "título público" || content != "contenido público" {
		t.Fatalf("fila saneada = filepath=%q topic_key=%v título=%q contenido=%q", path, topic, title, content)
	}
	if metadataLeaks(t, db) {
		t.Fatal("el saneamiento dejó un canario en los metadatos")
	}
	if left, err := store.Candidates(ctx, "proj"); err != nil || len(left) != 0 {
		t.Fatalf("quedan candidatos tras sanear: %v %v", candidateIDs(left), err)
	}
	var cleanTopic string
	if err := db.QueryRow(`SELECT topic_key FROM memories WHERE id = 21`).Scan(&cleanTopic); err != nil || cleanTopic != "tema-limpio" {
		t.Fatalf("el saneamiento tocó una fila limpia: %q %v", cleanTopic, err)
	}
}
