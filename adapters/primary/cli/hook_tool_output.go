package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/domain"
)

// Hook de salidas de herramientas (feature 033, US3, contracts/hook-tool-output.md).
//
// Comprime la salida de una herramienta del agente antes de que la vea el
// modelo, en los runtimes cuyo sistema de ganchos lo permite (verificado contra
// los binarios: Claude Code 2.1.282 con updatedToolOutput para todas las
// herramientas; OpenCode con output.output mutable en tool.execute.after).
// Codex NO lo permite: su parser de hooks rechaza siempre updatedMCPToolOutput
// («PostToolUse hook returned unsupported updatedMCPToolOutput»,
// codex-rs/hooks/src/engine/output_parser.rs), así que con Codex no se emite nada.
//
// Invariantes:
//   - H1: ante cualquier duda, salida vacía y código 0 (el runtime usa la
//     salida original). Nunca rompe la herramienta.
//   - H2: misma forma. Solo se sustituyen valores de texto dentro de
//     tool_response; ni claves ni tipos cambian.
//   - H3: Read, Edit, Write, MultiEdit, NotebookEdit y las herramientas de
//     gomemory nunca se tocan (Edit necesita el texto exacto que devolvió Read).
//   - H4: lo privado no genera originales (lo garantiza el motor).

// toolOutputExcluded: herramientas cuya salida nunca se reescribe.
var toolOutputExcluded = map[string]bool{
	"Read": true, "Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true,
	// OpenCode
	"read": true, "edit": true, "write": true, "patch": true, "multiedit": true,
}

func toolOutputIsExcluded(name string) bool {
	if toolOutputExcluded[name] || name == "mcp__codebase-memory-mcp__get_code_snippet" {
		return true
	}
	return strings.HasPrefix(name, "mcp__gomemory__") || strings.HasPrefix(name, "gomemory_")
}

// readCommandPrefixes son los comandos de shell cuya salida es una lectura
// exacta o un diagnóstico (feature 035, FR-013): el agente la pidió literal,
// y comprimirla le entregó código sin indentación y diagnósticos incompletos.
var readCommandPrefixes = []string{"sed ", "cat ", "head ", "tail ", "git diff", "git show", "mem doctor"}

func isReadCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	for _, p := range readCommandPrefixes {
		if strings.HasPrefix(cmd, p) {
			return true
		}
	}
	return false
}

// toolOutputExclusion dice si la salida descrita por payload nunca se
// reescribe, y con qué nombre se registra la exclusión.
func toolOutputExclusion(runtime string, payload []byte) (string, bool) {
	var ev struct {
		ToolName  string `json:"tool_name"`
		Tool      string `json:"tool"`
		Command   string `json:"command"`
		ToolInput struct {
			Command any `json:"command"`
		} `json:"tool_input"`
	}
	if json.Unmarshal(payload, &ev) != nil {
		return "", false
	}
	name, cmd := ev.ToolName, ev.ToolInput.Command
	if runtime == "opencode" {
		name, cmd = ev.Tool, ev.Command
	}
	if toolOutputIsExcluded(name) {
		return name, true
	}
	if c, ok := cmd.(string); ok && isReadCommand(c) {
		return name + ": " + strings.Fields(c)[0], true
	}
	return "", false
}

// textFields son las claves cuyo valor string se puede comprimir.
var textFields = map[string]bool{"stdout": true, "stderr": true, "output": true, "content": true, "text": true}

// compressFunc comprime un texto y dice si hubo compresión recuperable.
type compressFunc func(string) (string, bool)

// rewriteToolResponse recorre v y comprime los strings de textFields (también
// dentro de arrays de bloques {type,text}). Devuelve el valor con la MISMA
// forma y si cambió algo. No recorre objetos anidados arbitrarios: solo el
// primer nivel y los bloques de contenido, que es donde viven las salidas.
func rewriteToolResponse(v any, compress compressFunc) (any, bool) {
	changed := false
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, fv := range x {
			out[k] = fv
			if !textFields[k] {
				continue
			}
			switch f := fv.(type) {
			case string:
				if c, ok := compress(f); ok {
					out[k] = c
					changed = true
				}
			case []any:
				if nv, ok := rewriteBlocks(f, compress); ok {
					out[k] = nv
					changed = true
				}
			}
		}
		return out, changed
	case []any:
		return rewriteBlocks(x, compress)
	}
	return v, false
}

// rewriteBlocks trata un array de bloques de contenido MCP ({type,text}).
func rewriteBlocks(blocks []any, compress compressFunc) (any, bool) {
	out := make([]any, len(blocks))
	changed := false
	for i, b := range blocks {
		out[i] = b
		m, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if t, ok := m["text"].(string); ok {
			if c, ok := compress(t); ok {
				nm := make(map[string]any, len(m))
				for k, v := range m {
					nm[k] = v
				}
				nm["text"] = c
				out[i] = nm
				changed = true
			}
		}
	}
	return out, changed
}

