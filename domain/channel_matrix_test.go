package domain

import (
	"strings"
	"testing"
)

// TestChannelMatrix_CeldaDeclaraRutaOMotivo cubre INV-1 y el contrato C1: una
// celda tiene ruta o motivo, nunca ambos ni ninguno.
//
// Es la regla que convierte la ausencia silenciosa en fallo. Los cuatro
// defectos que originaron esta feature fueron celdas que una actividad conocía
// y otra no; ninguna estaba declarada como inexistente.
func TestChannelMatrix_CeldaDeclaraRutaOMotivo(t *testing.T) {
	for _, c := range ChannelMatrix {
		tieneRuta := len(c.Path) > 0
		tieneMotivo := strings.TrimSpace(c.NotApplicableReason) != ""

		switch {
		case !tieneRuta && !tieneMotivo:
			t.Errorf("%s: sin ruta y sin motivo declarado — una celda no puede quedar sin declarar", c)
		case tieneRuta && tieneMotivo:
			t.Errorf("%s: declara ruta y motivo a la vez — son excluyentes", c)
		}
	}
}

// TestChannelMatrix_RutasRelativas cubre INV-2 y el contrato C6: el dominio no
// conoce el sistema de archivos. Resolver la ruta contra un directorio concreto
// es trabajo del adaptador.
func TestChannelMatrix_RutasRelativas(t *testing.T) {
	for _, c := range ChannelMatrix {
		for _, seg := range c.Path {
			if seg == ".." {
				t.Errorf("%s: la ruta escapa del directorio destino con %q", c, seg)
			}
			if strings.HasPrefix(seg, "/") || strings.HasPrefix(seg, "~") {
				t.Errorf("%s: segmento no relativo %q", c, seg)
			}
		}
	}
}

// TestChannelMatrix_AgenteExiste cubre INV-3 y la primera mitad del contrato
// C2: una celda huérfana es un error de declaración.
func TestChannelMatrix_AgenteExiste(t *testing.T) {
	for _, c := range ChannelMatrix {
		if _, ok := AgentByName(c.Agent); !ok {
			t.Errorf("%s: el agente no existe en KnownAgents", c)
		}
	}
}

// TestChannelMatrix_CapacidadDeclaradaTieneCelda cubre la segunda mitad de C2:
// un agente que declara un nivel en un ámbito debe tener celda para ese canal
// en ese ámbito, o motivo declarado. Sin esto, añadir un agente al registro
// puede quedarse sin efecto en silencio.
func TestChannelMatrix_CapacidadDeclaradaTieneCelda(t *testing.T) {
	porNivel := map[AgentLevel]ChannelKind{
		AgentLevelEntry:     KindPlanEntry,
		AgentLevelGuard:     KindPlanGuard,
		AgentLevelTextFloor: KindInstructions,
	}

	for _, agente := range KnownAgents {
		for nivel, kind := range porNivel {
			if !agente.HasLevel(nivel) {
				continue
			}
			for scope := range agente.Scopes {
				if !agente.Scopes[scope] {
					continue
				}
				if len(CellsFor(agente.Name, kind, scope)) == 0 {
					t.Errorf("agente %q declara el nivel %q con ámbito %q y no tiene celda para el canal %q: añadir un agente no puede quedarse sin efecto",
						agente.Name, nivel, scope, kind)
				}
			}
		}
	}
}

// TestChannelMatrix_ActividadDeProyectoNoAlcanzaAlUsuario cubre el contrato C4
// y el invariante INV-4.
//
// Es el contrato que faltaba cuando una desinstalación dirigida a un proyecto
// eliminó un artefacto compartido por todos los proyectos de la máquina.
func TestChannelMatrix_ActividadDeProyectoNoAlcanzaAlUsuario(t *testing.T) {
	for _, act := range LifecycleActivities {
		// El diagnóstico solo lee: la contención restringe escribir y retirar
		// fuera del alcance, nunca observar.
		if act.ReadOnly || act.Scope != ScopeProject {
			continue
		}
		for _, c := range CellsForActivity(act.Name) {
			if c.Scope == ScopeUser {
				t.Errorf("la actividad %q tiene alcance de proyecto y alcanza la celda de ámbito de usuario %s", act.Name, c)
			}
		}
	}
}

// TestChannelMatrix_SimetriaInstalarDesinstalar cubre el contrato C3: lo que la
// instalación escribe en el ámbito del proyecto es exactamente lo que la
// desinstalación retira.
func TestChannelMatrix_SimetriaInstalarDesinstalar(t *testing.T) {
	escritas := map[string]MatrixCell{}
	for _, c := range CellsForActivity(ActivityInstall) {
		escritas[c.Key()] = c
	}
	retiradas := map[string]bool{}
	for _, c := range CellsForActivity(ActivityUninstall) {
		retiradas[c.Key()] = true
	}

	for k, c := range escritas {
		if !retiradas[k] {
			t.Errorf("%s: la instalación la escribe y la desinstalación no la retira", c)
		}
	}
}

// TestChannelMatrix_LegacyNoSeEscribe cubre INV-5: un artefacto heredado solo
// se retira; volver a escribirlo reintroduciría lo que la versión 2.9.0 quitó.
func TestChannelMatrix_LegacyNoSeEscribe(t *testing.T) {
	for _, c := range CellsForActivity(ActivityInstall) {
		if c.Legacy {
			t.Errorf("%s: es un artefacto heredado y la instalación lo escribe", c)
		}
	}
}
