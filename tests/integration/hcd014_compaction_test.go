package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
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
	"mem/domain"
)

// This is a local continuity contract, not observation of a host compactor.
// The host's summary retains the call/result pair and stable session identity.
func TestHCD014CompactionContinuity(t *testing.T) {
	t.Setenv("GOMEMORY_DATA_HOME", t.TempDir())
	t.Setenv("GOMEMORY_NO_CODE_REFRESH", "1")
	bin := buildMemBinary(t)
	ctx := context.Background()
	t.Run("recovery_hint", func(t *testing.T) {
		root := proyectoCompresion(t, `{"tool_output_compression":true,"context_compression_level":"max"}`)
		payload, err := json.Marshal(map[string]any{"session_id": "stable-hint-session", "tool_name": "Bash", "tool_input": map[string]string{"command": "go test ./..."}, "tool_response": map[string]any{"stdout": hcd014Log(), "stderr": "", "interrupted": false}})
		if err != nil {
			t.Fatal(err)
		}
		hook := func() string {
			r := correrMem(t, bin, root, os.Environ(), string(payload), "hook", "tool-output", "claude")
			if r.code != 0 || !strings.Contains(r.stdout, "ref=") {
				t.Fatalf("hook: %s %s", r.stdout, r.stderr)
			}
			return r.stdout
		}
		if !strings.Contains(hook(), "pack_retrieve") {
			t.Fatal("initial reduction has no recovery hint")
		}
		if strings.Contains(hook(), "pack_retrieve") {
			t.Fatal("hint repeated before compacting")
		}
		r := correrMem(t, bin, root, os.Environ(), "", "hook", "post-compact")
		if r.code != 0 {
			t.Fatalf("post-compact: %s", r.stderr)
		}
		if !strings.Contains(hook(), "pack_retrieve") {
			t.Fatal("compaction lost the recovery hint for new reductions")
		}
	})
	for _, agent := range []string{"codex", "opencode", "claude"} {
		t.Run(agent, func(t *testing.T) {
			root := proyectoCompresion(t, `{"context_compression_level":"max"}`)
			if err := persistence.EnsureDir(root); err != nil {
				t.Fatal(err)
			}
			db, err := persistence.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = db.Close() }()
			project := persistence.ProjectKey(root)
			session, err := persistence.NewSessionRepository(db).Start(project)
			if err != nil {
				t.Fatal(err)
			}
			blocks := persistence.NewDeliveredBlocksRepository(db, project, clock.SystemClock{})
			if err := blocks.Mark(ctx, []string{"before-compaction"}); err != nil {
				t.Fatal(err)
			}
			seen, err := blocks.Seen(ctx, []string{"before-compaction"})
			if err != nil || !seen["before-compaction"] {
				t.Fatal("delta fixture not active")
			}
			store := persistence.NewOriginalStoreRepository(db, clock.SystemClock{}, 0, 0)
			grants := persistence.NewAutomaticOriginalGrantsRepository(db)
			scope := ports.AutomaticOriginalScope{Project: project, Agent: agent, SessionID: "stable-" + agent}
			body := hcd014Request(hcd014Log())
			body = append([]byte(`{"instructions":"KEEP MANDATORY INSTRUCTION",`), body[1:]...)
			original := bytes.Clone(body)
			reduced := requestcompression.TransformForSession(ctx, body, "responses", native.NewEngine(store, nil, nil, project), requestcompression.SessionAccess{Originals: store, Grants: grants, Scope: scope})
			refs := regexp.MustCompile(`ref=([a-f0-9]{12,16})`).FindStringSubmatch(string(reduced))
			if len(refs) != 2 || bytes.Equal(reduced, body) {
				t.Fatal("no recoverable reduction")
			}
			// Preserve literal instruction and call/result linkage across this
			// simulated summary boundary; only the eligible result can differ.
			var request struct {
				Instructions string
				Input        []map[string]any
			}
			if err := json.Unmarshal(reduced, &request); err != nil {
				t.Fatal(err)
			}
			if request.Instructions != "KEEP MANDATORY INSTRUCTION" || len(request.Input) != 2 || request.Input[0]["call_id"] != request.Input[1]["call_id"] || request.Input[0]["arguments"] != `{"cmd":"go test ./..."}` {
				t.Fatal("instruction or tool contract altered")
			}
			summary, err := json.Marshal(map[string]string{"compact_summary": string(reduced)})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			r := correrMem(t, bin, root, os.Environ(), string(summary), "hook", "compact-summary")
			if r.code != 0 {
				t.Fatalf("summary: %s", r.stderr)
			}
			r = correrMem(t, bin, root, os.Environ(), "", "hook", "post-compact")
			if r.code != 0 || !strings.Contains(r.stdout, domain.RecoverySteps) || !strings.Contains(r.stdout, refs[1]) {
				t.Fatalf("recovery context missing: %s %s", r.stdout, r.stderr)
			}
			db, err = persistence.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			active, err := persistence.NewSessionRepository(db).Active(project)
			if err != nil || active == nil || active.ID != session.ID || active.Summary != string(reduced) {
				t.Fatal("compaction lost session or summary")
			}
			blocks = persistence.NewDeliveredBlocksRepository(db, project, clock.SystemClock{})
			seen, err = blocks.Seen(ctx, []string{"before-compaction"})
			if err != nil || seen["before-compaction"] {
				t.Fatal("compaction failed to reset delta")
			}
			store = persistence.NewOriginalStoreRepository(db, clock.SystemClock{}, 0, 0)
			grants = persistence.NewAutomaticOriginalGrantsRepository(db)
			got, _, err := usecases.RetrieveAutomaticOriginal(ctx, store, grants, scope, refs[1])
			if err != nil || sha256.Sum256([]byte(got)) != sha256.Sum256([]byte(hcd014Log())) {
				t.Fatalf("original inaccessible after compaction: %v", err)
			}
			resumed := requestcompression.TransformForSession(ctx, body, "responses", native.NewEngine(store, nil, nil, project), requestcompression.SessionAccess{Originals: store, Grants: grants, Scope: scope})
			if !bytes.Equal(resumed, reduced) {
				t.Fatal("resumed session changed the recoverable tool contract")
			}
			exact := bytes.ReplaceAll(body, []byte("go test ./..."), []byte("cat source.go"))
			if got := requestcompression.TransformForSession(ctx, exact, "responses", native.NewEngine(store, nil, nil, project), requestcompression.SessionAccess{Originals: store, Grants: grants, Scope: scope}); !bytes.Equal(got, exact) {
				t.Fatal("resumed session compressed an exact read")
			}
			if !bytes.Equal(body, original) {
				t.Fatal("host original mutated")
			}
		})
	}
}
