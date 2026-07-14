package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mem/adapters/secondary/persistence"

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
