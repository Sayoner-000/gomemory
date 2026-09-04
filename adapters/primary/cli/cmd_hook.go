package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
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
		hookUserPromptSubmit(deps, args[1:])
	case "nudge":
		hookNudge(deps)
	case "octopus-delegation-policy":
		hookOctopusDelegationPolicy(deps, args[1:])
	case "channel-fired":
		hookChannelActivity(deps, args[1:], "")
	case "channel-error":
		hookChannelActivity(deps, args[1:], firstOr(args, 4, "fallo sin descripción"))
	case "turn-end":
		hookTurnEnd(deps, args[1:])
	case "subagent-start":
		hookSubagentStart(deps)
	case "subagent-stop":
		hookSubagentStop(deps)
	case "plan-approved":
		hookPlanApproved(deps)
	case "plan-guard":
		hookPlanGuard(deps, args[1:])
	case "plan-entered":
		hookPlanEntered(deps, args[1:])
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

// planEnteredMarkerPath marca si el documento completo de plan-entered
// (feature 019, Historia 2) ya se emitió en la sesión actual (FR-008:
// reentrar en modo plan no debe repetir el bloque completo). Se borra en los
// mismos puntos que sessionMarkerPath — sesión nueva o tras compactar — para
// que "una vez por sesión" sea consistente con el bootstrap de ToolSearch.
func planEnteredMarkerPath(deps *Deps, root string) string {
	return filepath.Join(root, deps.ProjectRepo.MemDir(), ".plan-entered-emitted")
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
	os.Remove(planEnteredMarkerPath(deps, root))

	if ctx := entregaContextoDeArranque(deps); ctx != "" {
		fmt.Print(ctx)
	}
	os.Exit(0)
}

