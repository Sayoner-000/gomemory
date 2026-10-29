package hcd

import (
	"encoding/json"
	"fmt"
	"io"
	"mem/adapters/primary/cli/clikit"
	"os"
)

const usage = `Uso: mem hcd <comando> [unidad] [flags] [--json] [--client CLIENTE]
  start   --kind fix|feature|evolution|technical --lane fast|full --title T --why MOTIVO [--intent I]
  show    [unidad]                 paso, tareas y siguiente skill (no escribe)
  next    [unidad]                 alias de show
  clarify [unidad] --ask 'pregunta' | --id Qn --answer 'respuesta'
  approve [unidad] --quote 'cita'  aprueba la revisión de la definición (solo carril full)
  verify  [unidad] --task N[,M] [--expect-fail] -- 'comando'
  verify  [unidad] --task N --manual 'observación'
  analyze [unidad]                 coherencia de la unidad (no escribe)
  close   [unidad] --quote 'cita del Implementador'
  cancel  [unidad] --quote 'motivo'
  promote [unidad]                 de la Vía rápida a la ceremonia completa
`

func Run(deps *clikit.Deps, args []string) int { return run(deps, args, os.Stdout, os.Stderr) }

func run(deps *clikit.Deps, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	return execute(deps, args, stdout, stderr)
}

func emitError(stdout, stderr io.Writer, jsonMode bool, code, message string) int {
	if jsonMode {
		_ = emitJSON(stdout, Response{ProtocolVersion: ProtocolVersion, OK: false, Error: &Error{Code: code, Message: message}})
		return ExitCode(code)
	}
	_, _ = fmt.Fprintln(stderr, message)
	return ExitCode(code)
}

func emitJSON(w io.Writer, response Response) error {
	if response.Tasks == nil {
		response.Tasks = []Task{}
	}
	if response.PendingChecks == nil {
		response.PendingChecks = []PendingCheck{}
	}
	if response.Findings == nil {
		response.Findings = []Finding{}
	}
	if response.RedMissing == nil {
		response.RedMissing = []string{}
	}
	if response.Warnings == nil {
		response.Warnings = []string{}
	}
	if response.Data == nil {
		response.Data = map[string]any{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(response)
}
