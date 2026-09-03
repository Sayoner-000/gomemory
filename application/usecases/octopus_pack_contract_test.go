package usecases_test

import (
	"strings"
	"testing"

	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// specKitConSecreto simula un artefacto de Spec Kit que trae una credencial
// pegada por error. Es la vía que IMPORTA para esta prueba: MemoryRepository.
// Insert ya redacta al persistir, así que una memoria nunca llega sucia al
// paquete. Los artefactos de Spec Kit no pasan por Insert — ahí está el hueco
// real que cubre la redacción del caso de uso.
type specKitConSecreto struct{}

func (specKitConSecreto) ActiveFeature(string) (string, error) { return "027-octopus-aar", nil }

func (specKitConSecreto) Read(_, _, _ string) (domain.SpecKitFeatureContext, error) {
	return domain.SpecKitFeatureContext{
		Feature: "027-octopus-aar",
		Requirements: []string{
			"- **FR-001**: el despliegue usa el token ghp_AAAABBBBCCCCDDDDEEEEFFFFGGGGHHHHIIII",
		},
	}, nil
}

func repoOctopus(t *testing.T) ports.MemoryRepository {
	t.Helper()
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return persistence.NewMemoryRepository(db)
}

// T057 — FR-027: ninguna credencial viaja al contexto delegado. Delegar
// minimiza tokens Y exposición.
func TestPackContract_RedactaSecretosDeFuentesQueNoPasanPorInsert(t *testing.T) {
	uc := usecases.NewPackContractUseCase(
		repoOctopus(t), compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, specKitConSecreto{})

	pkg, err := uc.Build(usecases.PackContractRequest{
		Unit: domain.WorkUnit{
			ID: "T004", Objective: "revisar el despliegue",
			Scope: domain.Scope{Files: []string{"a.go"}, ReadOnly: true},
		},
		Decision: domain.RouteDecision{
			WorkUnitID: "T004", Route: domain.RouteDelegate,
			ContextBudget: 2000, OutputBudget: 800,
		},
		Project:           "proyecto",
		Root:              t.TempDir(),
		IncludeSpecKit:    true,
		ParentPermissions: domain.Permissions{Filesystem: domain.FSReadWrite},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var vioElRequisito bool
	for _, item := range pkg.Pack.Items {
		if strings.Contains(item.Content, "ghp_AAAABBBBCCCCDDDDEEEEFFFFGGGGHHHHIIII") {
			t.Errorf("el secreto viajó al contexto delegado: %q", item.Content)
		}
		if strings.Contains(item.Content, "FR-001") {
			vioElRequisito = true
		}
	}
	// Control: si el artefacto no llegó al paquete, la ausencia del secreto no
	// probaría nada. Sin esto la prueba mentiría con cara de éxito.
	if !vioElRequisito {
		t.Fatal("el requisito de Spec Kit no llegó al paquete: la prueba no está midiendo lo que cree")
	}
}

// Una unidad que no está enrutada como delegada no produce paquete: armar un
// contrato para algo que se ejecuta inline sugeriría una delegación inexistente.
func TestPackContract_RechazaLoQueNoSeDelega(t *testing.T) {
	uc := usecases.NewPackContractUseCase(nil, nil, nil, nil)

	_, err := uc.Build(usecases.PackContractRequest{
		Unit:     domain.WorkUnit{ID: "T001", Objective: "algo"},
		Decision: domain.RouteDecision{WorkUnitID: "T001", Route: domain.RouteInline},
	})
	if err == nil {
		t.Fatal("una unidad inline no debería producir paquete de delegación")
	}
}

// AC-020: el alcance determina los permisos, y nunca se excede al padre.
func TestPackContract_NoElevaPrivilegios(t *testing.T) {
	uc := usecases.NewPackContractUseCase(nil, nil, nil, nil)

	pkg, err := uc.Build(usecases.PackContractRequest{
		Unit: domain.WorkUnit{
			ID: "T004", Objective: "investigar",
			Scope: domain.Scope{Files: []string{"a.go"}, ReadOnly: true},
		},
		Decision:          domain.RouteDecision{WorkUnitID: "T004", Route: domain.RouteDelegate, ContextBudget: 2000, OutputBudget: 800},
		ParentPermissions: domain.Permissions{Filesystem: domain.FSReadWrite},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if pkg.Contract.Permissions.Filesystem != domain.FSReadOnly {
		t.Errorf("permisos = %q, esperaba solo lectura", pkg.Contract.Permissions.Filesystem)
	}

	_, err = uc.Build(usecases.PackContractRequest{
		Unit: domain.WorkUnit{
			ID: "T005", Objective: "modificar", Scope: domain.Scope{Files: []string{"a.go"}},
		},
		Decision:          domain.RouteDecision{WorkUnitID: "T005", Route: domain.RouteDelegate, ContextBudget: 2000, OutputBudget: 800},
		ParentPermissions: domain.Permissions{Filesystem: domain.FSReadOnly},
	})
	if err == nil {
		t.Fatal("delegar no puede conceder escritura que el padre no tiene")
	}
}

// El contrato siempre sale completo y sin autorizar recursión.
func TestPackContract_ContratoCompleto(t *testing.T) {
	uc := usecases.NewPackContractUseCase(nil, nil, nil, nil)

	pkg, err := uc.Build(usecases.PackContractRequest{
		Unit:              domain.WorkUnit{ID: "T004", Objective: "investigar", Scope: domain.Scope{Files: []string{"a.go"}, ReadOnly: true}},
		Decision:          domain.RouteDecision{WorkUnitID: "T004", Route: domain.RouteDelegate, ContextBudget: 2000, OutputBudget: 800},
		ParentPermissions: domain.Permissions{Filesystem: domain.FSReadWrite},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := pkg.Contract.Validate(); err != nil {
		t.Errorf("el contrato debe salir válido: %v", err)
	}
	if pkg.Contract.MaxDepth != 0 {
		t.Errorf("MaxDepth = %d: con el tope de fábrica el hijo no delega", pkg.Contract.MaxDepth)
	}
	if len(pkg.Contract.Output.Required) == 0 {
		t.Error("el contrato debe declarar la forma esperada del resultado")
	}
}