// entregaContextoDeArranque construye el contexto de la sesión y deja constancia
// de la entrega. Devuelve "" cuando no hay nada que emitir.
//
// Anotar la entrega es lo que permite a get_plan_context no reenviar el mismo
// historial en esta sesión (feature 023, FR-006), igual que ya hacen el handler
// MCP de get_context y `mem context`. Sin esta anotación la supresión no se
// aplicaba NUNCA en uso real: la entrega dominante es este hook —479 llamadas
// por canal cli contra 88 por mcp en un proyecto medido—, así que la guarda se
// quedaba sin nada con que comparar y el historial completo viajaba dos veces
// en la misma sesión.
//
// Vive fuera de hookSessionStart, que termina en os.Exit, para poder verificarla.
func entregaContextoDeArranque(deps *Deps) string {
	ctx, err := deps.ContextBuilder.Build()
	if err != nil || ctx == "" {
		return ""
	}
	if deps.DeliveryLog != nil {
		deps.DeliveryLog.Record(ports.DeliveryContext, usecases.HashDeContenido(ctx))
	}
	return ctx
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
		os.Remove(planEnteredMarkerPath(deps, root))
		footprintReset(root)                      // tras compactar, la huella cuenta desde cero
		os.Remove(preferenceNudgeStatePath(root)) // el refuerzo también arranca de cero
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
func hookUserPromptSubmit(deps *Deps, args []string) {
	// El dialecto se resuelve ANTES de cualquier salida, incluida la de fallo:
	// un `{}` impreso a un agente que lee stdout como contexto le inyectaría
	// esas dos llaves como si fueran una instrucción.
	//
	// El defecto es Claude y NO el neutral de detectDialect: Claude Code y Codex
	// validan el sobre JSON del protocolo de hooks. Los adaptadores que reciben
	// texto plano deben pedirlo explícitamente con --emit=text.
	dialect := dialectClaude
	if v := emitFlagValue(args); isKnownDialect(v) {
		dialect = hookDialect(v)
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		emitHookOutput(renderPromptContext(dialect, ""))
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
		//
		// Va en additionalContext, NO en systemMessage: systemMessage es un
		// aviso que Claude Code muestra SOLO al humano en la terminal, nunca se
		// inyecta en el contexto del modelo (confirmado contra la documentación
		// oficial de hooks). saveNudgeMessage está escrito en segunda persona
		// dirigido al agente ("llama a save_memory ahora") — con systemMessage
		// el agente jamás lo veía, así que el recordatorio de guardado nunca
		// llegaba a aplicarse, solo se mostraba en la UI del usuario.
		var parts []string
		if msg, ok := computeSaveNudge(deps, root, project); ok {
			parts = append(parts, msg)
		}
		// Recordatorio de modo plan (feature 019, Historia 2): en CADA turno,
		// sin debounce — es el camino que cubre a los agentes sin señal de
		// entrada observable, y refuerza a los que sí la tienen.
		if msg, ok := computePlanModeReminder(deps.SettingsRepo.Read(root).AtomicPlanDisabled); ok {
			parts = append(parts, msg)
		}
		// Regla de delegación de Octopus, leída fresca en CADA turno — no solo
		// el primero. El bootstrap completo (más abajo) solo se emite una vez
		// por sesión, protegido por este mismo marker: sin esto, activar
		// Octopus a mitad de sesión no le llegaba nunca al agente raíz hasta
		// reiniciar o borrar el marcador (ACR 029, hallazgo C-002).
		if msg, ok := octopusDelegationReminder(deps.SettingsRepo.Read(root).OctopusEnabled, "mcp__gomemory__octopus_route_task"); ok {
			parts = append(parts, msg)
		}

		emitHookOutput(renderPromptContext(dialect, strings.Join(parts, "\n\n")))
		os.Exit(0)
	}

	// Primer prompt de la sesión: forzar la carga de las tools MCP diferidas y
	// el recordatorio del protocolo, ambos en additionalContext (lo único que
	// Claude Code inyecta en el contexto del modelo). El bootstrap de ToolSearch
	// vivía antes en "systemMessage", que Claude Code muestra SOLO al humano en
	// la terminal y NUNCA inyecta en el contexto del modelo (confirmado contra
	// la documentación oficial de hooks) — por eso el agente nunca ejecutaba el
	// ToolSearch forzado ni materializaba codebase-memory-mcp automáticamente:
	// la instrucción "PRIMERA ACCIÓN — ejecuta este ToolSearch AHORA" jamás
	// llegaba a su contexto, solo aparecía como texto en la UI del usuario. El
	// campo "tools": true que se usaba antes de eso tampoco era un campo
	// soportado por Claude Code en UserPromptSubmit: era un no-op silencioso.
	//
	// Se dispara SIEMPRE aquí, sin mirar qué pide el prompt (chat, plan,
	// resumen, lo que sea): el protocolo se declara "OBLIGATORIO y SIEMPRE
	// ACTIVO" sin excepción por tipo de tarea, y materializar las tools bajo
	// demanda según el propósito detectado sería precisamente la excepción que
	// ese principio prohíbe.
	os.WriteFile(marker, []byte("1"), 0644)
	settings := deps.SettingsRepo.Read(root)
	bootstrap := buildMemoryToolBootstrap(!settings.CodeGraphDisabled, settings.OctopusEnabled)
	out := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "UserPromptSubmit",
			"additionalContext": bootstrap + "\n\n" + memoryProtocolReminder,
		},
	}
	data, _ := json.Marshal(out)
	fmt.Print(string(data))
	os.Exit(0)
}

