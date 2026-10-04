package cli

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/domain"
)

// Estado por conversación (feature 035). El estado por turno —huella,
// debounces, aviso pendiente— era por proyecto y ningún inicio de conversación
// lo reiniciaba de forma coherente: un aviso de compactación dejado por una
// conversación de Codex llegaba al segundo prompt de la siguiente.

func conversationPath(root string) string {
	return filepath.Join(root, persistence.MemDir, ".conversation")
}

// endConversation deja el próximo arranque sin id libre para crear una
// conversación local nueva, tanto desde el hook como desde la CLI.
func endConversation(root string) {
	_ = os.Remove(conversationPath(root))
}

// readConversation lee la conversación registrada. Un archivo ilegible o sin
// id equivale a ausente y se borra: el estado nunca bloquea un turno.
func readConversation(root string) (*domain.Conversation, bool) {
	raw, err := os.ReadFile(conversationPath(root))
	if err != nil {
		return nil, false
	}
	var c domain.Conversation
	if json.Unmarshal(raw, &c) != nil || c.ID == "" {
		_ = os.Remove(conversationPath(root))
		return nil, false
	}
	return &c, true
}

func writeConversation(root string, c domain.Conversation) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(conversationPath(root)), 0o700); err != nil {
		return err
	}
	// writeFileAtomic (cmd_mcp_setup.go): varios procesos —un MCP por agente—
	// escriben estos archivos a la vez; sin rename atómico se entrelazaban.
	return writeFileAtomic(conversationPath(root), data, 0o600)
}

// currentConversationID es el id de la conversación registrada, o "" si no hay.
func currentConversationID(root string) string {
	if c, ok := readConversation(root); ok {
		return c.ID
	}
	return ""
}

// conversationAge devuelve los segundos desde que empezó la conversación
// registrada. ok=false si no hay ninguna.
func conversationAge(root string) (int64, bool) {
	c, ok := readConversation(root)
	if !ok || c.StartedAt <= 0 {
		return 0, false
	}
	age := time.Now().Unix() - c.StartedAt
	if age < 0 {
		age = 0
	}
	return age, true
}

// perTurnStatePaths es el estado que pertenece a una conversación
// (data-model.md). Ninguno debe sobrevivir a la conversación que lo generó.
func perTurnStatePaths(deps *Deps, root string) []string {
	return []string{
		footprintPath(root),
		nudgeStatePath(deps, root),
		compactNudgeStatePath(root),
		preferenceNudgeStatePath(root),
		pendingAgentNoticePath(root),
		sessionMarkerPath(deps, root),
		planEnteredMarkerPath(deps, root),
		planReminderMarkerPath(root),
		octopusEmittedPath(root),
		markersNotePath(root),
	}
}

// beginConversation es el ÚNICO punto de inicio de una conversación (feature
// 035, FR-016): lo llaman session-start, `mem session start` (OpenCode) y el
// autoarranque del MCP. Si convID abre una conversación nueva, borra todo el
// estado por turno, registra la conversación y cierra la sesión de memoria que
// lleve horas inactiva. Una reanudación (mismo id) no toca nada.
func beginConversation(deps *Deps, root, convID string) {
	stored, _ := readConversation(root)
	// Sin id del host no podemos comparar conversaciones. Un cierre explícito
	// borra el registro; si faltó ese cierre, la inactividad de la sesión es
	// la única señal fiable para abrir una conversación local nueva.
	if convID == "" && stored != nil && rotateStaleSession(deps, root) {
		stored = nil
	}
	if !domain.IsNewConversation(stored, convID) {
		return
	}
	now := time.Now().Unix()
	if convID == "" {
		convID = domain.NewLocalConversationID(now, randomSuffix())
	}
	for _, p := range perTurnStatePaths(deps, root) {
		_ = os.Remove(p)
	}
	_ = writeConversation(root, domain.Conversation{ID: convID, StartedAt: now})
	rotateStaleSession(deps, root)
}

func randomSuffix() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// staleSessionSummary es el resumen con que se cierra una sesión inactiva.
const staleSessionSummary = "Sesión cerrada automáticamente por inactividad (más de 4 h sin actividad) al iniciar una conversación nueva."

// rotateStaleSession cierra la sesión activa si lleva más de
// domain.StaleSessionSecs sin escrituras y abre otra (FR-021). Con actividad
// reciente no rota: otro agente puede estar trabajando en el proyecto.
// Best-effort: sin el puerto de actividad o ante cualquier error, no hace nada.
func rotateStaleSession(deps *Deps, root string) bool {
	if deps == nil || deps.SessionRepo == nil || deps.ProjectRepo == nil {
		return false
	}
	reader, ok := deps.SessionRepo.(ports.SessionActivityReader)
	if !ok {
		return false
	}
	project := deps.ProjectRepo.Key(root)
	active, err := deps.SessionRepo.Active(project)
	if err != nil || active == nil {
		return false
	}
	ts, ok, err := reader.LastActivity(active.ID)
	if err != nil || !ok {
		return false
	}
	idle, ok := ageSeconds(ts)
	if !ok || !domain.ShouldRotate(idle) {
		return false
	}
	if deps.SessionRepo.End(active.ID, staleSessionSummary) != nil {
		return false
	}
	backupSessionSnapshot(deps, project)
	_, _ = deps.SessionRepo.Start(project)
	return true
}
