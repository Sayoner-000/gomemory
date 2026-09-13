package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

func TestParseMassFlags(t *testing.T) {
	task, top, err := ParseMassFlags(nil)
	if err != nil || task != "" || top != 15 {
		t.Fatalf("defaults = (%q, %d, %v), want (\"\", 15, nil)", task, top, err)
	}
	if _, _, err := ParseMassFlags([]string{"--top", "0"}); err == nil {
		t.Fatal("--top 0 debe fallar")
	}
	task, top, err = ParseMassFlags([]string{"--task", "sinapsis cache", "--top", "3"})
	if err != nil || task != "sinapsis cache" || top != 3 {
		t.Fatalf("= (%q, %d, %v)", task, top, err)
	}
}

func newMassDeps(t *testing.T) *Deps {
	t.Helper()
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &Deps{
		Project:      "proj",
		MemoryRepo:   persistence.NewMemoryRepository(db),
		RelationRepo: persistence.NewRelationRepository(db),
		SessionRepo:  persistence.NewSessionRepository(db),
	}
}

func insertMass(t *testing.T, deps *Deps, m domain.Memory) int64 {
	t.Helper()
	m.Project = deps.Project
	id, err := deps.MemoryRepo.Insert(&m)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	return id
}

func relateMass(t *testing.T, deps *Deps, a, b int64) {
	t.Helper()
	if _, err := deps.RelationRepo.Insert(&domain.Relation{Project: deps.Project, MemoryIDA: a, MemoryIDB: b, Relation: domain.Related, Confidence: 1}); err != nil {
		t.Fatalf("relate: %v", err)
	}
}

func TestRunMass_ContratoDeSalida(t *testing.T) {
	deps := newMassDeps(t)
	a := insertMass(t, deps, domain.Memory{Type: domain.Decision, Title: "caché de sinapsis", Content: "sinapsis cache redis"})
	b := insertMass(t, deps, domain.Memory{Type: domain.Learning, Title: "otra memoria", Content: "algo distinto"})
	cp := insertMass(t, deps, domain.Memory{Type: domain.Checkpoint, Title: "Checkpoint automático", Content: "Editó: x.go"})
	relateMass(t, deps, a, b)
	relateMass(t, deps, a, cp)

	var first, second bytes.Buffer
	if err := runMass(deps, "", 15, &first); err != nil {
		t.Fatalf("runMass: %v", err)
	}
	if err := runMass(deps, "", 15, &second); err != nil {
		t.Fatalf("runMass: %v", err)
	}
	out := first.String()
	if out != second.String() {
		t.Fatal("dos ejecuciones difieren")
	}
	if !strings.HasPrefix(out, "Masa de memorias — semillas: todas las memorias (uniforme)\n\n") {
		t.Fatalf("cabecera inesperada:\n%s", out)
	}
	for _, want := range []string{"  1. [", "(decision) caché de sinapsis — masa 0.", "(learning) otra memoria — masa 0."} {
		if !strings.Contains(out, want) {
			t.Fatalf("falta %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, fmt.Sprintf("[%d]", cp)) {
		t.Fatalf("el ranking no debe listar checkpoints:\n%s", out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	last := lines[len(lines)-1]
	if last != "masa = centralidad en el grafo de memorias sembrado en todas las memorias (uniforme); no mide importancia ni corrección" {
		t.Fatalf("última línea = %q", last)
	}
}

func TestRunMass_TareaSinResultados(t *testing.T) {
	deps := newMassDeps(t)
	insertMass(t, deps, domain.Memory{Type: domain.Decision, Title: "algo", Content: "contenido"})
	var w bytes.Buffer
	if err := runMass(deps, "zzzqqq", 15, &w); err != nil {
		t.Fatalf("runMass: %v", err)
	}
	if !strings.Contains(w.String(), `Sin memorias que coincidan con "zzzqqq": masa no calculada`) {
		t.Fatalf("salida = %q", w.String())
	}
}

func TestRunMass_SemillasDeTareaYDeSesion(t *testing.T) {
	deps := newMassDeps(t)
	insertMass(t, deps, domain.Memory{Type: domain.Decision, Title: "redis como caché", Content: "usamos redis"})
	var task bytes.Buffer
	if err := runMass(deps, "redis", 15, &task); err != nil {
		t.Fatalf("runMass: %v", err)
	}
	if !strings.HasPrefix(task.String(), `Masa de memorias — semillas: tarea "redis" (1)`) {
		t.Fatalf("cabecera con tarea = %q", task.String())
	}

	sess, err := deps.SessionRepo.Start(deps.Project)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	insertMass(t, deps, domain.Memory{SessionID: sess.ID, Type: domain.Learning, Title: "de la sesión", Content: "s"})
	var session bytes.Buffer
	if err := runMass(deps, "", 15, &session); err != nil {
		t.Fatalf("runMass: %v", err)
	}
	if !strings.HasPrefix(session.String(), "Masa de memorias — semillas: sesión activa (1)") {
		t.Fatalf("cabecera con sesión = %q", session.String())
	}
}