// hookNudge imprime, en texto plano, los recordatorios de turno que
// correspondan (o nada). Es el punto de entrada transversal para integraciones
// que inyectan contexto por turno pero no consumen el JSON del hook de Claude
// Code —p. ej. el plugin de OpenCode, que lo invoca con `mem hook nudge` en
// CADA turno vía chat.system.transform—. Comparte la decisión con
// hookUserPromptSubmit (save-nudge) y hookTurnEnd (compact-nudge / refuerzo de
// preferencias) vía las mismas funciones compute*, así el umbral y el
// debounce son idénticos en todos los agentes.
//
// En Claude Code estos tres recordatorios viajan por eventos separados
// (UserPromptSubmit para el de guardado, Stop para compactar/preferencias)
// porque ahí sí existe un canal de additionalContext en el evento Stop. En
// OpenCode no hay un equivalente: session.idle (donde correría el chequeo de
// compactar/preferencias) ya terminó el turno y no tiene forma de inyectar
// contenido en la respuesta que el modelo ya dio. Por eso los tres se
// consolidan aquí, en el único punto de OpenCode que sí llega al modelo: el
// arranque del turno siguiente.
func hookNudge(deps *Deps) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		os.Exit(0)
	}
	project := deps.ProjectRepo.Key(root)

	var parts []string
	if msg, ok := computeSaveNudge(deps, root, project); ok {
		parts = append(parts, msg)
	}
	if msg, ok := computePlanModeReminder(deps.SettingsRepo.Read(root).AtomicPlanDisabled); ok {
		parts = append(parts, msg)
	}

	threshold := deps.SettingsRepo.Read(root).CompactThreshold
	if msg, ok := computeCompactNudge(root, threshold); ok {
		parts = append(parts, msg)
	} else if msg, ok := computePreferenceReinforcement(deps, root, project, threshold); ok {
		// Misma exclusividad que hookTurnEnd: si ya se sugirió compactar, no
		// compite por espacio con el refuerzo de preferencias.
		parts = append(parts, msg)
	}

	fmt.Print(strings.Join(parts, "\n\n"))
	os.Exit(0)
}

// hookTurnEnd corre al terminar cada turno del agente (hook "Stop" en Claude
// Code, evento "session.idle" en OpenCode). Registra determinísticamente —
// sin gastar tokens del agente — qué archivos se editaron y qué comandos
// corrieron en el turno recién terminado, como red de seguridad ante
// actividad que el agente no llegó a resumir con save_memory. Turnos de puro
// chat (sin ediciones ni comandos) no generan checkpoint.
func hookTurnEnd(deps *Deps, args []string) {
	// Igual que hookUserPromptSubmit, Codex valida el sobre JSON del protocolo
	// en Stop; el texto plano solo corresponde a adaptadores que lo documenten.
	dialect := dialectClaude
	if v := emitFlagValue(args); isKnownDialect(v) {
		dialect = hookDialect(v)
	}

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

	// Importación de ADR (feature 010, Historia 2, sentido proveedor→
	// gomemory): mismo criterio best-effort/no-bloqueante que MaybeRefresh
	// arriba — timeout corto, cualquier error (proveedor caído, no
	// disponible) se ignora en silencio, nunca aborta el cierre del turno.
	if deps.ADRSyncProvider != nil && deps.ADRSyncRepo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		usecases.ImportADRs(ctx, deps.ADRSyncProvider, deps.ADRSyncRepo, deps.MemoryRepo, deps.Project)
		cancel()
	}

	// Recordatorio de compactación (feature 008): si la huella emitida por
	// gomemory en la sesión superó el umbral, sugiere compactar de forma NEUTRAL
	// (sin nombrar comandos de cliente) y no bloqueante. Va ANTES de
	// recordActivityCheckpoint (que consume stdin y hace os.Exit); computeCompactNudge
	// NO consume stdin, así el checkpoint sigue viendo el payload intacto.
	if root, err := deps.ProjectRepo.FindRoot(); err == nil {
		threshold := deps.SettingsRepo.Read(root).CompactThreshold
		if msg, ok := computeCompactNudge(root, threshold); ok {
			fmt.Print(renderTurnEnd(dialect, msg, true))
		} else if msg, ok := computePreferenceReinforcement(deps, root, deps.Project, threshold); ok {
			// Solo uno de los dos por turno: si ya se sugirió compactar, no
			// compite por espacio con el refuerzo de preferencias — la
			// compactación reinyecta el contexto completo de todos modos.
			//
			// Va en additionalContext, no en systemMessage: el mensaje está
			// dirigido al agente ("no las pierdas de vista"), y systemMessage
			// solo lo ve el humano en la terminal. Stop SÍ soporta
			// hookSpecificOutput.additionalContext sin bloquear el turno
			// (documentado como "non-error feedback that continues the
			// conversation"), a diferencia de decision:"block", que forzaría
			// una iteración extra no deseada aquí.
			fmt.Print(renderTurnEnd(dialect, msg, false))
		}
	}

	recordActivityCheckpoint(deps, "Checkpoint automático")
}

