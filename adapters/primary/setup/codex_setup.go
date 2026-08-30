package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// codexHook declara un enganche del ciclo de vida de gomemory en Codex: el
// evento de su config.toml, el filtro de orígenes y el subcomando `mem hook`
// que lo implementa.
//
// Es el equivalente de claudeHookEvents para el dialecto de Codex. Se limita a
// los eventos que Codex expone de verdad: declarar uno que nunca dispara
// produciría un fallo silencioso, justo lo que el registro de capacidades
// existe para impedir.
//
// «De verdad» significa comprobado en una sesión interactiva, no leído en la
// documentación ni deducido del binario. Esta tabla dijo durante varias
// versiones que Codex solo exponía SessionStart y Stop, y era falso:
// UserPromptSubmit dispara y su stdout llega al modelo como contexto. Nadie lo
// notó porque una capacidad declarada ausente no la vuelve a mirar nadie.
//
// `codex exec` NO ejecuta hooks —ninguno, tampoco SessionStart—, así que
// cualquier verificación futura de esta tabla exige sesión interactiva.
type CodexHook struct {
	Event   string
	Matcher string
	Sub     string
	// Emit fija el dialecto de salida del subcomando. Vacío deja el de por
	// defecto (JSON de Claude Code). Codex toma el stdout del hook como
	// contexto tal cual, así que los hooks que inyectan texto al modelo deben
	// pedir el dialecto plano de forma explícita.
	Emit string
}

// codexGomemoryHooks es el ciclo mínimo que hace que Codex ejerza memoria y no
// solo pueda consultarla: contexto al arrancar, recuperación tras compactar, y
// registro determinista de la actividad al cerrar cada turno.
var codexGomemoryHooks = []CodexHook{
	{Event: "SessionStart", Matcher: "startup|resume|clear", Sub: "session-start"},
	{Event: "SessionStart", Matcher: "compact", Sub: "post-compact"},
	{Event: "Stop", Sub: "turn-end", Emit: "text"},
	// Inyección por turno. Sin ella, Codex recibía el protocolo una sola vez al
	// arrancar, en un archivo estático que compite con todo el contexto y se
	// diluye según crece la conversación — la diferencia observable entre "el
	// agente sigue el método" y "lo siguió al principio".
	{Event: "UserPromptSubmit", Sub: "user-prompt-submit", Emit: "text"},
}

// CodexGomemoryHooks expone la tabla para quien tenga que escribirla. Vive en
// este paquete —el dueño de los artefactos de cada agente— y no en el
// instalador, para que el diagnóstico y la instalación lean la MISMA
// declaración: un hook que se instala y no se observa es el punto ciego que
// esta feature cierra.
func CodexGomemoryHooks() []CodexHook { return codexGomemoryHooks }

// CodexConfigPath resuelve ~/.codex/config.toml.
func CodexConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "config.toml"), nil
}

// CodexGlobalRegistrationExists reporta si config.toml registra el servidor
// gomemory bajo [mcp_servers.gomemory].
func CodexGlobalRegistrationExists() bool {
	doc, err := readCodexConfig()
	if err != nil {
		return false
	}
	servers, _ := doc["mcp_servers"].(map[string]any)
	_, existe := servers["gomemory"]
	return existe
}

// SostieneEntradaDePlan reconoce, dentro de la tabla, el hook por el que viaja
// la entrada al modo plan de Codex.
//
// El agente no expone un evento de cambio de modo —por eso no sostiene el nivel
// 1—, así que el nivel 2 lo sostiene la inyección por turno de UserPromptSubmit
// (verificado en sesión interactiva, Codex 0.151.0). Se declara aquí, junto a
// la tabla, para que el diagnóstico compruebe EL MISMO artefacto que el
// instalador escribe: mientras esta correspondencia vivió solo en un comentario,
// el inspector no supo qué mirar y reportó el canal como desconocido.
func (h CodexHook) SostieneEntradaDePlan() bool { return h.Event == "UserPromptSubmit" }

// CodexPlanEntryHook devuelve el hook que sostiene la entrada al modo plan.
// ok=false significa que la tabla dejó de declararlo: el diagnóstico lo reporta
// como canal roto antes que afirmar una capacidad que nadie comprobó.
func CodexPlanEntryHook() (CodexHook, bool) {
	for _, h := range codexGomemoryHooks {
		if h.SostieneEntradaDePlan() {
			return h, true
		}
	}
	return CodexHook{}, false
}

// CodexHookInstalado reporta si config.toml tiene enganchado ese hook concreto.
func CodexHookInstalado(h CodexHook) bool {
	doc, err := readCodexConfig()
	if err != nil {
		return false
	}
	hooks, _ := doc["hooks"].(map[string]any)
	return CodexHookPresente(hooks, h)
}

