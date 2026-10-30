package usecases_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// TestHCD002ReducedContext fija la palanca de Q4: bajo presupuesto, cada
// entrada de get_context lleva un extracto de 80 caracteres como máximo y su
// puntero get_memory, mientras la regla fijada sigue íntegra (C-02).
func TestHCD002ReducedContext(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	memRepo := persistence.NewMemoryRepository(db)
	rules := "REGLA-HCD002-FIJADA: " + strings.Repeat("verifica contra el sistema en ejecución ", 30)
	if _, err := usecases.SeedDefaults(memRepo.(ports.MemorySeeder), memRepo.(ports.MemoryTopicQuerier), "proj", []usecases.Seed{{
		TopicKey: domain.TopicWorkRules, Type: domain.Preference, Title: "Reglas de trabajo del proyecto", Content: rules,
	}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		content := fmt.Sprintf("decisión %d á漢字: ", i) + strings.Repeat(fmt.Sprintf("motivo %d del contrato ", i), 40)
		if _, err := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: fmt.Sprintf("decisión %d", i), Content: content}); err != nil {
			t.Fatal(err)
		}
	}
	builder := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), "", "proj")
	builder.Topics = memRepo.(ports.MemoryTopicQuerier)
	builder.Budget = 24000
	out, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, strings.TrimSpace(rules)) {
		t.Fatal("la regla fijada debe entregarse íntegra")
	}
	entry := regexp.MustCompile("(?m)^- \\*\\*decisión \\d+\\*\\*: (.*) → `get_memory \\d+`$")
	entries := entry.FindAllStringSubmatch(out, -1)
	if len(entries) == 0 {
		t.Fatalf("no hay entradas de decisiones con puntero:\n%s", out)
	}
	for _, e := range entries {
		if n := utf8.RuneCountInString(e[1]); n > 81 {
			t.Errorf("extracto de %d caracteres (máximo 80 más el corte): %q", n, e[1])
		}
	}
}
