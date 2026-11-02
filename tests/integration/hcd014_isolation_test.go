package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/compression/native"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/requestcompression"
	"mem/application/ports"
	"mem/application/usecases"
)

func TestHCD014RecoveryIsolation(t *testing.T) {
	t.Run("hook_and_cli", testHCD014AutomaticRecoveryCLI)
	ctx := context.Background()
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	store := persistence.NewOriginalStoreRepository(db, clock.SystemClock{}, 0, 0)
	grants := persistence.NewAutomaticOriginalGrantsRepository(db)
	owner := ports.AutomaticOriginalScope{Project: "alpha", Agent: "codex", SessionID: "host-session-1"}
	body := hcd014Request(hcd014Log())
	out := requestcompression.TransformForSession(ctx, body, "responses", native.NewEngine(store, nil, nil, owner.Project), requestcompression.SessionAccess{Originals: store, Grants: grants, Scope: owner})
	if string(out) == string(body) {
		t.Fatal("scoped route did not reduce")
	}
	scopedRef := regexp.MustCompile(`ref=([a-f0-9]{12,16})`).FindStringSubmatch(string(out))
	if len(scopedRef) != 2 {
		t.Fatal("scoped result has no original reference")
	}
	if got, _, err := usecases.RetrieveAutomaticOriginal(ctx, store, grants, owner, scopedRef[1]); err != nil || got != hcd014Log() {
		t.Fatal("scoped request result not recoverable")
	}
	if !strings.Contains(string(out), "--automatic --agent codex --session-id 'host-session-1'") {
		t.Fatal("scoped delivery omitted automatic recovery instructions")
	}
	for _, access := range []requestcompression.SessionAccess{
		{Originals: store, Grants: grants, Scope: ports.AutomaticOriginalScope{Project: owner.Project, Agent: owner.Agent}},
		{Originals: store, Scope: owner},
		{Grants: grants, Scope: owner},
		{Originals: store, Grants: grants, Scope: ports.AutomaticOriginalScope{Project: "beta", Agent: owner.Agent, SessionID: owner.SessionID}},
	} {
		got := requestcompression.TransformForSession(ctx, body, "responses", native.NewEngine(store, nil, nil, owner.Project), access)
		if string(got) != string(body) {
			t.Fatal("route published reduction without recoverable session grant")
		}
	}
	ref, err := store.Put(ctx, "alpha", "original from session one", "log", "log")
	if err != nil {
		t.Fatal(err)
	}
	if err := usecases.GrantAutomaticOriginals(ctx, store, grants, owner, []string{ref}); err != nil {
		t.Fatal(err)
	}
	// A second session can hold its own reference, without inheriting the first.
	other := owner
	other.SessionID = "host-session-2"
	ref2, err := store.Put(ctx, "alpha", "original from session two", "log", "log")
	if err != nil {
		t.Fatal(err)
	}
	if err := usecases.GrantAutomaticOriginals(ctx, store, grants, other, []string{ref2}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	store = persistence.NewOriginalStoreRepository(db, clock.SystemClock{}, 0, 0)
	grants = persistence.NewAutomaticOriginalGrantsRepository(db)
	got, _, err := usecases.RetrieveAutomaticOriginal(ctx, store, grants, owner, ref)
	if err != nil || got != "original from session one" {
		t.Fatalf("same stable session after reopen: %q %v", got, err)
	}
	for _, scope := range []ports.AutomaticOriginalScope{
		other,
		{Project: "beta", Agent: "codex", SessionID: owner.SessionID},
		{Project: "alpha", Agent: "claude", SessionID: owner.SessionID},
		{Project: "alpha", Agent: "codex"},
	} {
		got, _, err := usecases.RetrieveAutomaticOriginal(ctx, store, grants, scope, ref)
		if !errors.Is(err, usecases.ErrOriginalNotFound) || got != "" {
			t.Fatalf("foreign/missing scope recovered: %+v %q %v", scope, got, err)
		}
	}
	if got, _, err := usecases.RetrieveAutomaticOriginal(ctx, store, grants, owner, ref2); got != "" || !errors.Is(err, usecases.ErrOriginalNotFound) {
		t.Fatal("owner inherited session two")
	}
	// Existing CLI/MCP use this project-scoped function: Q3 A preserves it.
	if got, _, err := usecases.RetrieveOriginal(ctx, store, "alpha", ref); err != nil || got != "original from session one" {
		t.Fatal("manual retrieval compatibility broken")
	}
	if err := usecases.GrantAutomaticOriginals(ctx, store, grants, ports.AutomaticOriginalScope{Project: "beta", Agent: "codex", SessionID: "foreign"}, []string{ref}); err == nil {
		t.Fatal("foreign project granted original")
	}
	if err := usecases.GrantAutomaticOriginals(ctx, store, grants, ports.AutomaticOriginalScope{Project: "alpha", Agent: "codex"}, []string{ref}); err == nil {
		t.Fatal("missing identity granted original")
	}
	// TTL/LRU deletion must revoke grants even with this repository's default
	// foreign_keys=OFF. Recreating identical content must not revive access.
	if _, err := db.Exec("DELETE FROM compression_originals WHERE ref = ?", ref); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(ctx, "alpha", "original from session one", "log", "log"); err != nil {
		t.Fatal(err)
	}
	if got, _, err := usecases.RetrieveAutomaticOriginal(ctx, store, grants, owner, ref); got != "" || !errors.Is(err, usecases.ErrOriginalNotFound) {
		t.Fatal("deleted original revived an old session grant")
	}
}

func testHCD014AutomaticRecoveryCLI(t *testing.T) {
	bin := buildMemBinary(t)
	root := proyectoCompresion(t, `{"tool_output_compression":true,"context_compression_level":"max"}`)
	env := append(os.Environ(), "GOMEMORY_DATA_HOME="+t.TempDir(), "GOMEMORY_NO_CODE_REFRESH=1")
	payload, _ := json.Marshal(map[string]any{"session_id": "stable-host-session", "tool_name": "Bash", "tool_input": map[string]any{"command": "go test ./..."}, "tool_response": map[string]any{"stdout": hcd014Log(), "stderr": "", "interrupted": false}})
	r := correrMem(t, bin, root, env, string(payload), "hook", "tool-output", "claude")
	refs := regexp.MustCompile(`ref=([a-f0-9]{12,16})`).FindStringSubmatch(r.stdout)
	if r.code != 0 || len(refs) != 2 {
		t.Fatalf("hook did not publish original: %s %s", r.stdout, r.stderr)
	}
	ref := refs[1]
	for _, tc := range []struct {
		name, session string
		want          int
	}{{"owner", "stable-host-session", 0}, {"foreign", "another-session", 2}, {"missing", "", 2}} {
		t.Run(tc.name, func(t *testing.T) {
			r := correrMem(t, bin, root, env, "", "pack", "retrieve", "--automatic", "--agent", "claude", "--session-id", tc.session, ref)
			if r.code != tc.want {
				t.Fatalf("automatic retrieval exit=%d want=%d stdout=%.100s stderr=%s", r.code, tc.want, r.stdout, r.stderr)
			}
			if tc.want == 0 && r.stdout != hcd014Log() {
				t.Fatal("owner content changed")
			}
			if tc.want != 0 && r.stdout != "" {
				t.Fatal("foreign scope leaked content")
			}
		})
	}
	manual := correrMem(t, bin, root, env, "", "pack", "retrieve", ref)
	if manual.code != 0 || manual.stdout != hcd014Log() {
		t.Fatalf("manual retrieval changed: %d %s", manual.code, manual.stderr)
	}
}
