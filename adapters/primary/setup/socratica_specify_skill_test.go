package setup

import (
	"strings"
	"testing"
)

// seccionFlujoSpecify devuelve el cuerpo de «Flujo specify» de la plantilla
// canónica, la fuente de la que se copian las skills distribuidas.
func seccionFlujoSpecify(t *testing.T) string {
	t.Helper()
	return seccion(t, leerSkillSocratica(t), "## Flujo specify")
}

func posicion(t *testing.T, texto, frase string) int {
	t.Helper()
	i := strings.Index(texto, frase)
	if i < 0 {
		t.Fatalf("falta %q", frase)
	}
	return i
}

// El orden del traspaso es el contrato: comprobar disponibilidad, confirmar,
// invocar solo con concesión y comunicar el resultado real (T038).
func TestSocraticaSpecifySkill_OrdenDelTraspaso(t *testing.T) {
	s := seccionFlujoSpecify(t)
	orden := []string{
		"**Traspaso a Spec Kit.**",
		"**Disponibilidad primero.**",
		"**no llames a `ack`**",
		"**Confirmar.**",
		"`ack_nonce` nuevo",
		"**Invocar una sola vez.**",
		"`execution_granted=true`",
		"**Comunicar el resultado real.**",
		"`report`",
	}
	ultimo := -1
	for _, frase := range orden {
		i := posicion(t, s, frase)
		if i < ultimo {
			t.Errorf("%q aparece antes de lo que debe preceder", frase)
		}
		ultimo = i
	}
}

func TestSocraticaSpecifySkill_DisponibilidadYTextoExacto(t *testing.T) {
	s := seccionFlujoSpecify(t)
	exigirFrases(t, "disponibilidad", s,
		"catálogo activo de skills",
		"Un binario `specify` en PATH o una ruta supuesta no lo demuestran",
		"conserva la preparación y la aprobación",
		"no se consume la autorización ni se instala nada por tu cuenta",
	)
	exigirFrases(t, "invocación", s,
		"con exactamente `approved_prompt` como argumento de usuario",
		"sin reescribirlo ni añadirle prefijos",
		"sin construir un shell con su contenido",
		"sin implementar la necesidad descrita",
	)
	if strings.Contains(s, "mem socratica") {
		t.Error("el traspaso no debe pedir órdenes de shell de gomemory")
	}
}

func TestSocraticaSpecifySkill_ReplayYReenvioNoAutorizan(t *testing.T) {
	s := seccionFlujoSpecify(t)
	exigirFrases(t, "replay", s,
		"Un replay del mismo nonce (`acked=true` sin `execution_granted`)",
		"otro nonce (`already_dispatched=true`)",
		"`get`, `resume` o `approve` nunca autorizan ejecutar",
		"reenviar con `force_resend` (autorización nueva) o dejarlo como está, sin elegir por ella",
	)
	// mark_dispatched never applies to specify (C-007, ACR 042 r2).
	if strings.Contains(s, "mark_dispatched") {
		t.Error("el flujo specify no debe ofrecer mark_dispatched")
	}
}

func TestSocraticaSpecifySkill_ResultadoSeInspeccionaYNoSeSupone(t *testing.T) {
	s := seccionFlujoSpecify(t)
	exigirFrases(t, "resultado", s,
		"Abre el `spec.md` y su checklist generados",
		"comprueba que el alcance no se amplió",
		"traza cada criterio preparado a sus historias de origen",
		"`failed` o `uncertain`",
		"el acuse de `ack` o el estado `dispatched` no prueban que se generó nada",
		"no reintentes con la autorización consumida",
	)
}

func TestSocraticaSpecifySkill_SinHerramientaNoAfirmaPreparacionNiTraspaso(t *testing.T) {
	s := seccionFlujoSpecify(t)
	exigirFrases(t, "sin herramienta", s,
		"`socratica_flow` no existe o responde `unavailable`",
		"no hay preparación validada",
		"no inicies el traspaso gestionado",
	)
}

// Los modos anteriores conservan su texto y su protocolo de ejecución.
func TestSocraticaSpecifySkill_ModosPreviosIntactos(t *testing.T) {
	cuerpo := leerSkillSocratica(t)
	exigirFrases(t, "refine", seccion(t, cuerpo, "## Flujo /socratica"),
		"**Ejecutar.** Entrega el prompt aprobado una sola vez",
		"Llama a `socratica_flow(action=approve)`",
		"Llama a `socratica_flow(action=ack, dispatch_id=…, ack_nonce=…)` **antes** de ejecutar",
		"Ejecuta exactamente `approved_prompt` solo si `ack` responde `acked=true`",
	)
	exigirFrases(t, "socratic-q", seccion(t, cuerpo, "## Flujo /socratic-q"),
		"Examinar no autoriza a ejecutar",
	)
	if strings.Contains(seccion(t, cuerpo, "## Flujo /socratica"), "execution_granted") {
		t.Error("el flujo de refine no debe depender de execution_granted")
	}
}
