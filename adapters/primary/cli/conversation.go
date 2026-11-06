package cli

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"mem/adapters/secondary/atomicfs"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/domain"
)

// Estado por conversación (feature 035). El estado por turno —huella,
// debounces, aviso pendiente— era por proyecto y ningún inicio de conversación
// lo reiniciaba de forma coherente: un aviso de compactación dejado por una
// conversación de Codex llegaba al segundo prompt de la siguiente.

func conversationPath(root string) string {
	return perConversationPath(root, ".conversation")
}

// stateConversation es la conversación de agente del proceso (HCD-036). La fija
// Run con deps.Conversation; nil = estado por proyecto, como antes.
var stateConversation ports.ConversationResolver

// perConversationPath ubica un archivo de estado por turno. Con conversación va
// en .memory/conversations/<hash del id>/, para que el arranque o el cierre de
// otro agente no lo borre (C-02); sin ella, en .memory/ como siempre.
func perConversationPath(root, name string) string {
	return perConversationPathIn(root, persistence.MemDir, name)
}

// perConversationPathIn es perConversationPath con el directorio de memoria
// del repositorio de proyecto (las marcas que ya lo usaban lo conservan).
func perConversationPathIn(root, memDir, name string) string {
	dir := filepath.Join(root, memDir)
	if stateConversation != nil {
		if id := stateConversation.Current(); id != "" {
			sum := sha256.Sum256([]byte(id))
			dir = filepath.Join(dir, "conversations", hex.EncodeToString(sum[:8]))
			// Los escritores dan por hecho que el directorio existe.
			_ = os.MkdirAll(dir, 0o700)
		}
	}
	return filepath.Join(dir, name)
}

// endConversation deja el próximo arranque sin id libre para crear una
// conversación local nueva, tanto desde el hook como desde la CLI.
func endConversation(root string) {
	_ = os.Remove(conversationPath(root))
}

// readConversation lee la conversación registrada. Un archivo ilegible o sin
// id equivale a ausente y se borra: el estado nunca bloquea un turno.
func readConversation(root string) (*domain.Conversation, bool) {
	raw, err := atomicfs.ReadFile(conversationPath(root))
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
	// Las sesiones abiertas sin actividad no son de nadie: se cierran aunque
	// sean de otra conversación o anteriores a ellas (HCD-036). Sin esto, una
	// sesión sin conversación quedaba abierta para siempre.
	closeStaleSessions(deps, project, reader)
	active, err := deps.SessionRepo.Active(project)
	if err != nil || active == nil {
		return false
	}
	if !sessionIsStale(reader, active.ID) {
		return false
	}
	if deps.SessionRepo.End(active.ID, staleSessionSummary) != nil {
		return false
	}
	backupSessionSnapshot(deps, project)
	_, _ = deps.SessionRepo.Start(project)
	return true
}

func sessionIsStale(reader ports.SessionActivityReader, id string) bool {
	ts, ok, err := reader.LastActivity(id)
	if err != nil || !ok {
		return false
	}
	idle, ok := ageSeconds(ts)
	return ok && domain.ShouldRotate(idle)
}

// closeStaleSessions cierra las sesiones abiertas del proyecto que llevan
// demasiado tiempo inactivas, de cualquier conversación. Best-effort.
func closeStaleSessions(deps *Deps, project string, reader ports.SessionActivityReader) {
	lister, ok := deps.SessionRepo.(interface {
		OpenSessionIDs(project string) ([]string, error)
	})
	if !ok {
		return
	}
	ids, err := lister.OpenSessionIDs(project)
	if err != nil {
		return
	}
	closed := false
	for _, id := range ids {
		if sessionIsStale(reader, id) && deps.SessionRepo.End(id, staleSessionSummary) == nil {
			closed = true
		}
	}
	if closed {
		backupSessionSnapshot(deps, project)
	}
}
