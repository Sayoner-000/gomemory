package domain

import (
	"strings"
	"testing"
)

// TestEfecto_TodoCanalDeclaraQueSePierde cubre FR-001 y FR-008: cada tipo de
// canal declara qué deja de ocurrir cuando no funciona, en términos de quien
// trabaja y no del mecanismo.
//
// El informe decía "no encontrado: <ruta>". Nombraba el archivo ausente y no lo
// que se pierde por ello, así que quien leía tenía que conocer el sistema por
// dentro para saber si le importaba.
func TestEfecto_TodoCanalDeclaraQueSePierde(t *testing.T) {
	for _, k := range TodosLosCanales() {
		efecto := EfectoDelCanal(k)
		if strings.TrimSpace(efecto) == "" {
			t.Errorf("el canal %q no declara qué se pierde cuando falla", k)
		}
		if strings.Contains(strings.ToLower(efecto), "hook") || strings.Contains(efecto, ".json") {
			t.Errorf("el canal %q describe el mecanismo y no el efecto: %q", k, efecto)
		}
	}
}

// TestCorreccion_TodoCanalCorregibleTieneComando cubre FR-002: un resultado
// corregible indica el comando que lo corrige.
func TestCorreccion_TodoCanalCorregibleTieneComando(t *testing.T) {
	for _, agente := range KnownAgents {
		for _, scope := range []AgentScope{ScopeProject, ScopeUser} {
			if !agente.Scopes[scope] {
				continue
			}
			r := CorreccionPara(agente.Name, scope)
			if strings.TrimSpace(r.Comando) == "" {
				t.Errorf("agente %q en ámbito %q no declara comando de corrección", agente.Name, scope)
			}
		}
	}
}

// TestCorreccion_DeclaraAlcanceCuandoSaleDelProyecto cubre FR-006: si la
// corrección modifica artefactos fuera del proyecto, se advierte antes de
// proponerla.
func TestCorreccion_DeclaraAlcanceCuandoSaleDelProyecto(t *testing.T) {
	for _, agente := range KnownAgents {
		if !agente.Scopes[ScopeUser] {
			continue
		}
		r := CorreccionPara(agente.Name, ScopeUser)
		if strings.TrimSpace(r.Advertencia) == "" {
			t.Errorf("la corrección de %q en ámbito de usuario no advierte que afecta a otros proyectos", agente.Name)
		}
	}
}

// TestCorreccion_AmbitoDeProyectoNoAdvierte: una corrección contenida en el
// proyecto no debe alarmar sin motivo.
func TestCorreccion_AmbitoDeProyectoNoAdvierte(t *testing.T) {
	for _, agente := range KnownAgents {
		if !agente.Scopes[ScopeProject] {
			continue
		}
		if r := CorreccionPara(agente.Name, ScopeProject); r.Advertencia != "" {
			t.Errorf("la corrección de %q en ámbito de proyecto advierte sin salir del proyecto: %q", agente.Name, r.Advertencia)
		}
	}
}
