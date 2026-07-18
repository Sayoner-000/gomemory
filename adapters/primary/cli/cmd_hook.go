package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// backupKeepEnvOverride permite ajustar cuántos snapshots automáticos de
// backup se retienen por proyecto (specs/009-mitigacion-riesgos, Historia de
// Usuario 1). Mismo patrón que dataHomeEnvOverride en globalstore.go: una
// constante + os.Getenv leída en el punto de uso, sin struct de config nueva.
const backupKeepEnvOverride = "GOMEMORY_BACKUP_KEEP"
const defaultBackupKeep = 10

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
	case "post-compact":
		hookPostCompact(deps)
	case "user-prompt-submit":
		hookUserPromptSubmit(deps)
	case "nudge":
		hookNudge(deps)
	case "turn-end":
		hookTurnEnd(deps)
	case "subagent-stop":
		hookSubagentStop(deps)
	case "plan-approved":
		hookPlanApproved(deps)
	case "prompt":
		hookPrompt(deps)
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
		os.Exit(0) // No se pudo resolver el directorio de trabajo: nada que hacer.
	}
	project := deps.ProjectRepo.Key(root)

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
	project := deps.ProjectRepo.Key(root)

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
	backupSessionSnapshot(deps, project)
	os.Exit(0)
}

// backupSessionSnapshot genera, en modo best-effort, un snapshot automático de
// memorias+relaciones al cerrar sesión (specs/009-mitigacion-riesgos, Historia
// de Usuario 1: mitigar la ausencia de backup entre máquinas). Cualquier error
// se descarta en silencio — regla de oro de este archivo: un hook nunca debe
// abortar ni retrasar el cierre de sesión.
func backupSessionSnapshot(deps *Deps, project string) {
	dir, err := persistence.BackupDir(project)
	if err != nil {
		return
	}

	keep := defaultBackupKeep
	if v := os.Getenv(backupKeepEnvOverride); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			keep = n
		}
	}

	usecases.CreateSnapshot(deps.MemoryRepo, deps.RelationRepo, project, dir, keep)
}

// hookPreCompact se ejecuta ANTES de la compactación del contexto. Es el
// registro legado: su salida es justo lo que la compactación resume/descarta,
// por eso el mecanismo vigente es hookPostCompact (SessionStart matcher=compact),
// cuya salida sobrevive a la compactación. Se conserva el handler para
// instalaciones anteriores que aún lo tengan registrado.
func hookPreCompact(deps *Deps) {
	printRecoveryAndContext(deps)
	os.Exit(0)
}

// hookPostCompact corre DESPUÉS de la compactación (SessionStart matcher=compact).
// A diferencia de PreCompact, su salida sobrevive a la compactación: re-inyecta
// las instrucciones de recuperación + el contexto previo, y borra el marcador de
// sesión para que el siguiente user-prompt-submit re-materialice las tools MCP
// diferidas (que la compactación descarta) vía el bootstrap de ToolSearch.
func hookPostCompact(deps *Deps) {
	if root, err := deps.ProjectRepo.FindRoot(); err == nil {
		os.Remove(sessionMarkerPath(deps, root))
		footprintReset(root) // tras compactar, la huella cuenta desde cero
	}
	printRecoveryAndContext(deps)
	os.Exit(0)
}