// hookSubagentStart corre al arrancar un subagente (tool Task) en Claude Code,
// ANTES de su primer prompt. Un subagente es un contexto nuevo y aislado: no
// pasa por hookSessionStart ni por la rama de "primer prompt" de
// hookUserPromptSubmit, así que sin este hook arrancaba sin el bootstrap de
// ToolSearch ni el recordatorio del protocolo — el agente principal delega
// exploración de código a un subagente (p. ej. tipo Explore) y este, al no
// saber que codebase-memory-mcp existe, recurre a grep/glob manuales. Mismo
// contenido que el primer prompt de la sesión principal, sin marker: cada
// invocación de un subagente es una sesión corta de un solo uso, no hace falta
// deduplicar entre turnos como si se repite en hookUserPromptSubmit.
func hookSubagentStart(deps *Deps) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fmt.Print("{}")
		os.Exit(0)
	}
	settings := deps.SettingsRepo.Read(root)
	bootstrap := buildMemoryToolBootstrap(!settings.CodeGraphDisabled, settings.OctopusEnabled)
	out := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "SubagentStart",
			"additionalContext": bootstrap + "\n\n" + memoryProtocolReminder,
		},
	}
	data, _ := json.Marshal(out)
	fmt.Print(string(data))
	os.Exit(0)
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

	// Cierra el episodio de plan (feature 019, data-model.md §2): aprobar
	// el plan reinicia el contador de devoluciones de plan-guard, sin
	// importar si el resto de este hook logra extraer texto o guardar
	// memoria — la aprobación en sí misma es la señal de cierre.
	planEpisodeReset(root)

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

// maxCommandChars acota CADA comando registrado en un checkpoint. El límite de
// 5 comandos por turno no bastaba: un solo comando puede traer un heredoc con
// un archivo entero. Sin este techo, la actividad automática llegó a ser el
// 78 % de la memoria del proyecto (426 de 508 entradas, con un checkpoint de
// 25 478 caracteres). El checkpoint es una red de seguridad de lo que pasó, no
// una transcripción de la terminal.
const maxCommandChars = 300

