package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

func editPayload(session, path string) []byte {
	return []byte(`{"hook_event_name":"PreToolUse","tool_name":"Edit","session_id":"` + session + `","tool_input":{"file_path":"` + path + `","old_string":"a","new_string":"b"}}`)
}

// FR-025: editar código sin unidad HCD activa avisa una vez por episodio y
// nunca bloquea; documentación, una unidad activa o un error no avisan.
func TestRunHookExpand_HCDWarnsOnceWithoutActiveUnit(t *testing.T) {
	var rec []recorded
	d := expandTestDeps(t, &rec)
	d.run = func(context.Context, domain.ExpandSignals, []domain.SupertoolAction, usecases.ExpandHookDeps) domain.ExpandOutcome {
		return domain.ExpandOutcome{}
	}
	active, failure := false, error(nil)
	d.hcdActive = func(context.Context) (bool, error) { return active, failure }
	run := func(session, path string) hookRenderedOutput {
		return runHookExpand(context.Background(), d, "pre-tool", "claude", editPayload(session, path))
	}
	out := run("s1", "app/save.go")
	if !strings.Contains(out.stdout, "sin una unidad HCD activa") || strings.Contains(out.stdout, "deny") || out.exitCode != 0 {
		t.Fatalf("first code edit must warn without blocking: %+v", out)
	}
	if !hasKind(rec, "expand.hcd") {
		t.Fatalf("warning not recorded: %+v", rec)
	}
	if out := run("s1", "app/other.go"); strings.Contains(out.stdout, "HCD") {
		t.Fatalf("warning repeated in the same session: %+v", out)
	}
	for _, c := range []struct {
		session, path string
		active        bool
		failure       error
	}{
		{"s2", "README.md", false, nil},
		{"s3", "hcd/HCD-001-x.md", false, nil},
		{"s4", "app/save.go", true, nil},
		{"s5", "app/save.go", false, errors.New("db locked")},
	} {
		active, failure = c.active, c.failure
		if out := run(c.session, c.path); strings.Contains(out.stdout, "HCD") {
			t.Errorf("%+v must not warn: %+v", c, out)
		}
	}
	d.hcdActive = nil
	active, failure = false, nil
	if out := run("s6", "app/save.go"); strings.Contains(out.stdout, "HCD") {
		t.Fatalf("without HCD wired the hook must not warn: %+v", out)
	}
}
