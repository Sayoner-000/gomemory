package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"mem/domain"
)

// HCD-028 VS-03: el hook inyecta el contexto de edición una vez por archivo y
// sesión, igual en los cuatro runtimes, sin bloquear la edición.

func editInput(ids ...int64) func(context.Context, string) domain.EditContextInput {
	return func(_ context.Context, path string) domain.EditContextInput {
		in := domain.EditContextInput{Path: "domain/rules.go"}
		for _, id := range ids {
			in.Memories = append(in.Memories, domain.EditContextMemory{ID: id, Type: "decision", Title: "decisión"})
		}
		return in
	}
}

func writePayload(session string) []byte {
	return []byte(`{"hook_event_name":"PreToolUse","tool_name":"Write","session_id":"` + session + `","tool_input":{"file_path":"/repo/domain/rules.go","content":"x"}}`)
}

func TestEditContext_UnaVezPorArchivo(t *testing.T) {
	var rec []recorded
	d := expandTestDeps(t, &rec)
	d.editContext = editInput(5)

	first := runHookExpand(context.Background(), d, "pre-tool", "", writePayload("s1"))
	if !strings.Contains(first.stdout, "Contexto de domain/rules.go") || !strings.Contains(first.stdout, "get_memory 5") {
		t.Fatalf("la primera edición debe recibir el contexto: %+v", first)
	}
	if strings.Contains(first.stdout, `"permissionDecision"`) {
		t.Fatalf("el contexto nunca aprueba ni deniega la herramienta (G6): %s", first.stdout)
	}
	if second := runHookExpand(context.Background(), d, "pre-tool", "", writePayload("s1")); strings.Contains(second.stdout, "Contexto de") {
		t.Fatalf("la segunda edición del mismo archivo no repite el contexto: %+v", second)
	}

	// Una memoria nueva anclada al archivo vuelve a merecer contexto.
	d.editContext = editInput(6, 5)
	if again := runHookExpand(context.Background(), d, "pre-tool", "", writePayload("s1")); !strings.Contains(again.stdout, "get_memory 6") {
		t.Fatalf("una memoria nueva debe volver a inyectar el contexto: %+v", again)
	}

	// Tras compactar, lo entregado se perdió: se vuelve a inyectar.
	runHookExpand(context.Background(), d, "session-start", "", []byte(`{"source":"compact","session_id":"s1"}`))
	if after := runHookExpand(context.Background(), d, "pre-tool", "", writePayload("s1")); !strings.Contains(after.stdout, "Contexto de") {
		t.Fatalf("tras compactar debe volver a inyectarse: %+v", after)
	}

	// Las claves edit: y hcd: no son marcadores corruptos: las sugerencias de
	// flujo siguen saliendo en la misma sesión.
	if !expandEpisodeClaim(d.root, expandSessionKey("s1"), "hcd:no-unit") {
		t.Fatal("no se pudo reclamar hcd:no-unit")
	}
	commit := `{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"s1","tool_input":{"command":"git commit -m x"}}`
	if out := runHookExpand(context.Background(), d, "pre-tool", "", []byte(commit)); !strings.Contains(out.stdout, domain.SkillVerifyLoop) {
		t.Fatalf("con claves edit:/hcd: en el episodio la sugerencia verify-loop debe seguir saliendo: %+v", out)
	}
}

func TestEditContext_CuatroRuntimes(t *testing.T) {
	cases := []struct{ name, emit, payload string }{
		{"claude", "", `{"hook_event_name":"PreToolUse","tool_name":"Edit","session_id":"c","tool_input":{"file_path":"/repo/domain/rules.go","new_string":"x"}}`},
		{"codex", "json", `{"tool_name":"apply_patch","session_id":"x","tool_input":{"command":"*** Begin Patch\n*** Update File: domain/rules.go\n@@\n-a\n+b\n*** End Patch"}}`},
		{"agy", "agy", `{"hook_event_name":"PreToolUse","tool_name":"Write","session_id":"a","tool_input":{"file_path":"/repo/domain/rules.go","content":"x"}}`},
		{"opencode", "text", `{"tool":"edit","sessionID":"o","args":{"filePath":"/repo/domain/rules.go","newString":"x"}}`},
	}
	for _, c := range cases {
		for _, off := range []bool{false, true} {
			var rec []recorded
			d := expandTestDeps(t, &rec)
			var gotPath string
			d.editContext = func(ctx context.Context, p string) domain.EditContextInput {
				gotPath = p
				return editInput(7)(ctx, p)
			}
			if off {
				d.settings.ExpandOff = []string{string(domain.SupertoolContext)}
			}
			out := runHookExpand(context.Background(), d, "pre-tool", c.emit, []byte(c.payload))
			text := out.stdout
			if c.emit == "" || c.emit == "json" {
				var env struct {
					HookSpecificOutput struct {
						AdditionalContext string `json:"additionalContext"`
					} `json:"hookSpecificOutput"`
				}
				_ = json.Unmarshal([]byte(out.stdout), &env)
				text = env.HookSpecificOutput.AdditionalContext
			}
			has := strings.Contains(text, "Contexto de domain/rules.go") && strings.Contains(text, "get_memory 7")
			if has == off {
				t.Errorf("%s (context apagada=%v): contexto presente=%v; salida %+v", c.name, off, has, out)
			}
			if !off && !strings.HasSuffix(gotPath, "domain/rules.go") {
				t.Errorf("%s: la ruta editada no llegó a la fuente: %q", c.name, gotPath)
			}
		}
	}
}

func TestEditContext_Plazo(t *testing.T) {
	var rec []recorded
	d := expandTestDeps(t, &rec)
	d.editContext = func(ctx context.Context, _ string) domain.EditContextInput {
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
		return domain.EditContextInput{Path: "domain/rules.go", Memories: []domain.EditContextMemory{{ID: 1, Type: "decision", Title: "tarde"}}}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	start := time.Now()
	out := runHookExpand(ctx, d, "pre-tool", "", writePayload("lento"))
	if time.Since(start) > time.Second {
		t.Fatalf("una fuente lenta no debe retener la edición: %v", time.Since(start))
	}
	if strings.Contains(out.stdout, "deny") || strings.Contains(out.stdout, "tarde") {
		t.Fatalf("con la fuente lenta la edición sigue, sin su contexto: %+v", out)
	}
}
