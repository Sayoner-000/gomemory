package main

import (
	"strings"
	"testing"

	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/usecases"
	"mem/domain"
)

// T054 — AC-006: una tarea delegada que necesita dos artefactos y una memoria
// recibe eso, no el mundo. El aislamiento de contexto es el objetivo de
// optimización principal: sin él, delegar duplica contexto en vez de acotarlo.
func TestOctopusContextPack_AislaElContexto(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	repo := persistence.NewMemoryRepository(db)

	const proyecto = "proyecto"
	relevante := domain.Memory{
		Project: proyecto, Type: domain.Learning,
		Title:   "La expiración compite con el refresco de memorias",
		Content: "expiration.go toma el lock después de leer; store.go antes.",
	}
	if _, err := repo.Insert(&relevante); err != nil {
		t.Fatalf("insertar memoria relevante: %v", err)
	}

	// Ruido deliberado: temas que nada tienen que ver con la unidad delegada.
	for _, m := range []domain.Memory{
		{Project: proyecto, Type: domain.Decision, Title: "Paleta de colores de la TUI", Content: "lipgloss adaptativo por perfil de terminal"},
		{Project: proyecto, Type: domain.Decision, Title: "Estrategia de publicación de releases", Content: "goreleaser con tags semánticos"},
		{Project: proyecto, Type: domain.Pattern, Title: "Convención de imports", Content: "stdlib, terceros, proyecto"},
	} {
		mm := m
		if _, err := repo.Insert(&mm); err != nil {
			t.Fatalf("insertar ruido: %v", err)
		}
	}

	uc := usecases.NewPackContractUseCase(repo, compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, nil)

	pkg, err := uc.Build(usecases.PackContractRequest{
		Unit: domain.WorkUnit{
			ID:        "T004",
			Objective: "Determinar si la expiración compite con el refresco de memorias",
			Class:     domain.ClassInvestigation,
			Scope:     domain.Scope{Files: []string{"expiration.go", "store.go"}, ReadOnly: true},
		},
		Decision: domain.RouteDecision{
			WorkUnitID: "T004", Route: domain.RouteDelegate,
			ContextBudget: 2000, OutputBudget: 800,
		},
		Project:           proyecto,
		ParentPermissions: domain.Permissions{Filesystem: domain.FSReadWrite},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	todo := strings.Join(contenidos(pkg.Pack), "\n")

	// Control primero: si lo relevante no entró, la ausencia del ruido no
	// probaría aislamiento, solo un paquete vacío.
	if !strings.Contains(todo, "expiration.go") {
		t.Fatalf("la memoria relevante no llegó al paquete: la prueba no mide aislamiento\n%s", todo)
	}
	for _, ajeno := range []string{"lipgloss", "goreleaser", "Convención de imports"} {
		if strings.Contains(todo, ajeno) {
			t.Errorf("contexto no relacionado en el paquete delegado: %q", ajeno)
		}
	}

	if pkg.Pack.TokenCount > pkg.Contract.ContextBudget {
		t.Errorf("el paquete excede su presupuesto: %d > %d", pkg.Pack.TokenCount, pkg.Contract.ContextBudget)
	}
}

func contenidos(p domain.ContextPack) []string {
	out := make([]string, 0, len(p.Items))
	for _, it := range p.Items {
		out = append(out, it.ID+" "+it.Content)
	}
	return out
}
