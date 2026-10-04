package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mem/adapters/secondary/persistence"
)

// Umbrales del recordatorio de guardado (nudge). Alineados con el
// comportamiento de referencia: no molestar al arranque, recordar tras un rato
// largo sin guardar, y no repetir el recordatorio con demasiada frecuencia.
const (
	nudgeMinSessionAgeSecs = 300 // no molestar en los primeros 5 min de sesión
	nudgeThresholdSecs     = 900 // 15 min sin un guardado real → recordar
	nudgeCooldownSecs      = 900 // tras recordar, callar 15 min antes de repetir
)

const saveNudgeMessage = `RECORDATORIO DE MEMORIA: pasaron más de 15 minutos desde tu último guardado real. ` +
	`Si en ese tiempo tomaste una decisión, corregiste un bug, descubriste algo no obvio o ` +
	`estableciste una convención, llama a save_memory ahora. Si no hay nada relevante que guardar, ` +
	`ignora este recordatorio.`

// planModeReminderMessage es el recordatorio de una línea del modo plan
// atómico (feature 019, Historia 2): cubre a los agentes sin señal de
// entrada observable (mejor esfuerzo del borde de entrada) y refuerza a los
// que sí la tienen, sin competir con las directivas del brazo extensor de
// grafo de código — nombra el grafo como el instrumento de exploración, no
// como un mandato rival (research.md §10, INV-5).
const planModeReminderMessage = "Si vas a entrar en modo plan (o la tarea pide un plan/enfoque antes de tocar código), " +
	"llama a get_plan_context() ANTES de redactar: trae el método de descomposición atómica y el " +
	"historial del proyecto. Usa el grafo de código para explorar y el árbol de tareas atómicas para " +
	"presentar el resultado."

// computePlanModeReminder decide si el turno debe llevar el recordatorio de
// modo plan. A diferencia de computeSaveNudge, NO tiene debounce: se emite en
// CADA turno mientras la planificación atómica esté activa (FR-003, FR-006) —
// el coste es una sola línea, y la garantía que sostiene es "en cualquier
// punto de la sesión", no "de vez en cuando".
func computePlanModeReminder(atomicPlanDisabled bool) (string, bool) {
	if atomicPlanDisabled {
		return "", false
	}
	return planModeReminderMessage, true
}

// nudgeStatePath es el marcador de debounce: guarda el epoch del último
// recordatorio emitido para no repetirlo dentro del período de enfriamiento.
func nudgeStatePath(deps *Deps, root string) string {
	return filepath.Join(root, deps.ProjectRepo.MemDir(), ".last-nudge")
}

// computeSaveNudge es la ÚNICA fuente de la decisión "¿toca recordar que
// guarde?" para todos los agentes: la comparten el hook UserPromptSubmit de
// Claude Code y el evento `mem hook nudge` que invocan el plugin de OpenCode y
// cualquier otra integración con inyección por turno. Devuelve el texto del
// recordatorio y true solo cuando: hay sesión activa con más de 5 min de vida,
// el último guardado real fue hace más de 15 min (o no hay ninguno y la sesión
// ya superó ese umbral), y no se emitió otro recordatorio en los últimos 15 min.
// Best-effort: ante cualquier error o duda, devuelve ("", false) — nunca molesta
// de más ni rompe el turno.
func computeSaveNudge(deps *Deps, root, project string) (string, bool) {
	active, err := deps.SessionRepo.Active(project)
	if err != nil || active == nil {
		return "", false
	}
	sessionAge, ok := ageSeconds(active.CreatedAt)
	if !ok {
		return "", false
	}

	// El reloj es la conversación, no la sesión de memoria: Codex no cierra
	// sesiones y una conversación nueva heredaba una sesión de horas, así que el
	// recordatorio saltaba enseguida (feature 035, FR-020).
	age := sessionAge
	if convAge, ok := conversationAge(root); ok {
		age = convAge
	}
	secs, exists, err := deps.MemoryRepo.SecondsSinceLastSave(project)
	if err != nil {
		return "", false
	}
	since := int64(-1)
	if exists {
		since = secs
	}
	if !saveNudgeDue(age, since) {
		return "", false
	}

	// Debounce: no repetir si ya recordamos hace poco.
	now := time.Now().Unix()
	state := nudgeStatePath(deps, root)
	if raw, err := os.ReadFile(state); err == nil {
		if last, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64); err == nil {
			if now-last < nudgeCooldownSecs {
				return "", false
			}
		}
	}
	_ = writeFileAtomic(state, []byte(strconv.FormatInt(now, 10)), 0644)
	return saveNudgeMessage, true
}

