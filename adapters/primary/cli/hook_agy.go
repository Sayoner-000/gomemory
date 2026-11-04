package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// agyHookBudget deja margen dentro del timeout que declara cada hook de agy.
const agyHookBudget = 8 * time.Second

// agyTranscriptMax acota la lectura del transcript en cada cierre de turno.
const agyTranscriptMax = 16 << 20

// agyToolNames traduce las herramientas de agy a la clasificación de expand.
var agyToolNames = map[string]string{
	"run_command":     "Bash",
	"write_file":      "Write",
	"edit_file":       "Edit",
	"invoke_subagent": "Agent",
}

type agyEvent struct {
	ConversationID string   `json:"conversationId"`
	WorkspacePaths []string `json:"workspacePaths"`
	TranscriptPath string   `json:"transcriptPath"`
	LastUserInput  string   `json:"lastUserInput"`
	InvocationNum  int      `json:"invocationNum"`
	FullyIdle      bool     `json:"fullyIdle"`
	ToolCall       struct {
		Name string `json:"name"`
		Args struct {
			CommandLine string `json:"CommandLine"`
			Path        string `json:"Path"`
			Content     string `json:"Content"`
			Replacement string `json:"Replacement"`
		} `json:"args"`
	} `json:"toolCall"`
}

// agyStep es la parte usada de una línea de transcript_full.jsonl.
type agyStep struct {
	Type      string `json:"type"`
	Error     string `json:"error"`
	ToolCalls []struct {
		Name string `json:"name"`
		Args struct {
			CommandLine string `json:"CommandLine"`
			Path        string `json:"Path"`
		} `json:"args"`
	} `json:"tool_calls"`
}

// agyCall es una invocación de `mem hook` con su payload neutral.
type agyCall struct {
	args  []string
	stdin []byte
}

type agyPlan struct {
	dir   string
	calls []agyCall
}

// agyRunner ejecuta una llamada en el proyecto dir y devuelve su stdout.
type agyRunner func(dir string, call agyCall) (string, error)

func agyHookCall(payload map[string]any, args ...string) agyCall {
	stdin, _ := json.Marshal(payload)
	return agyCall{args: append(args, "--emit="+string(dialectAgy)), stdin: stdin}
}

// planAgyHook traduce un evento de agy a las llamadas neutrales de `mem hook`.
// Un evento ilegible o desconocido no produce ninguna llamada.
func planAgyHook(event string, raw []byte) agyPlan {
	var ev agyEvent
	if json.Unmarshal(raw, &ev) != nil {
		return agyPlan{}
	}
	plan := agyPlan{}
	if len(ev.WorkspacePaths) > 0 {
		plan.dir = ev.WorkspacePaths[0]
	}
	session := map[string]any{"session_id": ev.ConversationID}
	switch event {
	case "SessionStart":
		plan.calls = []agyCall{agyHookCall(session, "session-start"), agyHookCall(session, "expand", "session-start")}
	case "PreInvocation":
		// agy lo repite en cada llamada al modelo; el prompt solo llega una vez.
		if ev.InvocationNum == 0 {
			session["prompt"] = ev.LastUserInput
			plan.calls = []agyCall{agyHookCall(session, "user-prompt-submit"), agyHookCall(session, "expand", "prompt")}
		}
	case "PreToolUse", "PostToolUse":
		plan.calls = []agyCall{agyHookCall(agyToolPayload(event, ev), "expand", agyExpandToolEvent(event))}
	case "Stop":
		if ev.FullyIdle {
			// turn-end interpreta transcript_path con el formato de Claude; aquí
			// recibe ya la actividad, como el plugin de OpenCode.
			session["files"], session["commands"] = agyTurnActivity(ev.TranscriptPath)
			plan.calls = []agyCall{agyHookCall(session, "turn-end"), agyHookCall(session, "expand", "stop")}
		}
	}
	return plan
}

func cleanAgyArg(s string) string {
	return strings.Trim(strings.TrimSpace(s), "\"")
}

// agyTurnActivity extrae del transcript los archivos y comandos del último
// turno. Una llamada denegada o fallida no cuenta como actividad realizada.
func agyTurnActivity(path string) (files, commands []string) {
	files, commands = []string{}, []string{}
	file, err := os.Open(path)
	if err != nil {
		return files, commands
	}
	defer func() { _ = file.Close() }()
	var pending *agyStep
	apply := func() {
		if pending == nil {
			return
		}
		for _, call := range pending.ToolCalls {
			switch {
			case call.Name == "run_command" && call.Args.CommandLine != "":
				if cmd := cleanAgyArg(call.Args.CommandLine); cmd != "" {
					commands = append(commands, cmd)
				}
			case call.Args.Path != "":
				if p := cleanAgyArg(call.Args.Path); p != "" {
					files = append(files, p)
				}
			}
		}
		pending = nil
	}
	decoder := json.NewDecoder(io.LimitReader(file, agyTranscriptMax))
	for {
		var step agyStep
		if decoder.Decode(&step) != nil {
			apply()
			return files, commands
		}
		switch {
		case step.Type == "USER_INPUT":
			pending, files, commands = nil, []string{}, []string{}
		case len(step.ToolCalls) > 0:
			apply()
			current := step
			pending = &current
		case step.Error != "":
			pending = nil
		default:
			apply()
		}
	}
}