// acotaComando recorta un comando dejando constancia explícita de cuánto se
// omitió. No reusa domain.Extract a propósito: su heurística de "primera
// oración" está pensada para prosa y sobre un comando cortaría en cualquier
// punto seguido de espacio, produciendo un fragmento engañoso.
func acotaComando(cmd string) string {
	r := []rune(strings.TrimSpace(cmd))
	if len(r) <= maxCommandChars {
		return string(r)
	}
	corte := strings.TrimRight(string(r[:maxCommandChars]), " \n\t")
	return fmt.Sprintf("%s… (+%d caracteres omitidos)", corte, len(r)-maxCommandChars)
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
		acotados := make([]string, 0, len(cmds))
		for _, c := range cmds {
			acotados = append(acotados, acotaComando(c))
		}
		parts = append(parts, "Comandos: "+strings.Join(acotados, "; "))
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

// readHookStdinRaw lee el payload crudo de stdin sin exigir que sea JSON
// válido: a diferencia de readHookStdin, plan-guard y plan-entered aceptan
// texto plano como forma válida de plan (contracts/agent-integration.md,
// «Entrada»). Devuelve nil si no hay datos en pipe (ejecución manual en
// terminal), igual que readHookStdin.
func readHookStdinRaw() []byte {
	stat, err := os.Stdin.Stat()
	if err != nil || (stat.Mode()&os.ModeCharDevice) != 0 {
		return nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil
	}
	return data
}

// emitHookOutput escribe la salida ya traducida a un dialecto (hook_dialect.go)
// y termina el proceso con su código de salida. Punto único de salida para
// plan-guard y plan-entered, para que ningún camino olvide escribir a la
// corriente correcta.
func emitHookOutput(out hookRenderedOutput) {
	if out.stdout != "" {
		fmt.Print(out.stdout)
	}
	if out.stderr != "" {
		fmt.Fprint(os.Stderr, out.stderr)
	}
	os.Exit(out.exitCode)
}

// planGuardDenialReason redacta el motivo de una devolución de plan-guard:
// qué falta, qué hacer, y el aviso de que no se repetirá
// (contracts/hook-plan-guard.md, «Requisitos del motivo»).
func planGuardDenialReason() string {
	return "El plan no cumple el contrato de forma del proyecto: falta el árbol de tareas atómicas. " +
		"Cada hoja debe ser un verbo + objeto con un resultado verificable " +
		"(`[1.1] verbo + objeto → resultado`). Llama a get_plan_context() si necesitas el método y " +
		"el historial, y presenta el plan otra vez. Este aviso se emite una sola vez por plan."
}

// hookPlanGuard implementa `mem hook plan-guard`: el borde de salida
// determinista del modo plan atómico (feature 019, Historia 1). Evalúa la
// forma del plan y, si claramente no cumple el contrato de árbol, lo
// devuelve con el motivo — como máximo una vez por episodio (plan_episode.go),
// nunca sobre planes triviales (domain.EvaluatePlanShape), apagable
// (PlanGuardDisabled), y siempre sesgado a permitir ante la duda
// (contracts/hook-plan-guard.md). La salida se traduce al dialecto que
// corresponda (hook_dialect.go) — el motor de decisión de aquí no sabe nada
// de agentes concretos.
func hookPlanGuard(deps *Deps, args []string) {
	raw := readHookStdinRaw()
	payload, plan := parseHookPayload(raw)
	dialect := detectDialect(payload, emitFlagValue(args))

	permit := func() {
		emitHookOutput(renderGuardDecision(dialect, planGuardDecision{}))
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		permit()
	}
	if deps.SettingsRepo.Read(root).PlanGuardDisabled {
		permit()
	}
	if planEpisodeDenied(root) {
		permit()
	}
	if domain.EvaluatePlanShape(plan) != domain.ShapeMissing {
		permit()
	}

	planEpisodeMarkDenied(root)
	emitHookOutput(renderGuardDecision(dialect, planGuardDecision{deny: true, reason: planGuardDenialReason()}))
}

// defaultPlanEnteredBudget es el tope aplicado por defecto al documento de
// plan-entered: 9500 caracteres, con margen de 500 sobre el tope documentado
// de 10000 para salidas de hook (contracts/hook-plan-entered.md, «Presupuesto
// del canal»). Ajustable con --budget para canales con otro límite.
const defaultPlanEnteredBudget = 9500

// planEnteredShortReminder es lo que se emite en la segunda invocación de la
// misma sesión en adelante (FR-008): no repite el bloque completo, solo
// recuerda el método y cómo recuperarlo.
const planEnteredShortReminder = "Modo plan: aplica el método de descomposición atómica ya cargado esta sesión. " +
	"Si necesitas el método completo y el historial de nuevo, llama a get_plan_context()."

// hookPlanEntered implementa `mem hook plan-entered`: el borde de entrada al
// modo plan atómico (feature 019, Historia 2 — mejor esfuerzo). Pone a
// disposición del agente el método de descomposición y el historial del
// proyecto, ajustados al presupuesto del canal (domain.AdjustPlanDocumentToBudget),
// y reinicia el episodio de plan (entrar en modo plan abre un episodio
// nuevo). Respeta el mismo gate que `mem plan-context`/get_plan_context
// (AtomicPlanDisabled, feature 013): degradar en silencio ante cualquier
// circunstancia ambiental (FR-009).
// recordPlanEntryActivity deja rastro de que el canal plan_entry de Claude se
// ejerció, en paridad con lo que hace el complemento de OpenCode (feature 024,
// FR-009). Este hook solo lo registra Claude Code, así que aquí es la única
// puerta por donde ese canal demuestra vida. Se invoca antes de cualquier
// salida temprana: aunque el gate esté deshabilitado, el canal sí fue llamado.
// Es fire-and-forget: un rastro que abortara la entrada al plan sería peor que
// no tenerlo.
func recordPlanEntryActivity(deps *Deps) {
	if deps == nil || deps.ChannelActivity == nil {
		return
	}
	_ = deps.ChannelActivity.RecordFired("claude", "user", "plan_entry")
}

func hookPlanEntered(deps *Deps, args []string) {
	raw := readHookStdinRaw()
	payload, _ := parseHookPayload(raw)
	dialect := detectDialect(payload, emitFlagValue(args))

	silence := func() {
		emitHookOutput(renderEnteredDocument(dialect, ""))
	}

	recordPlanEntryActivity(deps)

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		silence()
		return
	}
	if deps.SettingsRepo.Read(root).AtomicPlanDisabled {
		silence()
		return
	}

	planEpisodeReset(root) // entrar en modo plan abre un episodio nuevo

	marker := planEnteredMarkerPath(deps, root)
	if _, statErr := os.Stat(marker); statErr == nil {
		// Ya se emitió el documento completo esta sesión (FR-008): un
		// recordatorio corto basta, sin volver a saturar la conversación.
		emitHookOutput(renderEnteredDocument(dialect, planEnteredShortReminder))
	}
	_ = os.MkdirAll(filepath.Dir(marker), 0o755)
	_ = os.WriteFile(marker, []byte("1"), 0o644)

	context, err := deps.ContextBuilder.Build()
	if err != nil {
		context = ""
	}

	budget := defaultPlanEnteredBudget
	if b := budgetFlagValue(args); b > 0 {
		budget = b
	}
	doc := domain.AdjustPlanDocumentToBudget(planMethod, context, budget)

	emitHookOutput(renderEnteredDocument(dialect, doc))
}

