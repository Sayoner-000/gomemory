package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	if !ok || sessionAge < nudgeMinSessionAgeSecs {
		return "", false
	}

	secs, exists, err := deps.MemoryRepo.SecondsSinceLastSave(project)
	if err != nil {
		return "", false
	}
	var overdue bool
	if exists {
		overdue = secs > nudgeThresholdSecs
	} else {
		// Sin ningún guardado real todavía: el reloj es la propia sesión, así
		// se recuerda justamente cuando el agente lleva rato trabajando sin
		// registrar nada.
		overdue = sessionAge > nudgeThresholdSecs
	}
	if !overdue {
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
	os.WriteFile(state, []byte(strconv.FormatInt(now, 10)), 0644)
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