func agyExpandToolEvent(event string) string {
	if event == "PreToolUse" {
		return "pre-tool"
	}
	return "post-tool"
}

func agyToolPayload(event string, ev agyEvent) map[string]any {
	tool, ok := agyToolNames[ev.ToolCall.Name]
	if !ok {
		tool = ev.ToolCall.Name
	}
	content := ev.ToolCall.Args.Content
	if content == "" {
		content = ev.ToolCall.Args.Replacement
	}
	return map[string]any{
		"hook_event_name": event,
		"session_id":      ev.ConversationID,
		"tool_name":       tool,
		"tool_input": map[string]any{
			"command":   cleanAgyArg(ev.ToolCall.Args.CommandLine),
			"file_path": cleanAgyArg(ev.ToolCall.Args.Path),
			"content":   content,
		},
	}
}

// agyDefaultResponse es la respuesta que nunca bloquea: PreToolUse exige
// `decision`, y `allow` conserva la confirmación de permisos del host.
func agyDefaultResponse(event string) []byte {
	if event == "PreToolUse" {
		return []byte(`{"decision":"allow"}`)
	}
	return []byte(`{}`)
}

// agyHookResponse ejecuta el plan y traduce el resultado. Regla de oro de los
// hooks: ante cualquier fallo, responde sin bloquear al agente.
func agyHookResponse(event string, raw []byte, run agyRunner) (response []byte) {
	response = agyDefaultResponse(event)
	defer func() {
		if recover() != nil {
			response = agyDefaultResponse(event)
		}
	}()
	plan := planAgyHook(event, raw)
	var texts []string
	for _, call := range plan.calls {
		out, err := run(plan.dir, call)
		if err != nil {
			continue
		}
		if text := agyPlainText(out); text != "" {
			texts = append(texts, text)
		}
	}
	return renderAgyResponse(event, texts)
}

// agyPlainText quita el sobre JSON que el primer prompt emite en cualquier
// dialecto: agy solo debe recibir el contexto.
func agyPlainText(out string) string {
	text := strings.TrimSpace(out)
	var envelope struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if strings.HasPrefix(text, "{") && json.Unmarshal([]byte(text), &envelope) == nil {
		return strings.TrimSpace(envelope.HookSpecificOutput.AdditionalContext)
	}
	return text
}

func renderAgyResponse(event string, texts []string) []byte {
	switch event {
	case "PreToolUse":
		for _, text := range texts {
			if reason, blocked := strings.CutPrefix(text, "BLOCK: "); blocked {
				out, _ := json.Marshal(map[string]string{"decision": "deny", "reason": reason})
				return out
			}
		}
	case "SessionStart", "PreInvocation":
		if len(texts) == 0 {
			break
		}
		text := strings.Join(texts, "\n\n")
		// En SessionStart, agy 1.4.12 ignora systemMessage si es una cadena.
		step := map[string]any{"ephemeralMessage": text}
		if event == "SessionStart" {
			step = map[string]any{"systemMessage": map[string]string{"systemMessage": text}}
		}
		out, err := json.Marshal(map[string]any{"injectSteps": []any{step}})
		if err == nil {
			return out
		}
	}
	return agyDefaultResponse(event)
}

// runAgyCall relanza este binario en el proyecto: los manejadores de hook
// resuelven la raíz desde el directorio de trabajo y terminan el proceso.
func runAgyCall(ctx context.Context, binary, dir string, call agyCall) (string, error) {
	cmd := exec.CommandContext(ctx, binary, append([]string{"hook"}, call.args...)...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(call.stdin)
	out, err := cmd.Output()
	return string(out), err
}

// hookAgy implementa `mem hook agy <evento>`.
func hookAgy(args []string) {
	if len(args) == 0 {
		os.Exit(0)
	}
	event := args[0]
	raw, _ := io.ReadAll(io.LimitReader(os.Stdin, expandStdinMax))
	binary, err := os.Executable()
	if err != nil {
		_, _ = os.Stdout.Write(agyDefaultResponse(event))
		os.Exit(0)
	}
	ctx, cancel := context.WithTimeout(context.Background(), agyHookBudget)
	defer cancel()
	_, _ = os.Stdout.Write(agyHookResponse(event, raw, func(dir string, call agyCall) (string, error) {
		return runAgyCall(ctx, binary, dir, call)
	}))
}