// printRecoveryAndContext imprime las instrucciones de recuperación de memoria
// seguidas del contexto de la sesión previa (si hay). Compartido por los hooks
// de pre y post compactación.
func printRecoveryAndContext(deps *Deps) {
	fmt.Print(compactionRecoveryInstructions)

	if _, err := deps.ProjectRepo.FindRoot(); err == nil {
		if ctx, err := deps.ContextBuilder.Build(); err == nil && ctx != "" {
			fmt.Print("\n\nContexto de la sesión previa:\n")
			fmt.Print(ctx)
		}
	}
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
	project := deps.ProjectRepo.Key(root)

	// Provenance: persistir el prompt de este turno en la sesión activa para que
	// InsertMemory lo adjunte a lo que se guarde. Transversal con OpenCode, que
	// hace lo mismo vía `mem hook prompt` desde su evento chat.message.
	if prompt := promptFromStdin(); strings.TrimSpace(prompt) != "" {
		deps.SessionRepo.SetLastPrompt(project, prompt)
	}

	marker := sessionMarkerPath(deps, root)
	if _, err := os.Stat(marker); err == nil {
		// Prompts subsiguientes: ya no son mudos. Si el agente lleva rato sin
		// guardar nada real, se le recuerda (con debounce) que llame a
		// save_memory; si no toca, salida pasiva.
		if msg, ok := computeSaveNudge(deps, root, project); ok {
			data, _ := json.Marshal(map[string]any{"systemMessage": msg})
			fmt.Print(string(data))
		} else {
			fmt.Print("{}")
		}
		os.Exit(0)
	}

	// Primer prompt de la sesión: forzar la carga de las tools MCP diferidas
	// (systemMessage con ToolSearch explícito) e inyectar el recordatorio del
	// protocolo como additionalContext. El campo "tools": true que se usaba
	// antes NO es un campo soportado por Claude Code en UserPromptSubmit: era un
	// no-op silencioso, por eso las tools de gomemory seguían llegando diferidas
	// y la memoria se sentía "manual" hasta que el usuario la mencionaba.
	os.WriteFile(marker, []byte("1"), 0644)
	out := map[string]any{
		"systemMessage": memoryToolBootstrap,
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "UserPromptSubmit",
			"additionalContext": memoryProtocolReminder,
		},
	}
	data, _ := json.Marshal(out)
	fmt.Print(string(data))
	os.Exit(0)
}

// hookNudge imprime, en texto plano, el recordatorio de guardado si corresponde
// (o nada). Es el punto de entrada transversal del nudge para integraciones que
// inyectan contexto por turno pero no consumen el JSON del hook de Claude Code
// —p. ej. el plugin de OpenCode, que lo invoca con `mem hook nudge`—. Comparte
// la decisión con hookUserPromptSubmit vía computeSaveNudge, así el umbral y el
// debounce son idénticos en todos los agentes.
func hookNudge(deps *Deps) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		os.Exit(0)
	}
	project := deps.ProjectRepo.Key(root)
	if msg, ok := computeSaveNudge(deps, root, project); ok {
		fmt.Print(msg)
	}
	os.Exit(0)
}

// hookTurnEnd corre al terminar cada turno del agente (hook "Stop" en Claude
// Code, evento "session.idle" en OpenCode). Registra determinísticamente —
// sin gastar tokens del agente — qué archivos se editaron y qué comandos
// corrieron en el turno recién terminado, como red de seguridad ante
// actividad que el agente no llegó a resumir con save_memory. Turnos de puro
// chat (sin ediciones ni comandos) no generan checkpoint.
func hookTurnEnd(deps *Deps) {
	// Mantiene tibio el snapshot del grafo externo por turno, sin depender de
	// get_context. MaybeRefresh es fire-and-forget (proceso detached, respeta el
	// TTL de 60s + debounce): nunca bloquea el cierre del turno. Cubre Claude
	// Code (Stop) y OpenCode (session.idle), que enrutan ambos a turn-end.
	// DEBE ir ANTES de recordActivityCheckpoint: ese helper termina con
	// os.Exit(0), así que nada después de él se ejecuta. El hijo detached
	// sobrevive al os.Exit del padre (setsid).
	for _, cp := range deps.CodeProviders {
		if cp != nil {
			cp.MaybeRefresh()
		}
	}

	// Recordatorio de compactación (feature 008): si la huella emitida por
	// gomemory en la sesión superó el umbral, sugiere compactar de forma NEUTRAL
	// (sin nombrar comandos de cliente) y no bloqueante. Va ANTES de
	// recordActivityCheckpoint (que consume stdin y hace os.Exit); computeCompactNudge
	// NO consume stdin, así el checkpoint sigue viendo el payload intacto.
	if root, err := deps.ProjectRepo.FindRoot(); err == nil {
		threshold := deps.SettingsRepo.Read(root).CompactThreshold
		if msg, ok := computeCompactNudge(root, threshold); ok {
			data, _ := json.Marshal(map[string]any{"systemMessage": msg})
			fmt.Print(string(data))
		}
	}

	recordActivityCheckpoint(deps, "Checkpoint automático")
}

