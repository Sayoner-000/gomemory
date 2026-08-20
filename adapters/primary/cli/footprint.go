package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mem/adapters/secondary/persistence"
	"mem/domain"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Huella de contexto (feature 008): gomemory no puede leer la ventana del
// cliente, pero SÍ puede medir los bytes que él mismo emite en respuestas de
// tools durante la sesión. Ese acumulado es un proxy honesto —y exactamente la
// métrica que el usuario quiere bajar— para decidir cuándo sugerir compactar.
// Se persiste en un archivo del proyecto porque el proceso MCP (que incrementa)
// y el hook de fin de turno (que lo lee) son procesos distintos.

func footprintPath(root string) string {
	return filepath.Join(root, persistence.MemDir, ".footprint")
}

// footprintRead devuelve el acumulado de la sesión (0 si no hay archivo o es
// inválido). Best-effort: nunca falla hacia afuera.
func footprintRead(root string) int {
	raw, err := os.ReadFile(footprintPath(root))
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// footprintAdd suma n bytes al acumulado. Fire-and-forget: ignora errores para
// no interferir jamás con la respuesta de una tool.
func footprintAdd(root string, n int) {
	if n <= 0 {
		return
	}
	total := footprintRead(root) + n
	p := footprintPath(root)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(strconv.Itoa(total)), 0o644)
}

// footprintReset pone el acumulado en cero (al iniciar sesión y tras compactar,
// para que el umbral mida la huella desde la última compactación).
func footprintReset(root string) {
	_ = os.Remove(footprintPath(root))
}

// callToolResultTextLen suma la longitud del texto emitido en un CallToolResult.
// Otros tipos de resultado (initialize, listas, etc.) no cuentan como huella de
// contenido de memoria.
func callToolResultTextLen(res mcp.Result) int {
	ctr, ok := res.(*mcp.CallToolResult)
	if !ok {
		return 0
	}
	total := 0
	for _, c := range ctr.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			total += len(tc.Text)
		}
	}
	return total
}

// Recordatorio de compactación (P3). NEUTRAL y agnóstico al agente: describe la
// acción («compactar el contexto») sin nombrar ningún comando de cliente
// (/compact, /clear…). El cliente decide cómo actuar. FR-011.
const compactNudgeMessage = `RECORDATORIO DE MEMORIA: la memoria persistente ya aportó ` +
	`bastante contexto a esta sesión. Para abaratar los próximos turnos, considera compactar ` +
	`el contexto de la conversación. Si prefieres seguir así, ignora este recordatorio.`

const compactNudgeCooldownSecs = 1800 // tras recordar, callar 30 min antes de repetir

func compactNudgeStatePath(root string) string {
	return filepath.Join(root, persistence.MemDir, ".last-compact-nudge")
}

// computeCompactNudge decide si el hook de fin de turno debe sugerir compactar:
// hay umbral (>0), la huella emitida lo superó, y no se recordó en los últimos
// 30 min (debounce). Best-effort: ante cualquier duda, ("", false). Es la ÚNICA
// fuente de la decisión, para que todos los agentes compartan umbral y debounce.
func computeCompactNudge(root string, threshold int) (string, bool) {
	if threshold <= 0 {
		return "", false
	}
	if footprintRead(root) < threshold {
		return "", false
	}
	now := time.Now().Unix()
	if raw, err := os.ReadFile(compactNudgeStatePath(root)); err == nil {
		if last, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64); err == nil {
			if now-last < compactNudgeCooldownSecs {
				return "", false
			}
		}
	}
	_ = os.MkdirAll(filepath.Dir(compactNudgeStatePath(root)), 0o755)
	_ = os.WriteFile(compactNudgeStatePath(root), []byte(strconv.FormatInt(now, 10)), 0o644)
	return compactNudgeMessage, true
}

// Refuerzo periódico de preferencias: el protocolo/preferencias solo se
// reinyectan en SessionStart y post-compact (printRecoveryAndContext); en una
// sesión larga que no llega a compactar, una regla como "español neutro" se
// diluye del contexto sin que nada la recuerde. Reusa el mismo contador de
// huella que ya mide cuánto emitió gomemory esta sesión, pero dispara antes
// del umbral de compactación —a un tercio del camino— para reforzar el
// contenido REAL de las preferencias, no un recordatorio genérico.
const preferenceReinforceFraction = 3    // dispara al superar 1/threshold del camino a compactar
const preferenceNudgeCooldownSecs = 1200 // tras reforzar, callar 20 min antes de repetir
const preferenceNudgeMaxItems = 3        // memorias type=preference más recientes a reinyectar

func preferenceNudgeStatePath(root string) string {
	return filepath.Join(root, persistence.MemDir, ".last-preference-nudge")
}

