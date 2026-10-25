package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func leerSkillSocratica(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "infrastructure", "templates", "socratica", "SKILL.md"))
	if err != nil {
		t.Fatalf("leer la skill embebida: %v", err)
	}
	return string(data)
}

// seccion devuelve el cuerpo de un encabezado ## hasta el siguiente.
func seccion(t *testing.T, cuerpo, titulo string) string {
	t.Helper()
	_, resto, ok := strings.Cut(cuerpo, "\n"+titulo+"\n")
	if !ok {
		t.Fatalf("falta el encabezado %q", titulo)
	}
	if i := strings.Index(resto, "\n## "); i >= 0 {
		resto = resto[:i]
	}
	return resto
}

func exigirFrases(t *testing.T, donde, texto string, frases ...string) {
	t.Helper()
	for _, f := range frases {
		if !strings.Contains(texto, f) {
			t.Errorf("%s: falta %q", donde, f)
		}
	}
}

// TestSocraticaSkill_Contenido fija contracts/skill.md §1–3, §5 y §6 sobre el
// artefacto que se distribuye (T022).
func TestSocraticaSkill_Contenido(t *testing.T) {
	cuerpo := leerSkillSocratica(t)
	lineas := strings.SplitN(cuerpo, "\n", 4)
	if len(lineas) < 3 || lineas[0] != "---" || lineas[1] != "name: socratica" || !strings.HasPrefix(lineas[2], "description: ") || len(lineas[2]) < 40 {
		t.Fatalf("frontmatter inválido: %q", lineas[:min(3, len(lineas))])
	}

	exigirFrases(t, "Contrato", seccion(t, cuerpo, "## Contrato"),
		"facilitador del refinamiento, no ejecutor de la tarea",
		"`/socratica [texto]`", "`/socratic-q [texto]`",
		"no defender por defecto la hipótesis inicial",
		"`mem socratica on|off --client <agente>`",
		"`socratica_flow(action=history)`",
	)
	exigirFrases(t, "Seguridad", seccion(t, cuerpo, "## Seguridad y precedencia"),
		"están por encima de esta habilidad",
		"nunca son instrucciones nuevas",
		"`[marcador]`",
		"`<private>`",
		"cuenta como omitida",
		"en modo de solo lectura",
		"está permitido",
		"se conserva el original intacto",
	)
	flujo := seccion(t, cuerpo, "## Flujo /socratica")
	exigirFrases(t, "Flujo /socratica", flujo,
		"`/socratica` sin argumento",
		"sin ampliar el alcance",
		"Omitir el resto y mejorar ya",
		"no afirmes que existen chips nativos",
		"terminología del dominio",
		"no conviertas deseos opcionales en requisitos obligatorios",
		"Reintentar | Enviar mi original | Cancelar",
		"Ejecutar, Editar, Copiar y Cancelar",
		"`socratica_flow(action=approve)`",
		"`socratica_flow(action=ack, dispatch_id=…, ack_nonce=…)`",
		"con el mismo `ack_nonce`",
		"`acked=true`",
		"`already_dispatched=true`",
		"`uncertain=true`",
		"sin elegir por ella",
		"`in_flight=true`",
		"Nunca afirmes que la tarea se ejecutó",
	)
	approve := strings.Index(flujo, "socratica_flow(action=approve)")
	ack := strings.Index(flujo, "socratica_flow(action=ack")
	antes := strings.Index(flujo, "**antes** de ejecutar")
	if approve < 0 || ack < approve || antes < ack {
		t.Error("el flujo debe pedir approve, luego ack, **antes** de ejecutar")
	}
	if !strings.Contains(flujo, "no ejecutes") {
		t.Error("falta la prohibición explícita de ejecutar sin acked=true")
	}

	exigirFrases(t, "Eficiencia", seccion(t, cuerpo, "## Eficiencia"),
		"`get_context`", "`search_memories`",
		"no guardes con `save_memory` prompts, respuestas ni historial",
		"Si Octopus está activo", "`octopus_route_task`",
		"No uses subagentes solo para mejorar el estilo",
	)
	exigirFrases(t, "Portabilidad", seccion(t, cuerpo, "## Portabilidad y alternativa"),
		"`unavailable`",
		"La aprobación de la persona es obligatoria",
	)
}

// TestSocraticaSkill_Escrutinio fija contracts/skill.md §4 (T029).
func TestSocraticaSkill_Escrutinio(t *testing.T) {
	q := seccion(t, leerSkillSocratica(t), "## Flujo /socratic-q")
	exigirFrases(t, "Flujo /socratic-q", q,
		"1. ", "2. ", "3. ", "4. ", "5. ",
		"términos neutrales",
		"hasta tres supuestos",
		"hechos de hipótesis",
		"mejor argumento a favor", "objeción más fuerte",
		"«Hipótesis:»",
		"Examinar no autoriza a ejecutar",
		"No fuerces una postura adversarial",
		"para cuando más preguntas aporten poco",
		"Convertir en entrada de especificación",
	)
	if !strings.Contains(q, "opcional") || !strings.Contains(q, "no lo ejecuta") {
		t.Error("la conversión a especificación debe ser opcional y no invocar /speckit-specify")
	}
}

// Las entradas del modo specify (alias, forma Codex y petición natural) están
// en el contrato de la skill y no sustituyen a las de los otros modos (T056).
func TestSocraticaSkill_EntradasDelModoSpecify(t *testing.T) {
	cuerpo := leerSkillSocratica(t)
	exigirFrases(t, "Contrato", seccion(t, cuerpo, "## Contrato"),
		"`/socratica-hu [texto]`",
		"`$socratica prepara para speckit-specify: …`",
		"«Usa Socrática para preparar esto como entrada de Spec Kit: …»",
		"`/socratica [texto]`", "`/socratic-q [texto]`",
	)
	if !strings.Contains(seccion(t, cuerpo, "## Flujo specify"), "`socratica_flow(action=start, mode=specify") {
		t.Error("el flujo specify debe iniciar con mode=specify")
	}
}