const compactionRecoveryInstructions = `**TRAS LA COMPACTACIÓN — PRIMERA ACCIÓN REQUERIDA**

1. Llama a end_session() con un resumen de en qué estábamos trabajando,
   qué se logró y los próximos pasos.
2. Llama a get_context() para recuperar el estado de la sesión previa.
3. Solo ENTONCES continúa trabajando.

No omitas el paso 1. Sin él, todo lo hecho antes de la compactación
se pierde de la memoria.`

// buildMemoryToolBootstrap fuerza la carga de las tools MCP de gomemory. En
// Claude Code las tools de un MCP server llegan DIFERIDAS: existen por nombre
// pero su esquema no está cargado, así que no pueden invocarse hasta ejecutar
// un ToolSearch que las materialice. Sin este empujón, el agente "sabe" que
// hay memoria (por el recordatorio de protocolo) pero no puede llamarla hasta
// que el usuario la menciona — exactamente lo que hacía sentir la memoria
// pasiva. El único mecanismo que Claude Code respeta para esto es un
// systemMessage con el select explícito. Se emite SOLO aquí porque este hook
// lo registra únicamente Claude Code; OpenCode carga las tools por su plugin y
// el resto de agentes por las instrucciones MCP nativas, así que no necesitan
// este bootstrap.
// La lista de gomemory se construye desde domain.MCPAllTools() en vez de
// llevarla escrita a mano: cuando estaba hardcodeada, la tool get_plan_context
// (feature 013) y las 5 del grafo de código quedaron fuera, así que el agente
// leía "llama a get_plan_context() al entrar en modo plan" y no podía hacerlo
// porque su esquema nunca se materializaba. Un test de contrato verifica que
// esta lista coincida con las tools que el servidor registra de verdad.
// MemoryToolBootstrap expone el bootstrap base (sin el proveedor externo de
// grafo) para el test de contrato que verifica que materialice TODAS las
// tools registradas por el servidor gomemory.
func MemoryToolBootstrap() string { return buildMemoryToolBootstrap(false, false) }

