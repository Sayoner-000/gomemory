package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"mem/application/usecases"
	"mem/domain"
)

// CmdHook es el entrypoint portable de los hooks de agentes.
//
// Reemplaza los scripts bash + curl (session-start.sh, session-stop.sh,
// post-compaction.sh, user-prompt-submit.sh) por un único binario que habla
// directo a los repositorios, sin servidor HTTP ni dependencias de shell.
// Funciona igual en Linux, macOS y Windows.
//
// Regla de oro: un hook NUNCA debe abortar el arranque del agente. Ante
// cualquier error se sale con código 0 y, como mucho, sin salida.
func CmdHook(deps *Deps, args []string) {
	if len(args) == 0 {
		// Sin evento: no romper nada.
		os.Exit(0)
	}

	switch args[0] {
	case "session-start":
		hookSessionStart(deps)
	case "session-end":
		hookSessionEnd(deps)
	case "pre-compact":
		hookPreCompact(deps)
	case "user-prompt-submit":
		hookUserPromptSubmit(deps)
	case "turn-end":
		hookTurnEnd(deps)
	default:
		// Evento desconocido: salida vacía, sin error.
		os.Exit(0)
	}
}

// sessionMarkerPath es el archivo que marca que el recordatorio de protocolo
// ya se inyectó en la sesión actual (ver hookUserPromptSubmit). Debe borrarse
// en cada nuevo arranque de sesión para que el recordatorio vuelva a
// inyectarse una vez por sesión, no una sola vez en toda la vida del proyecto.
func sessionMarkerPath(deps *Deps, root string) string {
	return filepath.Join(root, deps.ProjectRepo.MemDir(), ".session-tools-injected")
}

// hookSessionStart inicia (si no existe) la sesión activa e inyecta el
// contexto de sesiones previas como additionalContext del agente.
func hookSessionStart(deps *Deps) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		os.Exit(0) // Proyecto sin .memory/: nada que hacer.
	}
	project := filepath.Base(root)

	if active, _ := deps.SessionRepo.Active(project); active == nil {
		deps.SessionRepo.Start(project)
	}

	// Nueva sesión: el recordatorio del protocolo debe volver a inyectarse en
	// el primer prompt (best-effort; si no existe, os.Remove no falla nada).
	os.Remove(sessionMarkerPath(deps, root))

	if ctx, err := deps.ContextBuilder.Build(); err == nil && ctx != "" {
		fmt.Print(ctx)
	}
	os.Exit(0)
}

// hookSessionEnd cierra la sesión activa como red de seguridad. El resumen
// rico lo aporta el modelo llamando a end_session; aquí solo se garantiza el
// cierre. Acepta un resumen opcional vía payload JSON en stdin.
func hookSessionEnd(deps *Deps) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		os.Exit(0)
	}
	project := filepath.Base(root)

	// Best-effort defensivo: cubre compactación/cierre sin un session-start
	// intermedio, para que el próximo primer prompt vuelva a inyectar el
	// recordatorio del protocolo.
	os.Remove(sessionMarkerPath(deps, root))

	active, err := deps.SessionRepo.Active(project)
	if err != nil || active == nil {
		os.Exit(0) // Sin sesión activa: nada que cerrar.
	}

	summary := ""
	if payload := readHookStdin(); payload != nil {
		if s, ok := payload["summary"].(string); ok {
			summary = s
		}
	}

	deps.SessionRepo.End(active.ID, summary)
	os.Exit(0)
}

// hookPreCompact se ejecuta antes de la compactación del contexto. Inyecta
// instrucciones de recuperación + el contexto previo para que nada se pierda.
func hookPreCompact(deps *Deps) {
	fmt.Print(compactionRecoveryInstructions)

	if _, err := deps.ProjectRepo.FindRoot(); err == nil {
		if ctx, err := deps.ContextBuilder.Build(); err == nil && ctx != "" {
			fmt.Print("\n\nContexto de la sesión previa:\n")
			fmt.Print(ctx)
		}
	}
	os.Exit(0)
}

// hookUserPromptSubmit corre en cada prompt del usuario. En el primer prompt
// de la sesión inyecta el bootstrap de tools y un recordatorio del protocolo
// de memoria; en los siguientes es pasivo para no agregar overhead.
func hookUserPromptSubmit(deps *Deps) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fmt.Print("{}")
		os.Exit(0)
	}

	marker := sessionMarkerPath(deps, root)
	if _, err := os.Stat(marker); err == nil {
		// Prompts subsiguientes: pasivo.
		fmt.Print("{}")
		os.Exit(0)
	}

	// Primer prompt de la sesión: cargar tools MCP + recordatorio de memoria.
	os.WriteFile(marker, []byte("1"), 0644)
	out := map[string]any{
		"tools":             true,
		"additionalContext": memoryProtocolReminder,
	}
	data, _ := json.Marshal(out)
	fmt.Print(string(data))
	os.Exit(0)
}

