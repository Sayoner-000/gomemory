package usecases_test

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// TestHCD002OmittedRecovery comprueba que todo lo que el extracto de 80
// caracteres deja fuera se recupera íntegro: cada puntero get_memory del
// contexto apunta a una memoria existente del proyecto cuyo contenido coincide
// por huella con el original guardado (C-02, Q2).
func TestHCD002OmittedRecovery(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	memRepo := persistence.NewMemoryRepository(db)
	originals := map[string][32]byte{}
	for i, kind := range []domain.MemoryType{domain.Decision, domain.Bugfix, domain.Learning, domain.Pattern, domain.Discovery, domain.Architecture} {
		title := fmt.Sprintf("entrada %d", i)
		content := fmt.Sprintf("original %d ñandú 漢字 ", i) + strings.Repeat(fmt.Sprintf("detalle %d que el extracto omite ", i), 25)
		if _, err := memRepo.Insert(&domain.Memory{Project: "proj", Type: kind, Title: title, Content: content}); err != nil {
			t.Fatal(err)
		}
		originals[title] = sha256.Sum256([]byte(content))
	}
	builder := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), "", "proj")
	builder.Budget = 24000
	out, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}

	pointer := regexp.MustCompile("(?m)^- \\*\\*(entrada \\d+)\\*\\*: .* → `get_memory (\\d+)`$")
	matches := pointer.FindAllStringSubmatch(out, -1)
	if len(matches) != len(originals) {
		t.Fatalf("se esperaban %d entradas recortadas con puntero, hay %d:\n%s", len(originals), len(matches), out)
	}
	for _, m := range matches {
		id, _ := strconv.ParseInt(m[2], 10, 64)
		got, err := memRepo.Get("proj", id)
		if err != nil || got == nil {
			t.Fatalf("el puntero de %q no resuelve: %v", m[1], err)
		}
		if got.Title != m[1] || sha256.Sum256([]byte(got.Content)) != originals[m[1]] {
			t.Errorf("get_memory %d no devuelve el original íntegro de %q", id, m[1])
		}
	}
}
