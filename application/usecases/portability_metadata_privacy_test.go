package usecases_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// HCD-044: un bundle con metadatos privados no se importa a medias.
func TestImportBundle_RechazaMetadatosPrivados(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	memRepo, relRepo := persistence.NewMemoryRepository(db), persistence.NewRelationRepository(db)

	in := domain.ExportBundle{Version: domain.ExportVersion, Memories: []domain.ExportMemory{
		{RefID: 1, Type: "decision", Title: "limpia", Content: "cuerpo"},
		{RefID: 2, Type: "decision", Title: "ruta", Content: "cuerpo", Filepath: "<private>SYNTH-PATH-0044</private>"},
		{RefID: 3, Type: "decision", Title: "tema", Content: "cuerpo", TopicKey: "<private>SYNTH-TOPIC-0044</private>"},
	}}
	rep, err := usecases.ImportBundle(memRepo, relRepo, "dst", in)
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("ImportBundle = %+v %v, quiero ErrValidation", rep, err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories WHERE project = 'dst'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("import parcial: %d memorias %v", n, err)
	}
}

// HCD-044: una fila histórica no reemite sus metadatos privados al exportar.
func TestExportProject_ProtegeMetadatosHistoricos(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`INSERT INTO memories (id, project, type, title, content, filepath, topic_key) VALUES (1, 'src', 'decision', 'A', 'cuerpo', 'src/<private>SYNTH-PATH-0044</private>a.go', '<private>SYNTH-TOPIC-0044</private>')`); err != nil {
		t.Fatal(err)
	}
	bundle, err := usecases.ExportProject(persistence.NewMemoryRepository(db), persistence.NewRelationRepository(db), "src")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := usecases.EncodeBundle(&buf, bundle); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "SYNTH-") {
		t.Fatalf("export reemite metadatos privados:\n%s", buf.String())
	}
}

type mockMemRepoForExport struct {
	ports.MemoryRepository
	mems []domain.Memory
}

func (m *mockMemRepoForExport) ListAll(project string) ([]domain.Memory, error) {
	return m.mems, nil
}

type mockRelRepoForExport struct {
	ports.RelationRepository
}

func (r *mockRelRepoForExport) ListAll(project string) ([]domain.Relation, error) {
	return nil, nil
}

func TestExportProject_ProtegeMetadatosDefensaEnProfundidad(t *testing.T) {
	memRepo := &mockMemRepoForExport{
		mems: []domain.Memory{
			{
				ID:       1,
				Project:  "src",
				Type:     domain.Decision,
				Title:    "limpio",
				Content:  "cuerpo",
				Filepath: "src/<private>SYNTH-RAW-PATH</private>/foo.go",
				TopicKey: "<private>SYNTH-RAW-TOPIC</private>",
			},
		},
	}
	bundle, err := usecases.ExportProject(memRepo, &mockRelRepoForExport{}, "src")
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Memories) != 1 {
		t.Fatalf("se esperaba 1 memoria, got %d", len(bundle.Memories))
	}
	if strings.Contains(bundle.Memories[0].Filepath, "SYNTH-RAW-PATH") {
		t.Errorf("Filepath conservó texto privado: %s", bundle.Memories[0].Filepath)
	}
	if strings.Contains(bundle.Memories[0].TopicKey, "SYNTH-RAW-TOPIC") {
		t.Errorf("TopicKey conservó texto privado: %s", bundle.Memories[0].TopicKey)
	}
}