// CodexMissingGomemoryHooks devuelve los subcomandos de gomemory que NO están
// enganchados en config.toml. Lista vacía = el ciclo está completo.
func CodexMissingGomemoryHooks() []string { return codexMissingHooks(nil) }

// CodexMissingLifecycleHooks restringe el diagnóstico al ciclo de sesión.
//
// Deja fuera el hook de entrada al modo plan porque ese artefacto tiene canal
// propio (plan_entry): contarlo en ambos haría que una sola ausencia se
// reportara dos veces, con dos remedios, como si fueran dos faltas.
func CodexMissingLifecycleHooks() []string {
	return codexMissingHooks(func(h CodexHook) bool { return !h.SostieneEntradaDePlan() })
}

// CodexLifecycleHookSubs enumera los subcomandos del ciclo de sesión. Existe
// para que el informe describa lo que la tabla declara HOY: la enumeración
// escrita a mano en el detalle del canal ya se había quedado corta al añadirse
// un hook, y una lista que miente no avisa de nada.
func CodexLifecycleHookSubs() []string {
	return codexHookSubs(func(h CodexHook) bool { return !h.SostieneEntradaDePlan() })
}

// codexMissingHooks recorre la tabla —filtrada por incluir, nil = todos— y
// devuelve los subcomandos ausentes de config.toml.
func codexMissingHooks(incluir func(CodexHook) bool) []string {
	doc, err := readCodexConfig()
	if err != nil {
		// Sin config legible no se puede afirmar que los hooks estén: se
		// reportan todos como faltantes antes que declarar un ciclo sano que
		// nadie comprobó.
		return codexHookSubs(incluir)
	}
	hooks, _ := doc["hooks"].(map[string]any)
	var faltan []string
	for _, h := range codexGomemoryHooks {
		if incluir != nil && !incluir(h) {
			continue
		}
		if !CodexHookPresente(hooks, h) {
			faltan = append(faltan, h.Sub)
		}
	}
	return faltan
}

func codexHookSubs(incluir func(CodexHook) bool) []string {
	out := make([]string, 0, len(codexGomemoryHooks))
	for _, h := range codexGomemoryHooks {
		if incluir != nil && !incluir(h) {
			continue
		}
		out = append(out, h.Sub)
	}
	return out
}

// CodexHookGroup arma el grupo TOML que Codex espera para un enganche.
// CodexHookCommand es la forma canónica del comando de un hook: el subcomando y,
// solo si la tabla lo declara, el dialecto de salida.
//
// Vive aparte porque tiene DOS consumidores —el que instala un hook nuevo y el
// que reconcilia uno ya instalado— y tenerla escrita dos veces era justo lo que
// permitía que divergieran: una sabía añadir el --emit y la otra no sabía
// retirarlo.
func CodexHookCommand(h CodexHook, memCommand string) string {
	comando := fmt.Sprintf("%s hook %s", memCommand, h.Sub)
	if h.Emit != "" {
		comando += " --emit=" + h.Emit
	}
	return comando
}

func CodexHookGroup(h CodexHook, memCommand string) map[string]any {
	group := map[string]any{
		"hooks": []any{map[string]any{
			"type":    "command",
			"command": CodexHookCommand(h, memCommand),
		}},
	}
	if h.Matcher != "" {
		group["matcher"] = h.Matcher
	}
	return group
}

// CodexHookPresente reconoce un hook de gomemory por el subcomando y el
// dialecto de salida que invoca, no por la ruta del binario: quien instaló con
// `mem` y quien lo hizo con una ruta absoluta tienen el mismo ciclo enganchado,
// y reinstalar no debe duplicarlo. El dialecto también es parte del contrato:
// un hook que inyecta texto sin --emit=text sigue siendo una instalación
// incompleta aunque el subcomando coincida.
func CodexHookPresente(hooks map[string]any, h CodexHook) bool {
	grupos, _ := hooks[h.Event].([]any)
	for _, g := range grupos {
		grupo, ok := g.(map[string]any)
		if !ok {
			continue
		}
		matcher, _ := grupo["matcher"].(string)
		if matcher != h.Matcher {
			continue
		}
		acciones, _ := grupo["hooks"].([]any)
		for _, a := range acciones {
			accion, ok := a.(map[string]any)
			if !ok {
				continue
			}
			cmd, _ := accion["command"].(string)
			if !strings.Contains(cmd, "hook "+h.Sub) {
				continue
			}
			if h.Emit == "" && !strings.Contains(cmd, "--emit") {
				return true
			}
			if h.Emit != "" && codexHookEmits(cmd, h.Emit) {
				return true
			}
		}
	}
	return false
}

func codexHookEmits(command, emit string) bool {
	fields := strings.Fields(command)
	for i, field := range fields {
		if field == "--emit="+emit || field == "--emit" && i+1 < len(fields) && fields[i+1] == emit {
			return true
		}
	}
	return false
}

func readCodexConfig() (map[string]any, error) {
	path, err := CodexConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc := map[string]any{}
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}
