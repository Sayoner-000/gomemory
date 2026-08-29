package setup

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// skillTarget describe dónde vive el directorio de habilidades de nivel USUARIO
// de un agente, relativo al HOME.
//
// Es una tabla aparte de globalTargets y no una columna nueva en aquella, y la
// razón importa: globalTargets modela "dónde escribo las instrucciones y el
// envoltorio nativo del método de planificación", con rutas ya entregadas y
// probadas para las features 013 y 021. Esta tabla modela otra cosa —"dónde
// acepta este agente una habilidad completa"— y sus rutas no coinciden.
//
// Las tres rutas están verificadas contra los binarios reales, no contra
// documentación: el binario de Codex contiene `.codex/skills` y `SKILL.md`, y
// el de OpenCode contiene literalmente `.opencode/skills/my-skill/SKILL.md`.
type skillTarget struct {
	agent string
	// dir es la raíz de configuración de usuario del agente, relativa al HOME.
	dir []string
	// skills es el directorio de habilidades, relativo a dir.
	skills []string
}

var skillTargets = []skillTarget{
	{agent: "claude", dir: []string{".claude"}, skills: []string{"skills"}},
	{agent: "codex", dir: []string{".codex"}, skills: []string{"skills"}},
	{agent: "opencode", dir: []string{".opencode"}, skills: []string{"skills"}},
}

// adversarialReviewSkillName es el nombre del directorio de la habilidad. Debe
// coincidir con el campo `name` de su frontmatter: los agentes emparejan ambos
// al descubrirla.
const adversarialReviewSkillName = "adversarial-consensus-review"

// InstallAgentSkill deposita una habilidad completa en el ámbito de usuario de
// cada agente que exponga un directorio de habilidades.
//
// `agents` filtra a qué agentes se escribe; vacío significa todos. El filtro no
// es un lujo de configuración, es lo que garantiza UN SOLO DUEÑO por archivo:
// `atomic-decomposition` y `constitution` ya llegan a Claude por
// InstallAtomicPlanGlobal e InstallConstitutionWrappers, así que este canal las
// lleva solo a Codex y OpenCode. Dos rutas de código escribiendo el mismo
// archivo es como empiezan las divergencias silenciosas.
//
// El cuerpo se escribe VERBATIM: quien llama es responsable de que incluya su
// frontmatter. Es deliberado — una habilidad es un artefacto completo, y
// reescribirle la cabecera por agente la convertiría en tres artefactos
// distintos que divergirían.
//
// Capa OPCIONAL e idempotente: solo escribe donde el agente ya tiene su
// directorio de configuración —crear ~/.codex para quien no usa Codex sería
// ensuciar su directorio personal— y solo si el contenido difiere del que ya
// está. Devuelve las rutas efectivamente escritas, para poder informarlas.
func InstallAgentSkill(name, body string, agents ...string) ([]string, error) {
	// Un cuerpo ausente degrada sin escribir: un SKILL.md vacío es peor que
	// ninguno — el agente lo descubriría y cargaría una habilidad rota.
	if strings.TrimSpace(body) == "" || strings.TrimSpace(name) == "" {
		return nil, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	content := []byte(body)
	var written []string
	for _, t := range skillTargets {
		if len(agents) > 0 && !slices.Contains(agents, t.agent) {
			continue
		}

		base := filepath.Join(append([]string{home}, t.dir...)...)
		if info, err := os.Stat(base); err != nil || !info.IsDir() {
			continue
		}

		dest := filepath.Join(append(append([]string{base}, t.skills...), name, "SKILL.md")...)
		if previo, err := os.ReadFile(dest); err == nil && string(previo) == string(content) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			return written, err
		}
		written = append(written, dest)
	}
	return written, nil
}

// InstallAdversarialReviewSkill distribuye la guía de participación en el
// protocolo de revisión adversarial (spec 027, FR-044).
//
// Va a los tres agentes sin filtro: a diferencia de las otras dos habilidades,
// este canal es su único dueño.
//
// La guía es autosuficiente —no depende de las herramientas de gomemory ni de
// que haya memoria persistente—, así que instalarla no promete nada que el
// binario no pueda cumplir hoy. Lo que gomemory añadirá cuando el protocolo
// esté implementado es hacer sus invariantes incumplibles en vez de meramente
// enunciadas.
func InstallAdversarialReviewSkill(body string) ([]string, error) {
	return InstallAgentSkill(adversarialReviewSkillName, body)
}

// atomicPlanSkillFrontmatter y constitutionSkillFrontmatter son las cabeceras
// que necesitan las dos habilidades ya existentes para ser descubiertas. Su
// texto replica el que globalTargets/constitutionWrappers ya usan para Claude:
// misma habilidad, misma descripción, distinto agente.
const atomicPlanSkillFrontmatter = "---\n" +
	"name: atomic-decomposition\n" +
	"description: Descompone una solicitud grande, multi-paso o de resultado incierto en tareas atómicas verificables antes de planificar. Úsala en modo plan.\n" +
	"---\n\n"

const constitutionSkillFrontmatter = "---\n" +
	"name: constitution\n" +
	"description: Aplica la constitución técnica vigente del proyecto, servida desde la memoria de gomemory. Úsala antes de decidir stack, capas o convenciones.\n" +
	"---\n\n"

// InstallExistingSkillsForAgents lleva a Codex y OpenCode las dos habilidades
// que hasta ahora solo alcanzaban a Claude Code.
//
// El hueco que cierra es real y estaba abierto desde las features 013 y 021:
// gomemory escribía habilidades ÚNICAMENTE en `.claude/skills/`, y su tabla de
// canales declaraba que Codex "no expone un formato propio de comandos o
// habilidades". Esa afirmación era cierta cuando se escribió y dejó de serlo
// sin que nada avisara — el informe de estado seguía en verde porque nadie
// comprobaba una celda declarada no aplicable.
//
// Claude queda deliberadamente fuera del filtro: ya recibe ambas por su vía
// propia, hacia las mismas rutas. Ver InstallAgentSkill sobre el dueño único.
func InstallExistingSkillsForAgents(planMethod, constitutionBody string) ([]string, error) {
	var written []string

	if strings.TrimSpace(planMethod) != "" {
		p, err := InstallAgentSkill("atomic-decomposition",
			atomicPlanSkillFrontmatter+strings.TrimSpace(planMethod)+"\n", "codex", "opencode")
		written = append(written, p...)
		if err != nil {
			return written, err
		}
	}

	// La constitución SÍ incluye a Claude, y la diferencia con la línea de
	// arriba no es un descuido. InstallConstitutionWrappers es de ámbito de
	// PROYECTO: escribe en `<root>/.claude/skills/`, no en `~/.claude/skills/`.
	// La ruta global de Claude no tenía dueño, así que dejarla fuera aquí
	// habría dado a Codex y OpenCode una cobertura global que Claude —el agente
	// mejor soportado— no tendría. Esa asimetría la introduciría este mismo
	// cambio, así que se paga aquí y no se hereda.
	if strings.TrimSpace(constitutionBody) != "" {
		p, err := InstallAgentSkill("constitution",
			constitutionSkillFrontmatter+strings.TrimSpace(constitutionBody)+"\n")
		written = append(written, p...)
		if err != nil {
			return written, err
		}
	}

	return written, nil
}

// ConstitutionSkillBody expone el cuerpo compartido de la constitución para que
// el paquete cli pueda pasarlo a InstallExistingSkillsForAgents sin duplicarlo.
func ConstitutionSkillBody() string { return constitutionWrapperBody }