// computePreferenceReinforcement decide si el hook de fin de turno debe
// reinyectar las preferencias del usuario: hay umbral de compactación
// configurado (>0, o el default), la huella emitida superó un tercio de ese
// umbral, no se reforzó en los últimos 20 min (debounce), y existe al menos
// una memoria type=preference guardada. Best-effort: ante cualquier duda,
// ("", false) — nunca bloquea el cierre del turno.
func computePreferenceReinforcement(deps *Deps, root, project string, threshold int) (string, bool) {
	if threshold <= 0 {
		threshold = persistence.DefaultCompactThreshold
	}
	step := threshold / preferenceReinforceFraction
	if step <= 0 || footprintRead(root) < step {
		return "", false
	}

	now := time.Now().Unix()
	if raw, err := os.ReadFile(preferenceNudgeStatePath(root)); err == nil {
		if last, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64); err == nil {
			if now-last < preferenceNudgeCooldownSecs {
				return "", false
			}
		}
	}

	mems, err := deps.MemoryRepo.List(project, 100)
	if err != nil {
		return "", false
	}
	var prefs []domain.Memory
	for _, m := range mems {
		if m.Type == domain.Preference {
			prefs = append(prefs, m)
			if len(prefs) == preferenceNudgeMaxItems {
				break
			}
		}
	}
	if len(prefs) == 0 {
		return "", false
	}

	_ = os.MkdirAll(filepath.Dir(preferenceNudgeStatePath(root)), 0o755)
	_ = os.WriteFile(preferenceNudgeStatePath(root), []byte(strconv.FormatInt(now, 10)), 0o644)

	var b strings.Builder
	b.WriteString("REFUERZO DE PREFERENCIAS (sesión larga sin compactar): estas reglas del usuario siguen activas, no las pierdas de vista:\n\n")
	for _, p := range prefs {
		fmt.Fprintf(&b, "- **%s**: %s\n", p.Title, p.Content)
	}
	return b.String(), true
}

// recordUsage registra una emisión medida (feature 020), convirtiendo
// caracteres a tokens con deps.TokenCounter. Nil-safe: sin UsageRecorder o sin
// TokenCounter, no hace nada — ningún emisor debe verificar esto por su
// cuenta.
func recordUsage(deps *Deps, operation string, rawChars, emittedChars int) {
	if deps == nil || deps.UsageRecorder == nil || deps.TokenCounter == nil {
		return
	}
	raw := deps.TokenCounter.Count(strings.Repeat("x", rawChars))
	emitted := deps.TokenCounter.Count(strings.Repeat("x", emittedChars))
	deps.UsageRecorder.Record(operation, raw, emitted)
}

// rawCharsOf suma el contenido íntegro (sin acotar) de un lote de memorias:
// es la línea base de las tools de divulgación progresiva (search_memories,
// list_memories), que solo emiten un extracto acotado por resultado.
// rawCharsOf es la línea base de search_memories/list_memories: emittedChars
// (lo que realmente salió, extracto ya incluido) más lo que domain.Extract
// cortó de cada memoria frente a su contenido íntegro. NUNCA la suma cruda de
// contenido: esa suma no incluye el envoltorio de la línea renderizada
// (id/tipo/título/formato), así que para memorias cortas —sin nada que
// truncar— podía terminar por DEBAJO de lo emitido, violando la invariante
// baseline >= emitted (bug real encontrado en TestMCPServer_SearchAndList_
// RecordUsage: baseline=32 < emitted=40 antes de este fix). Mismo patrón
// "raw = final + descartado" que Builder.discardedChars en build_context.go.
func rawCharsOf(mems []domain.Memory, emittedChars int) int {
	discarded := 0
	for _, m := range mems {
		extract := domain.Extract(m.Content, memListExtractChars)
		if cut := len(m.Content) - len(extract); cut > 0 {
			discarded += cut
		}
	}
	return emittedChars + discarded
}

// selfReportingTools son las tools MCP que YA registran su propio uso dentro
// del handler (o, para get_context, dentro del Builder concreto que
// container.go cablea) — el middleware de respaldo del canal "mcp" las salta
// para no contarlas dos veces (feature 020).
var selfReportingTools = map[string]bool{
	"search_memories": true,
	"list_memories":   true,
	"get_context":     true,
	"pack_build":      true,
	"pack_compress":   true,
}

// toolOperation traduce el nombre de una tool MCP a la operación de dominio
// que representa (feature 020, FR-003): el dominio no conoce el vocabulario
// de ningún canal. Un nombre no listado aquí —incluidas las tools futuras—
// cae en domain.OpOther, que es un valor legítimo, no un error.
func toolOperation(toolName string) string {
	switch toolName {
	case "save_memory":
		return domain.OpSaveMemory
	case "get_memory":
		return domain.OpGetMemory
	case "get_plan_context":
		return domain.OpPlanContext
	default:
		return domain.OpOther
	}
}
