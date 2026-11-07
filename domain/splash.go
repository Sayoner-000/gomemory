package domain

import "strings"

// SplashAgentType es el tipo de subagente revisor sin contexto (HCD-047): el
// «goldfish» del modelo Elephant-Goldfish con nombre propio de gomemory, la
// gota que salpica sin arrastrar memoria. Lee solo el documento y el código
// vivo; gomemory no le inyecta aviso, tools de memoria ni instrucciones.
const SplashAgentType = "splash"

// IsSplashAgent dice si el tipo o nombre de un subagente es splash, sin
// distinguir mayúsculas ni espacios laterales.
func IsSplashAgent(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), SplashAgentType)
}

// SplashCodexForkWarning es el aviso al agente raíz de Codex cuando lanza un
// splash que heredaría su conversación.
const SplashCodexForkWarning = `gomemory: este spawn_agent lanza un subagente splash sin fork_turns: "none". ` +
	`Heredará tu conversación y la memoria del proyecto, y deja de ser un revisor sin contexto. ` +
	`Lánzalo con spawn_agent(task_name: "splash", fork_turns: "none").`

// SplashSpawnInheritsContext dice si una llamada a spawn_agent de Codex lanza
// un splash que heredaría la conversación del raíz (HCD-047): en Codex el hijo
// no dispara hooks de arranque y solo fork_turns "none" lo aísla (sondeo real,
// Codex 0.166.0). Sin fork_turns, Codex hereda por defecto.
func SplashSpawnInheritsContext(tool string, input map[string]any) bool {
	if !strings.HasSuffix(tool, "spawn_agent") || input == nil {
		return false
	}
	agentType, _ := input["agent_type"].(string)
	taskName, _ := input["task_name"].(string)
	if !IsSplashAgent(agentType) && !IsSplashAgent(taskName) {
		return false
	}
	fork, _ := input["fork_turns"].(string)
	return !strings.EqualFold(strings.TrimSpace(fork), "none")
}
