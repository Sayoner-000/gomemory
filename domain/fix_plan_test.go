package domain

import "testing"

func TestPlanDeReparacion_AgrupaPorComandoYOrdenaProyectoAntesQueUsuario(t *testing.T) {
	plan := PlanDeReparacion([]FixAction{
		{Comando: "mem setup-mcp --scope global --agents codex", Ambito: ScopeUser, Motivos: []string{"codex · user · instructions"}},
		{Comando: "mem install . --yes", Ambito: ScopeProject, Motivos: []string{"claude · project · plan_entry"}},
		{Comando: "mem install . --yes", Ambito: ScopeProject, Motivos: []string{"codex · project · permissions"}},
		{Comando: "mem setup-mcp --scope global --agents codex", Ambito: ScopeUser, Motivos: []string{"codex · user · server_config"}},
	})
	if len(plan) != 2 {
		t.Fatalf("se esperaban 2 acciones sin duplicados, hay %d: %+v", len(plan), plan)
	}
	if plan[0].Ambito != ScopeProject || plan[1].Ambito != ScopeUser {
		t.Fatalf("el proyecto va antes que el usuario: %+v", plan)
	}
	if len(plan[0].Motivos) != 2 || len(plan[1].Motivos) != 2 {
		t.Fatalf("los motivos de los duplicados se conservan: %+v", plan)
	}
}

func TestPlanDeReparacion_AmbitoUsuarioSiempreAdvierte(t *testing.T) {
	plan := PlanDeReparacion([]FixAction{
		{Comando: "codegraph install --target codex --location global --yes", Ambito: ScopeUser},
		{Comando: "mem install . --yes", Ambito: ScopeProject},
	})
	for _, a := range plan {
		switch a.Ambito {
		case ScopeUser:
			if a.Advertencia == "" {
				t.Errorf("una acción de ámbito usuario debe advertir su alcance: %+v", a)
			}
		case ScopeProject:
			if a.Advertencia != "" {
				t.Errorf("una acción de proyecto no advierte: %+v", a)
			}
		}
	}
}

func TestPlanDeReparacion_VacioSinAcciones(t *testing.T) {
	if plan := PlanDeReparacion(nil); len(plan) != 0 {
		t.Fatalf("sin acciones no hay plan: %+v", plan)
	}
}
