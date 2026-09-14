package usecases_test

import (
	"bytes"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// Round-trip completo: export → JSON → import en un proyecto vacío, verificando
// preservación de timestamps, remapeo de relaciones, ausencia de sinapsis
// espurias y dedup en la reimportación.
func TestPortability_RoundTrip(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = db.Close() }()

	memRepo := persistence.NewMemoryRepository(db)
	relRepo := persistence.NewRelationRepository(db)

	// Proyecto "src": 2 memorias con timestamps conocidos + 1 relación explícita.
	// Se usa ImportMemory para controlar timestamps y NO generar sinapsis.
	id1, err := memRepo.ImportMemory(&domain.Memory{Project: "src", Type: domain.Decision, Title: "A", Content: "cuerpo A", CreatedAt: "2020-01-02 03:04:05", UpdatedAt: "2020-01-02 03:04:05"})
	if err != nil {
		t.Fatalf("import src A: %v", err)
	}
	id2, err := memRepo.ImportMemory(&domain.Memory{Project: "src", Type: domain.Bugfix, Title: "B", Content: "cuerpo B", CreatedAt: "2021-06-07 08:09:10", UpdatedAt: "2021-06-07 08:09:10"})
	if err != nil {
		t.Fatalf("import src B: %v", err)
	}
	if _, err := relRepo.ImportRelation(&domain.Relation{Project: "src", MemoryIDA: id1, MemoryIDB: id2, Relation: domain.Supersedes, Confidence: 0.9, Reasoning: "la nueva reemplaza", CreatedAt: "2021-06-07 08:09:11"}); err != nil {
		t.Fatalf("import src rel: %v", err)
	}

	bundle, err := usecases.ExportProject(memRepo, relRepo, "src")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(bundle.Memories) != 2 || len(bundle.Relations) != 1 {
		t.Fatalf("bundle inesperado: mem=%d rel=%d", len(bundle.Memories), len(bundle.Relations))
	}

	// Round-trip por JSON.
	var buf bytes.Buffer
	if err := usecases.EncodeBundle(&buf, bundle); err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := usecases.DecodeBundle(&buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Import en "dst" (vacío).
	rep, err := usecases.ImportBundle(memRepo, relRepo, "dst", decoded)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if rep.MemoriesImported != 2 || rep.RelationsImported != 1 {
		t.Fatalf("reporte import inesperado: %+v", rep)
	}

	dstMems, _ := memRepo.ListAll("dst")
	if len(dstMems) != 2 {
		t.Fatalf("dst memorias=%d", len(dstMems))
	}
	var da, dbID int64
	var createdA string
	for _, m := range dstMems {
		switch m.Title {
		case "A":
			da, createdA = m.ID, m.CreatedAt
		case "B":
			dbID = m.ID
		}
	}
	// Timestamp preservado (no sobrescrito por now()).
	if !strings.HasPrefix(createdA, "2020-01-02") {
		t.Fatalf("timestamp no preservado en import: %q", createdA)
	}

	// Relación remapeada a los ids de dst y sin sinapsis espurias.
	dstRels, _ := relRepo.ListAll("dst")
	if len(dstRels) != 1 {
		t.Fatalf("dst relaciones=%d (esperaba 1: sin auto-sinapsis)", len(dstRels))
	}
	r := dstRels[0]
	if r.MemoryIDA != da || r.MemoryIDB != dbID {
		t.Fatalf("relación no remapeada a ids de dst: %+v (da=%d db=%d)", r, da, dbID)
	}
	if r.Relation != domain.Supersedes {
		t.Fatalf("tipo de relación perdido: %s", r.Relation)
	}

	// Dedup: reimportar el mismo bundle no duplica nada.
	rep2, err := usecases.ImportBundle(memRepo, relRepo, "dst", decoded)
	if err != nil {
		t.Fatalf("reimport: %v", err)
	}
	if rep2.MemoriesImported != 0 || rep2.RelationsImported != 0 {
		t.Fatalf("dedup falló (importó de nuevo): %+v", rep2)
	}
	if rep2.MemoriesSkipped != 2 || rep2.RelationsSkipped != 1 {
		t.Fatalf("dedup: skips inesperados: %+v", rep2)
	}
}

// acr_5836d32d, C-001: el roundtrip export → JSON → import debe conservar la
// identidad de dedup (topic_key) y el linaje de revisión (source_review_id).
func TestPortability_RoundTripPreservaTopicKeyYLinaje(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = db.Close() }()

	memRepo := persistence.NewMemoryRepository(db)
	relRepo := persistence.NewRelationRepository(db)

	if _, err := memRepo.ImportMemory(&domain.Memory{Project: "src", Type: domain.Decision, Title: "Reglas",
		Content: "documento fijado", TopicKey: "tk-reglas", SourceReviewID: "acr_origen"}); err != nil {
		t.Fatalf("import src: %v", err)
	}

	bundle, err := usecases.ExportProject(memRepo, relRepo, "src")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	var buf bytes.Buffer
	if err := usecases.EncodeBundle(&buf, bundle); err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := usecases.DecodeBundle(&buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, err := usecases.ImportBundle(memRepo, relRepo, "dst", decoded); err != nil {
		t.Fatalf("import: %v", err)
	}

	dst, err := memRepo.ListAll("dst")
	if err != nil || len(dst) != 1 {
		t.Fatalf("dst: %v %v", dst, err)
	}
	if dst[0].TopicKey != "tk-reglas" || dst[0].SourceReviewID != "acr_origen" {
		t.Fatalf("la memoria reimportada perdió identidad o linaje: topic_key=%q source_review_id=%q", dst[0].TopicKey, dst[0].SourceReviewID)
	}
}

// Un bundle v1 (sin topic_key ni source_review_id) sigue importándose; uno de
// una versión posterior se rechaza en vez de perder campos en silencio.
func TestDecodeBundle_MigraV1YRechazaFuturo(t *testing.T) {
	v1 := `{"version":1,"exported_at":"2026-01-01T00:00:00Z","source_project":"src",
		"memories":[{"ref_id":1,"type":"decision","title":"A","content":"cuerpo A","created_at":"2020-01-02 03:04:05","updated_at":"2020-01-02 03:04:05"}],
		"relations":[]}`
	b, err := usecases.DecodeBundle(strings.NewReader(v1))
	if err != nil {
		t.Fatalf("decode v1: %v", err)
	}
	if b.Version != domain.ExportVersion || len(b.Memories) != 1 || b.Memories[0].TopicKey != "" {
		t.Fatalf("migración v1 inesperada: %+v", b)
	}

	futuro := `{"version":99,"memories":[],"relations":[]}`
	if _, err := usecases.DecodeBundle(strings.NewReader(futuro)); err == nil {
		t.Fatal("un bundle de versión posterior debe rechazarse")
	}
}