// gomemoryBootstrapToolNames son los nombres de gomemory ya prefijados, listos
// para el select: de ToolSearch. Depende del estado del módulo Octopus AAR
// (feature 027) porque su superficie MCP es condicional: registrar una tool sin
// materializarla aquí la deja INVOCABLE SOLO SOBRE EL PAPEL — es exactamente el
// bug de get_plan_context que este archivo documenta más arriba, y por eso se
// paga en el mismo cambio que introduce el registro condicional.
func gomemoryBootstrapToolNames(octopusEnabled bool) []string {
	return domain.MCPPrefixed("mcp__gomemory__", domain.MCPToolsFor(octopusEnabled))
}

// buildMemoryToolBootstrap arma el bootstrap de ToolSearch que fuerza la carga
// de las tools MCP diferidas de gomemory. Cuando includeCodeGraphProvider es
// true (el interruptor "Grafo de código externo" de la TUI está activo, mismo
// !settings.CodeGraphDisabled), añade al MISMO select: las tools de
// descubrimiento del proveedor externo (codebase-memory-mcp) — una sola
// llamada de ToolSearch materializa memoria + grafo propio + grafo externo, en
// vez de depender de que el agente recuerde por su cuenta el mensaje CRITICAL
// que ese proveedor inyecta por separado.
//
// Si el proveedor no está conectado, ToolSearch simplemente no encuentra esos
// nombres — degradación silenciosa, sin necesidad de detectar su
// disponibilidad de antemano (domain.CodebaseMemoryMCPDiscoveryTools).
func buildMemoryToolBootstrap(includeCodeGraphProvider, octopusEnabled bool) string {
	names := gomemoryBootstrapToolNames(octopusEnabled)
	if includeCodeGraphProvider {
		names = append(append([]string{}, names...),
			domain.MCPPrefixed(domain.CodebaseMemoryMCPPrefix, domain.CodebaseMemoryMCPDiscoveryTools)...)
	}
	return `PRIMERA ACCIÓN — ejecuta este ToolSearch AHORA, antes de responder al usuario, ` +
		`para cargar las tools de memoria del proyecto:
select:` + strings.Join(names, ",") + `
Luego llama a get_context() para cargar la memoria previa del proyecto antes de continuar.
Si entras en modo plan, llama además a get_plan_context() ANTES de redactar el plan.` +
		octopusDelegationPolicy(octopusEnabled, "mcp__gomemory__octopus_route_task")
}

// octopusDelegationReminder es la única fuente de la regla de consulta previa
// a delegar, sin separadores propios — cada llamador decide cómo unirla al
// resto. El runtime conserva la ejecución de subagentes; Octopus solo decide
// la ruta y el agente debe respetarla. routeTool cambia únicamente el prefijo
// que cada runtime usa para la misma tool MCP.
//
// El último párrafo (propagación manual) existe porque no hay ningún hook
// que reinyecte esta regla en un subagente de Codex: Codex no registra
// SubagentStart (a diferencia de Claude Code, que sí), y un subagente
// lanzado como `codex exec` no dispara NINGÚN hook, ni siquiera SessionStart
// (codex_setup.go). Sin esta instrucción, esos subagentes no tienen forma de
// enterarse de la regla — con ella, queda en manos del agente raíz seguirla
// (best-effort, no verificable por un hook; ACR 029, hallazgo C-001).
func octopusDelegationReminder(octopusEnabled bool, routeTool string) (string, bool) {
	if !octopusEnabled {
		return "", false
	}
	return `OCTOPUS AAR — REGLA OBLIGATORIA DE DELEGACIÓN: antes de crear cualquier subagente,
llama a ` + routeTool + ` con el objetivo, task_class, complejidad, riesgo, alcance
(files), dependencias y presupuesto/contexto conocidos; declara subagents=true si el
runtime los admite. Respeta su ruta: DELEGATE es la única autorización para crear el
subagente; INLINE se ejecuta aquí; WAIT espera las dependencias; REJECT no se ejecuta
y se informa la razón. Octopus recomienda política: no crea ni bloquea subagentes por sí
mismo.

Si el subagente que vas a crear no arranca su propia sesión con hooks de gomemory
(por ejemplo, un subproceso no interactivo tipo ` + "`codex exec`" + `, que no dispara
ningún hook), copia este párrafo completo, literal, en las instrucciones o el
prompt que le des: es la única forma de que reciba la regla, porque ningún
hook se la va a inyectar por su cuenta.`, true
}

