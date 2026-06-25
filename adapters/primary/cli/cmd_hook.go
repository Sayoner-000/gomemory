package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	default:
		// Evento desconocido: salida vacía, sin error.
		os.Exit(0)
	}
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

	marker := filepath.Join(root, deps.ProjectRepo.MemDir(), ".session-tools-injected")
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
	`usuario lo pida. Antes de cerrar, llama a end_session(summary).

IMPORTANTE — no confundir sistemas: este proyecto usa EXCLUSIVAMENTE las tools MCP de ` +
	`gomemory (save_memory, search_memories, get_memory, list_memories, start_session, ` +
	`end_session, get_context). El sistema de memoria nativo del harness (archivo MEMORY.md ` +
	`bajo ~/.claude/projects/.../memory/) NO aplica aquí — ignóralo por completo en este ` +
	`proyecto y no lo consultes ni escribas en él.`