// hookSubagentStop corre cuando un subagente (tool Task) termina en Claude Code.
// Captura la actividad del subagente que el hook Stop del agente principal NO
// ve: sus ediciones y comandos viven en el transcript propio del subagente —que
// este hook recibe vía transcript_path—, mientras que en el transcript principal
// el subagente aparece solo como un tool_use "Task" (que el parser de actividad
// ignora). En OpenCode no hace falta un equivalente: los subagentes son
// sub-sesiones que emiten session.idle y ya los captura handleTurnEnd.
func hookSubagentStop(deps *Deps) {
	recordActivityCheckpoint(deps, "Checkpoint de subagente")
}

// hookPlanApproved corre cuando el usuario APRUEBA un plan. Es la captura
// determinista del hueco que dejaban los demás hooks: un turno de plan mode es
// puro chat —el modelo escribe el plan y no hay ediciones ni comandos— así que el
// checkpoint de turn-end lo descarta por vacío (activity.empty()) y el nudge rara
// vez llega a tiempo. Aquí, sin gastar tokens del agente y sin depender de que
// decida guardar, se persiste el plan aprobado como memoria type=decision. El
// prompt originante (el `/plan ...`) lo adjunta InsertMemory automáticamente desde
// la sesión activa. Best-effort.
//
// Es transversal a todos los agentes (misma filosofía que turn-end/nudge/prompt):
// la lógica vive aquí en Go y cada agente la invoca con su propia señal —
//   - Claude Code: hook PostToolUse con matcher ExitPlanMode; el plan llega en
//     `tool_input.plan`. PostToolUse solo dispara si el usuario aprobó (un plan
//     rechazado no ejecuta la tool), así que solo se capturan planes aceptados.
//   - OpenCode y otros: invocan `mem hook plan-approved` con `{"plan":"..."}` en
//     stdin (campo `plan` de nivel superior), igual que `mem hook prompt`.
//
// Por eso extractPlanFromPayload acepta ambas formas del payload.
func hookPlanApproved(deps *Deps) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		os.Exit(0)
	}
	project := deps.ProjectRepo.Key(root)

	payload := readHookStdin()
	if payload == nil {
		os.Exit(0)
	}

	plan := extractPlanFromPayload(payload)
	if plan == "" {
		os.Exit(0) // Sin texto de plan: nada que guardar.
	}

	sessionID := ""
	if sess, _ := deps.SessionRepo.Active(project); sess != nil {
		sessionID = sess.ID
	}

	mem := domain.Memory{
		Project:   project,
		SessionID: sessionID,
		Type:      domain.Decision,
		Title:     planTitle(plan),
		Content:   plan,
	}
	deps.MemoryRepo.Insert(&mem)
	os.Exit(0)
}

// extractPlanFromPayload obtiene el texto del plan del payload del hook, aceptando
// las dos formas transversales: la de Claude Code (PostToolUse anida el input de
// la tool en `tool_input`, y ExitPlanMode expone el plan en `tool_input.plan`) y
// la genérica (`plan` de nivel superior) que usan OpenCode y cualquier otro agente
// al invocar `mem hook plan-approved` con `{"plan":"..."}`. Devuelve "" si ninguna
// está presente.
func extractPlanFromPayload(payload map[string]any) string {
	if p, ok := payload["plan"].(string); ok {
		if s := strings.TrimSpace(p); s != "" {
			return s
		}
	}
	if ti, ok := payload["tool_input"].(map[string]any); ok {
		if p, ok := ti["plan"].(string); ok {
			return strings.TrimSpace(p)
		}
	}
	return ""
}

