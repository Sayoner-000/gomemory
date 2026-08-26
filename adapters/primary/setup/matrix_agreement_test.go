package setup

import (
	"path/filepath"
	"testing"

	"mem/domain"
)

// rutasDeLaMatriz devuelve las rutas relativas que la matriz declara para un
// canal y ámbito dados.
func rutasDeLaMatriz(kind domain.ChannelKind, scope domain.AgentScope) map[string]bool {
	out := map[string]bool{}
	for _, c := range domain.ChannelMatrix {
		if c.Kind == kind && c.Scope == scope && c.Applies() && !c.Legacy {
			out[filepath.Join(c.Path...)] = true
		}
	}
	return out
}

// motivoDeclarado reporta si la matriz explica, con un motivo, por qué un
// agente no materializa un canal en un ámbito.
func motivoDeclarado(kind domain.ChannelKind, scope domain.AgentScope, agente string) bool {
	for _, c := range domain.ChannelMatrix {
		if c.Agent == agente && c.Kind == kind && c.Scope == scope && c.NotApplicableReason != "" {
			return true
		}
	}
	return false
}

// contieneBajo reporta si alguna ruta declarada es prefijo de la ruta dada. Los
// envoltorios nativos se declaran a nivel de directorio en la matriz, porque su
// nombre de archivo depende del envoltorio concreto y no del agente.
func contieneBajo(declaradas map[string]bool, ruta string) bool {
	for d := range declaradas {
		if ruta == d || len(ruta) > len(d) && ruta[:len(d)] == d && ruta[len(d)] == filepath.Separator {
			return true
		}
	}
	return false
}

// TestC7_EnvoltoriosDelMetodoConcuerdanConLaMatriz ata la tabla de envoltorios
// del método de planificación a la matriz.
//
// La tabla todavía no se deriva de la matriz, y eso es deliberado: migrar los
// catorce consumidores en un solo cambio es el tipo de refactor que introduce
// los defectos que esta feature previene. Lo que este contrato garantiza es que
// no puedan separarse — una tabla que concuerda y falla al dejar de concordar
// ya no es una isla.
func TestC7_EnvoltoriosDelMetodoConcuerdanConLaMatriz(t *testing.T) {
	for _, w := range atomicPlanWrappers {
		ruta := filepath.Join(w.path...)
		declaradas := rutasDeLaMatriz(domain.KindNativeWrapper, domain.ScopeProject)
		if !contieneBajo(declaradas, ruta) {
			t.Errorf("el envoltorio %q no está declarado en la matriz para el canal de envoltorio nativo", ruta)
		}
	}
}

// TestC7_EnvoltoriosDeLaConstitucionConcuerdanConLaMatriz hace lo mismo para
// los envoltorios de la constitución.
func TestC7_EnvoltoriosDeLaConstitucionConcuerdanConLaMatriz(t *testing.T) {
	for _, w := range constitutionWrappers {
		ruta := filepath.Join(w.path...)
		declaradas := rutasDeLaMatriz(domain.KindNativeWrapper, domain.ScopeProject)
		if !contieneBajo(declaradas, ruta) {
			t.Errorf("el envoltorio %q no está declarado en la matriz", ruta)
		}
	}
}

// TestC7_DestinosGlobalesConcuerdanConLaMatriz ata la tabla de destinos de
// ámbito de usuario: su archivo de instrucciones y su envoltorio nativo.
func TestC7_DestinosGlobalesConcuerdanConLaMatriz(t *testing.T) {
	instrucciones := rutasDeLaMatriz(domain.KindInstructions, domain.ScopeUser)
	envoltorios := rutasDeLaMatriz(domain.KindNativeWrapper, domain.ScopeUser)

	for _, tg := range globalTargets {
		base := filepath.Join(tg.dir...)

		ruta := filepath.Join(base, tg.instructions)
		if !instrucciones[ruta] {
			t.Errorf("el archivo de instrucciones de ámbito de usuario %q no está declarado en la matriz", ruta)
		}

		if len(tg.wrapper) == 0 {
			// Un agente sin formato propio de envoltorio no queda exento: debe
			// DECLARAR por qué en la matriz. El hueco sin motivo sigue siendo
			// un error, que es lo que este contrato existe para atrapar.
			if !motivoDeclarado(domain.KindNativeWrapper, domain.ScopeUser, tg.agent) {
				t.Errorf("el agente %q no instala envoltorio nativo y la matriz no declara por qué", tg.agent)
			}
			continue
		}

		rutaEnv := filepath.Join(append(tg.dir, tg.wrapper...)...)
		if !contieneBajo(envoltorios, rutaEnv) {
			t.Errorf("el envoltorio de ámbito de usuario %q no está declarado en la matriz", rutaEnv)
		}
	}
}

// TestC7_ArchivosDeInstruccionesInspeccionadosConcuerdan ata la lista que usa
// el diagnóstico: todo nombre que inspecciona debe existir en la matriz, sea
// como canal vigente o como artefacto heredado que todavía se retira.
func TestC7_ArchivosDeInstruccionesInspeccionadosConcuerdan(t *testing.T) {
	declarados := map[string]bool{}
	for _, c := range domain.ChannelMatrix {
		if c.Kind != domain.KindInstructions || !c.Applies() {
			continue
		}
		declarados[filepath.Base(filepath.Join(c.Path...))] = true
	}

	for _, nombre := range agentInstructionFiles {
		// Los archivos de reglas propios de otros agentes se inspeccionan como
		// legado que nunca se generó desde gomemory; quedan fuera del acuerdo.
		if nombre == ".cursorrules" || nombre == ".windsurfrules" {
			continue
		}
		if !declarados[nombre] {
			t.Errorf("el diagnóstico inspecciona %q y la matriz no lo declara", nombre)
		}
	}
}

// TestC7_ElDiagnosticoNoInventaCanales cierra FR-007 y SC-007 en su forma
// verificable: todo canal que el inspector emite corresponde a una celda de la
// matriz, y ninguno sale de una lista propia.
//
// El inspector no se deriva todavía de la matriz, y es deliberado: hacerlo
// añadiría al informe los canales de registro de servidor y de permisos, que
// hoy no reporta. Ese cambio pertenece a la especificación 024, que rediseña la
// salida del informe; adelantarlo aquí produciría una salida intermedia que
// nadie pidió. Mientras tanto, este contrato garantiza lo que importa: que el
// inspector no pueda reportar un canal que la matriz no declara.
func TestC7_ElDiagnosticoNoInventaCanales(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	declarados := map[string]bool{}
	for _, c := range domain.ChannelMatrix {
		declarados[c.Agent+"|"+string(c.Scope)+"|"+string(c.Kind)] = true
	}

	for _, ch := range NewActivationInspector().Inspect(t.TempDir()) {
		if ch.Arm != domain.ArmGomemory {
			continue // el brazo extensor es de solo lectura y ajeno a la matriz
		}
		clave := ch.Agent + "|" + string(ch.Scope) + "|" + string(ch.Kind)
		if !declarados[clave] {
			t.Errorf("el diagnóstico reporta el canal [%s · %s · %s] y la matriz no lo declara",
				ch.Agent, ch.Scope, ch.Kind)
		}
	}
}