// ageSeconds interpreta un timestamp SQLite ('YYYY-MM-DD HH:MM:SS') escrito con
// el mismo offset que Now ('-5 hours') y devuelve los segundos transcurridos. El
// offset se cancela: tanto el timestamp guardado como la referencia están en el
// mismo marco horario, así que la diferencia es tiempo real.
func ageSeconds(ts string) (int64, bool) {
	t, err := time.Parse("2006-01-02 15:04:05", strings.TrimSpace(ts))
	if err != nil {
		return 0, false
	}
	ref := time.Now().UTC().Add(-5 * time.Hour)
	d := ref.Sub(t)
	if d < 0 {
		return 0, true // relojes con desfase leve: tratar como recién creada
	}
	return int64(d.Seconds()), true
}

// planReminderMarkerPath marca que el recordatorio de modo plan ya acompañó la
// entrada a plan en esta conversación (feature 035, FR-008). Se borra al
// compactar y al empezar una conversación nueva.
func planReminderMarkerPath(root string) string {
	return filepath.Join(root, persistence.MemDir, ".plan-reminder-emitted")
}

// claimPlanReminder decide, para Claude Code, si este turno lleva el
// recordatorio de modo plan: solo la primera vez que llega permission_mode=plan
// en la conversación.
func claimPlanReminder(root string, payload map[string]any) bool {
	if mode, _ := payload["permission_mode"].(string); mode != "plan" {
		return false
	}
	if _, err := os.Stat(planReminderMarkerPath(root)); err == nil {
		return false
	}
	_ = writeHookMarker(planReminderMarkerPath(root))
	return true
}

// octopusEmittedPath guarda la última activación de Octopus que se le comunicó
// al agente por el canal acumulativo (user-prompt-submit).
func octopusEmittedPath(root string) string {
	return filepath.Join(root, persistence.MemDir, ".octopus-emitted")
}

func recordOctopusEmitted(root string, enabled bool) {
	_ = os.MkdirAll(filepath.Dir(octopusEmittedPath(root)), 0o700)
	_ = writeFileAtomic(octopusEmittedPath(root), []byte(strconv.FormatBool(enabled)), 0o600)
}

// octopusDelegationReminderOnChange emite la regla de delegación solo cuando la
// activación cambió respecto a lo último comunicado (FR-008a). Apagar Octopus
// no emite nada, pero se anota para que volver a encenderlo sí se comunique.
func octopusDelegationReminderOnChange(root string, enabled bool) (string, bool) {
	last := ""
	if raw, err := os.ReadFile(octopusEmittedPath(root)); err == nil {
		last = strings.TrimSpace(string(raw))
	}
	if last == strconv.FormatBool(enabled) {
		return "", false
	}
	recordOctopusEmitted(root, enabled)
	return octopusDelegationReminder(enabled, "mcp__gomemory__octopus_route_task")
}

// saveNudgeDue decide si toca recordar que guarde. age es la antigüedad de la
// conversación (o de la sesión, si no hay conversación registrada); since, los
// segundos desde el último guardado real (-1 si no hay ninguno). Un guardado
// anterior al inicio de la conversación no cuenta: el reloj es la conversación.
func saveNudgeDue(age, since int64) bool {
	if age < nudgeMinSessionAgeSecs {
		return false
	}
	if since < 0 || since > age {
		since = age
	}
	return since > nudgeThresholdSecs
}
