package domain

// GomemoryHookSubcommands es el registro ÚNICO de subcomandos de `mem hook`
// (feature 040, research R2). El instalador, el desinstalador y la
// deduplicación reconocen una entrada de hook como de gomemory consultando
// esta lista, en vez de enumerar los subcomandos a mano en cada sitio: un
// subcomando nuevo que no figure aquí dejaría hooks huérfanos tras
// `mem uninstall`, y tests/contract/hook_subcommands_registry_test.go lo
// detecta recorriendo lo que instalan los tres harnesses.
//
// pre-compact ya no se instala (lo sustituyó SessionStart(compact)), pero un
// settings.json antiguo puede contenerlo y el desinstalador debe reconocerlo.
var GomemoryHookSubcommands = []string{
	"session-start",
	"session-end",
	"pre-compact",
	"post-compact",
	"compaction-context",
	"compact-summary",
	"user-prompt-submit",
	"turn-end",
	"subagent-start",
	"subagent-stop",
	"plan-approved",
	"plan-guard",
	"plan-entered",
	"tool-output",
	"prompt",
	"socratica-decide",
	"nudge",
	"agent-notice",
	"octopus-delegation-policy",
	"channel-fired",
	"channel-error",
	"expand",
}

// IsGomemoryHookSubcommand reporta si sub es un subcomando registrado de
// `mem hook`.
func IsGomemoryHookSubcommand(sub string) bool {
	for _, s := range GomemoryHookSubcommands {
		if s == sub {
			return true
		}
	}
	return false
}