// rewriteToolOutput es la parte pura del hook: dado el runtime y el evento,
// devuelve lo que hay que escribir en stdout (nil = nada, H1).
func rewriteToolOutput(runtime string, payload []byte, compress compressFunc) []byte {
	if _, excluded := toolOutputExclusion(runtime, payload); excluded {
		return nil
	}
	switch runtime {
	case "opencode":
		var ev struct {
			Tool   string `json:"tool"`
			Output string `json:"output"`
		}
		if json.Unmarshal(payload, &ev) != nil || toolOutputIsExcluded(ev.Tool) {
			return nil
		}
		c, ok := compress(ev.Output)
		if !ok {
			return nil
		}
		out, _ := json.Marshal(map[string]string{"output": c})
		return out
	case "claude":
		var ev struct {
			ToolName     string `json:"tool_name"`
			ToolResponse any    `json:"tool_response"`
		}
		if json.Unmarshal(payload, &ev) != nil || ev.ToolResponse == nil || toolOutputIsExcluded(ev.ToolName) {
			return nil
		}
		nv, changed := rewriteToolResponse(ev.ToolResponse, compress)
		if !changed {
			return nil
		}
		out, _ := json.Marshal(map[string]any{
			"hookSpecificOutput": map[string]any{"hookEventName": "PostToolUse", "updatedToolOutput": nv},
		})
		return out
	}
	// "codex" (y cualquier otro): Codex rechaza updatedMCPToolOutput y marca el
	// hook como fallido en cada herramienta MCP, así que no se emite nada (H1).
	return nil
}

// withBudget ejecuta fn con un plazo. Si vence, devuelve nil (H1): la
// compresión puede seguir en segundo plano, pero su resultado se descarta.
func withBudget(d time.Duration, fn func() []byte) []byte {
	ch := make(chan []byte, 1)
	go func() {
		defer func() {
			if recover() != nil {
				ch <- nil
			}
		}()
		ch <- fn()
	}()
	select {
	case r := <-ch:
		return r
	case <-time.After(d):
		return nil
	}
}

// hookToolOutput implementa `mem hook tool-output <claude|codex|opencode>` y
// `mem hook tool-output --enabled`.
func hookToolOutput(deps *Deps, args []string) {
	settings := ports.SettingsData{}
	if deps.SettingsRepo != nil {
		if root, err := deps.ProjectRepo.FindRoot(); err == nil {
			settings = deps.SettingsRepo.Read(root)
		}
	}
	if len(args) > 0 && args[0] == "--enabled" {
		fmt.Println(settings.ToolOutputCompression)
		os.Exit(0)
	}
	if !settings.ToolOutputCompression || len(args) == 0 || deps.Compressor == nil {
		os.Exit(0)
	}
	payload, err := io.ReadAll(io.LimitReader(os.Stdin, int64(domain.CompressionMaxInputBytes)*2))
	if err != nil {
		os.Exit(0)
	}
	root, _ := deps.ProjectRepo.FindRoot()
	if name, excluded := toolOutputExclusion(args[0], payload); excluded {
		recordGuard(deps, args[0], domain.GuardToolOutputExcluded, name)
		os.Exit(0)
	}
	minTokens := domain.ToolOutputMinTokens
	compress := func(s string) (string, bool) {
		if (len([]rune(s))+3)/4 < minTokens {
			return "", false
		}
		res, err := deps.Compressor.Compress(s, ports.CompressionOptions{Level: ports.CompressionMax, PreserveCode: true,
			PreserveURLs: true, PreservePaths: true, PreserveErrors: true, ToolOutput: true})
		if err != nil || len(res.Refs) == 0 {
			return "", false
		}
		return res.Content + retrieveHintOnce(root), true
	}
	out := withBudget(domain.HookBudget, func() []byte { return rewriteToolOutput(args[0], payload, compress) })
	if len(out) > 0 {
		_, _ = os.Stdout.Write(out)
	}
	os.Exit(0)
}

func markersNotePath(root string) string {
	return filepath.Join(root, persistence.MemDir, ".markers-note-emitted")
}

// retrieveHintOnce devuelve la nota de cómo recuperar lo omitido solo la
// primera vez en la conversación (feature 035, FR-015): repetirla en cada
// salida comprimida añadía el mismo párrafo una y otra vez al historial.
func retrieveHintOnce(root string) string {
	if root == "" {
		return RetrieveHint
	}
	if _, err := os.Stat(markersNotePath(root)); err == nil {
		return ""
	}
	_ = writeHookMarker(markersNotePath(root))
	return RetrieveHint
}