// hookTurnEnd corre al terminar cada turno del agente (hook "Stop" en Claude
// Code, evento "session.idle" en OpenCode). Registra determinísticamente —
// sin gastar tokens del agente — qué archivos se editaron y qué comandos
// corrieron en el turno recién terminado, como red de seguridad ante
// actividad que el agente no llegó a resumir con save_memory. Turnos de puro
// chat (sin ediciones ni comandos) no generan checkpoint.
func hookTurnEnd(deps *Deps) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		os.Exit(0)
	}
	project := filepath.Base(root)

	payload := readHookStdin()
	if payload == nil {
		os.Exit(0)
	}

	var activity turnActivity
	if tp, ok := payload["transcript_path"].(string); ok && tp != "" {
		activity = extractLastTurnActivity(tp)
	} else {
		activity = turnActivity{
			Files:    stringSliceFromPayload(payload["files"]),
			Commands: stringSliceFromPayload(payload["commands"]),
		}
	}

	if activity.empty() {
		os.Exit(0)
	}

	sessionID := ""
	if sess, _ := deps.SessionRepo.Active(project); sess != nil {
		sessionID = sess.ID
	}

	filePath := ""
	if len(activity.Files) > 0 {
		filePath = activity.Files[0]
	}

	mem := domain.Memory{
		Project:   project,
		SessionID: sessionID,
		Type:      domain.Checkpoint,
		Title:     "Checkpoint automático",
		Content:   formatCheckpoint(activity),
		Filepath:  filePath,
	}
	deps.MemoryRepo.Insert(&mem)

	reindexTouchedGoFiles(deps, root, project, activity.Files)
	os.Exit(0)
}

// reindexTouchedGoFiles mantiene el grafo de código fresco automáticamente:
// tras cada turno, reindexa solo los archivos .go tocados (no todo el
// proyecto). Best-effort — nunca debe hacer fallar el hook, ni con
// CodeGraphRepo nil (containers/tests que no lo configuran).
func reindexTouchedGoFiles(deps *Deps, root, project string, files []string) {
	if deps.CodeGraphRepo == nil {
		return
	}
	var goFiles []string
	for _, f := range files {
		if filepath.Ext(f) != ".go" {
			continue
		}
		rel := f
		if filepath.IsAbs(f) {
			r, err := filepath.Rel(root, f)
			if err != nil || strings.HasPrefix(r, "..") {
				continue // fuera del proyecto
			}
			rel = r
		}
		goFiles = append(goFiles, filepath.ToSlash(rel))
	}
	if len(goFiles) == 0 {
		return
	}
	usecases.NewIndexer(deps.CodeGraphRepo, root, project).IndexFiles(goFiles)
}

func stringSliceFromPayload(v any) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func formatCheckpoint(a turnActivity) string {
	var parts []string
	if len(a.Files) > 0 {
		parts = append(parts, "Editó: "+strings.Join(a.Files, ", "))
	}
	if len(a.Commands) > 0 {
		cmds := a.Commands
		if len(cmds) > 5 {
			cmds = cmds[:5]
		}
		parts = append(parts, "Comandos: "+strings.Join(cmds, "; "))
	}
	return strings.Join(parts, ". ")
}

// readHookStdin lee el payload JSON que el agente pasa por stdin. Devuelve nil
// si no hay datos en pipe (ejecución manual en terminal) o si el parseo falla.
func readHookStdin() map[string]any {
	stat, err := os.Stdin.Stat()
	if err != nil || (stat.Mode()&os.ModeCharDevice) != 0 {
		return nil // No es un pipe: no bloquear leyendo la terminal.
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil || len(data) == 0 {
		return nil
	}
	var payload map[string]any
	if json.Unmarshal(data, &payload) != nil {
		return nil
	}
	return payload
}

const compactionRecoveryInstructions = `**TRAS LA COMPACTACIÓN — PRIMERA ACCIÓN REQUERIDA**

1. Llama a end_session() con un resumen de en qué estábamos trabajando,
   qué se logró y los próximos pasos.
2. Llama a get_context() para recuperar el estado de la sesión previa.
3. Solo ENTONCES continúa trabajando.

No omitas el paso 1. Sin él, todo lo hecho antes de la compactación
se pierde de la memoria.`

const memoryProtocolReminder = `Memoria persistente activa (gomemory). Guarda proactivamente con save_memory ` +
	`inmediatamente después de: una decisión técnica, un bug corregido (con causa raíz), ` +
	`un patrón o convención establecida, o un hallazgo no obvio. No esperes a que el ` +
	`usuario lo pida. La actividad rutinaria (qué archivos se editaron, qué comandos ` +
	`corrieron) ya se registra sola como checkpoint automático — no hace falta duplicarla ` +
	`a mano. Antes de cerrar, llama a end_session(summary).

JUEZ IMPARCIAL: si dos memorias se contradicen (aparecen en "Conflictos sin resolver" del ` +
	`contexto, o las notas al buscar), no asumas que la más reciente tiene razón. Relee el ` +
	`código/archivo fuente actual para verificar cuál refleja los hechos reales y registra el ` +
	`veredicto con judge_memories(id_a, id_b, verdict, confidence, reasoning), explicando en ` +
	`reasoning qué verificaste.

PRIVACIDAD: si vas a guardar algo que incluye un secreto, token o credencial, envuelve esa ` +
	`parte en <private>...</private> — nunca se persiste.

IMPORTANTE — no confundir sistemas: este proyecto usa EXCLUSIVAMENTE las tools MCP de ` +
	`gomemory (save_memory, search_memories, get_memory, list_memories, forget_memory, ` +
	`judge_memories, start_session, end_session, get_context). El sistema de memoria nativo ` +
	`del harness (archivo MEMORY.md bajo ~/.claude/projects/.../memory/) NO aplica aquí — ` +
	`ignóralo por completo en este proyecto y no lo consultes ni escribas en él.`