// planTitle deriva un título breve del plan: la primera línea con contenido, sin
// marcadores de encabezado markdown, acotada para no inflar el título.
func planTitle(plan string) string {
	line := ""
	for _, l := range strings.Split(plan, "\n") {
		if s := strings.TrimSpace(l); s != "" {
			line = strings.TrimSpace(strings.TrimLeft(s, "#*-> "))
			break
		}
	}
	if line == "" {
		line = "plan aprobado"
	}
	const maxLen = 80
	if len(line) > maxLen {
		line = strings.TrimSpace(line[:maxLen]) + "…"
	}
	return "Plan aprobado: " + line
}

// hookPrompt persiste el prompt del usuario del turno en curso (recibido por
// stdin como {"prompt": "..."}) en la sesión activa, para que InsertMemory lo
// adjunte como provenance a lo que se guarde. Es el punto de entrada transversal
// del guardado de prompt para integraciones que no comparten el flujo inline de
// `user-prompt-submit` de Claude Code —p. ej. el plugin de OpenCode, que lo
// invoca con `mem hook prompt` desde su evento chat.message—. Best-effort.
func hookPrompt(deps *Deps) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		os.Exit(0)
	}
	if prompt := promptFromStdin(); strings.TrimSpace(prompt) != "" {
		deps.SessionRepo.SetLastPrompt(deps.ProjectRepo.Key(root), prompt)
	}
	os.Exit(0)
}

// promptFromStdin extrae el texto del prompt del payload JSON en stdin (campo
// "prompt"). Devuelve "" si no hay pipe, el parseo falla o el campo no está.
func promptFromStdin() string {
	payload := readHookStdin()
	if payload == nil {
		return ""
	}
	if s, ok := payload["prompt"].(string); ok {
		return s
	}
	return ""
}

// recordActivityCheckpoint es el cuerpo compartido de los hooks que registran un
// checkpoint de actividad (turn-end y subagent-stop). Extrae la actividad del
// transcript o del payload y, si no está vacía, la guarda como checkpoint y
// reindexa los .go tocados. Best-effort: ante cualquier error sale con código 0.
func recordActivityCheckpoint(deps *Deps, title string) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		os.Exit(0)
	}
	project := deps.ProjectRepo.Key(root)

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
		Title:     title,
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

// memoryToolBootstrap fuerza la carga de las tools MCP de gomemory. En Claude
// Code las tools de un MCP server llegan DIFERIDAS: existen por nombre pero su
// esquema no está cargado, así que no pueden invocarse hasta ejecutar un
// ToolSearch que las materialice. Sin este empujón, el agente "sabe" que hay
// memoria (por el recordatorio de protocolo) pero no puede llamarla hasta que
// el usuario la menciona — exactamente lo que hacía sentir la memoria pasiva.
// El único mecanismo que Claude Code respeta para esto es un systemMessage con
// el select explícito. Se emite SOLO aquí porque este hook lo registra
// únicamente Claude Code; OpenCode carga las tools por su plugin y el resto de
// agentes por las instrucciones MCP nativas, así que no necesitan este bootstrap.
const memoryToolBootstrap = `PRIMERA ACCIÓN — ejecuta este ToolSearch AHORA, antes de responder al usuario, ` +
	`para cargar las tools de memoria del proyecto:
select:mcp__gomemory__get_context,mcp__gomemory__save_memory,mcp__gomemory__search_memories,mcp__gomemory__list_memories,mcp__gomemory__get_memory,mcp__gomemory__forget_memory,mcp__gomemory__judge_memories,mcp__gomemory__start_session,mcp__gomemory__end_session
Luego llama a get_context() para cargar la memoria previa del proyecto antes de continuar.`

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
