package hcd

import (
	"encoding/json"
	"fmt"
	"io"
	"mem/adapters/primary/cli/clikit"
	"os"
)

const usage = "Uso: mem hcd <start|list|show|resume|status|next|define|validate|clarify|approve|slice|tasks|check|run|report|verify|close|cancel|decision|block|memory|classify|focus|heartbeat|exception|playbook|migrate|subject> [unit] [flags] [--json] [--client CLIENT]\n"

func Run(deps *clikit.Deps, args []string) int { return run(deps, args, os.Stdout, os.Stderr) }

// stdin es la entrada de `--file -`; las pruebas la sustituyen.
var stdin io.Reader = os.Stdin

// readFileFlag lee --file desde disco, o desde stdin con «-». Con stdin el
// agente pasa el contenido en un heredoc y no necesita un archivo temporal,
// que los sandboxes de Codex y OpenCode suelen bloquear fuera del proyecto.
func readFileFlag(path string) ([]byte, error) {
	if path == "-" {
		return clikit.ReadBounded(stdin, 4<<20)
	}
	return os.ReadFile(path)
}

func run(deps *clikit.Deps, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	return execute(deps, args, stdout, stderr)
}

func emitError(stdout, stderr io.Writer, jsonMode bool, code, message string) int {
	if jsonMode {
		_ = emitJSON(stdout, Response{ProtocolVersion: 1, OK: false, Error: &Error{Code: code, Message: message}, AllowedActions: []string{}, PendingChecks: []PendingCheck{}, Blockers: []string{}, Data: map[string]any{}})
		return ExitCode(code)
	}
	_, _ = fmt.Fprintln(stderr, message)
	return ExitCode(code)
}

func emitJSON(w io.Writer, response Response) error {
	if response.AllowedActions == nil {
		response.AllowedActions = []string{}
	}
	if response.PendingChecks == nil {
		response.PendingChecks = []PendingCheck{}
	}
	if response.Blockers == nil {
		response.Blockers = []string{}
	}
	if response.Data == nil {
		response.Data = map[string]any{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(response)
}