// octopusDelegationPolicy envuelve octopusDelegationReminder con los
// separadores que necesita al concatenarse al final del bootstrap completo
// (buildMemoryToolBootstrap) o al imprimirse solo (hookOctopusDelegationPolicy).
func octopusDelegationPolicy(octopusEnabled bool, routeTool string) string {
	msg, ok := octopusDelegationReminder(octopusEnabled, routeTool)
	if !ok {
		return ""
	}
	return "\n\n" + msg
}

// hookOctopusDelegationPolicy permite que runtimes cuyo contexto se inyecta
// desde fuera de los hooks JSON de Go (OpenCode) consuman la misma política sin
// copiarla en otro lenguaje. Es de solo lectura y no emite nada con el módulo
// apagado, preservando su huella cero.
func hookOctopusDelegationPolicy(deps *Deps, args []string) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		return
	}
	routeTool := "mcp__gomemory__octopus_route_task"
	if firstOr(args, 0, "") == "opencode" {
		routeTool = "gomemory_octopus_route_task"
	}
	fmt.Print(octopusDelegationPolicy(deps.SettingsRepo.Read(root).OctopusEnabled, routeTool))
}

var memoryProtocolReminder = `Memoria persistente activa (gomemory). Guarda proactivamente con save_memory ` +
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
	`gomemory para memoria persistente (` + strings.Join(domain.MCPAllTools(), ", ") + `). El ` +
	`sistema de memoria nativo del harness (archivo MEMORY.md bajo ~/.claude/projects/.../memory/) ` +
	`NO aplica aquí — ignóralo por completo en este proyecto y no lo consultes ni escribas en él.

GRAFO DE CÓDIGO EXTERNO: si el servidor MCP codebase-memory-mcp está conectado, úsalo SIEMPRE ` +
	`para exploración de código en vez de leer archivos a mano: ` +
	strings.Join(domain.CodebaseMemoryMCPDiscoveryTools, ", ") + `. Para explorar el código usa las ` +
	`herramientas del grafo; para entregar un plan usa el árbol de tareas atómicas. Lo que descubras ` +
	`con el grafo alimenta las hojas del árbol. Si no está conectado, esta guía no aplica.`

// firstOr devuelve el argumento en la posición dada, o un valor por defecto.
func firstOr(args []string, i int, def string) string {
	if len(args) > i && strings.TrimSpace(args[i]) != "" {
		return args[i]
	}
	return def
}

// hookChannelActivity anota que un canal se ejerció, o que falló al intentarlo.
//
// Existe para que el complemento de OpenCode deje rastro: sus rutas de error
// absorbían el fallo en silencio, así que un cambio de interfaz del agente
// dejaba la inyección muerta sin que nada lo reportara (feature 024, FR-012).
//
// Uso: mem hook channel-fired <agente> <ámbito> <canal>
//
//	mem hook channel-error <agente> <ámbito> <canal> <mensaje>
//
// Es fire-and-forget: nunca falla ni escribe en stdout. Un rastro que
// interrumpiera el turno de quien trabaja sería peor que no tenerlo.
func hookChannelActivity(deps *Deps, args []string, errMsg string) {
	if deps.ChannelActivity == nil || len(args) < 3 {
		return
	}
	agente, ambito, canal := args[0], args[1], args[2]
	if errMsg != "" {
		deps.ChannelActivity.RecordError(agente, ambito, canal, errMsg)
		return
	}
	deps.ChannelActivity.RecordFired(agente, ambito, canal)
}
