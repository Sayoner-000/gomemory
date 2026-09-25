package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// Hook de salidas de herramientas (feature 033, US3, contracts/hook-tool-output.md).
//
// Comprime la salida de una herramienta del agente antes de que la vea el
// modelo, en los runtimes cuyo sistema de ganchos lo permite (verificado contra
// los binarios: Claude Code 2.1.282 con updatedToolOutput para todas las
// herramientas; Codex 0.157.0 solo con updatedMCPToolOutput; OpenCode con
// output.output mutable en tool.execute.after).
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
	if toolOutputExcluded[name] {
		return true
	}
	return strings.HasPrefix(name, "mcp__gomemory__") || strings.HasPrefix(name, "gomemory_")
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
	case "claude", "codex":
		var ev struct {
			ToolName     string `json:"tool_name"`
			ToolResponse any    `json:"tool_response"`
		}
		if json.Unmarshal(payload, &ev) != nil || ev.ToolResponse == nil || toolOutputIsExcluded(ev.ToolName) {
			return nil
		}
		field := "updatedToolOutput"
		if runtime == "codex" {
			// Codex solo admite reescribir herramientas MCP (verificado en
			// 0.157.0): se reconocen por su resultado con bloques de contenido.
			m, ok := ev.ToolResponse.(map[string]any)
			if !ok {
				return nil
			}
			if _, ok := m["content"].([]any); !ok {
				return nil
			}
			field = "updatedMCPToolOutput"
		}
		nv, changed := rewriteToolResponse(ev.ToolResponse, compress)
		if !changed {
			return nil
		}
		out, _ := json.Marshal(map[string]any{
			"hookSpecificOutput": map[string]any{"hookEventName": "PostToolUse", field: nv},
		})
		return out
	}
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
	minTokens := domain.ToolOutputMinTokens
	compress := func(s string) (string, bool) {
		if (len([]rune(s))+3)/4 < minTokens {
			return "", false
		}
		res, err := deps.Compressor.Compress(s, ports.CompressionOptions{Level: ports.CompressionMax, PreserveCode: true, PreserveURLs: true, PreservePaths: true, PreserveErrors: true})
		if err != nil || len(res.Refs) == 0 {
			return "", false
		}
		return res.Content + RetrieveHint, true
	}
	out := withBudget(domain.HookBudget, func() []byte { return rewriteToolOutput(args[0], payload, compress) })
	if len(out) > 0 {
		_, _ = os.Stdout.Write(out)
	}
	os.Exit(0)
}
