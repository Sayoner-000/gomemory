package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

// Two MCP processes over one project, privacy and loss of an instance. The
// private payload lives only in the memory of the process that created it; the
// shared database keeps a reservation without text.

const privateSentinel = "SENTINELA-PRIVADA-042"

// privateRoot is a project whose settings turn the private mode on, the way the
// person would before starting the agent's MCP server.
// dataHomes keeps one data home per test so every process of the test shares it.
var dataHomes = map[string]string{}

func sharedDataHome(t *testing.T) string {
	t.Helper()
	if dir, ok := dataHomes[t.Name()]; ok {
		return dir
	}
	dir := t.TempDir()
	dataHomes[t.Name()] = dir
	return dir
}

func privateRoot(t *testing.T, private bool) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("GOMEMORY_DATA_HOME", sharedDataHome(t))
	if err := persistence.EnsureDir(root); err != nil {
		t.Fatal(err)
	}
	if private {
		writePrivate(t, root, true)
	}
	return root
}

func writePrivate(t *testing.T, root string, private bool) {
	t.Helper()
	body := `{"socratica_private":false}`
	if private {
		body = `{"socratica_private":true}`
	}
	if err := os.WriteFile(persistence.SettingsPath(root), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixtureOn(t *testing.T, s *socraticaSession) *specifyFixture {
	t.Helper()
	return &specifyFixture{socraticaSession: s, home: t.TempDir(), counter: filepath.Join(t.TempDir(), "invocations"), workdir: t.TempDir()}
}

// durableText reports every file under dirs that contains the sentinel.
func durableText(t *testing.T, sentinel string, dirs ...string) []string {
	t.Helper()
	var hits []string
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if data, e := os.ReadFile(path); e == nil && bytes.Contains(data, []byte(sentinel)) {
				hits = append(hits, path)
			}
			return nil
		})
	}
	return hits
}

func TestSpecifyPrivate_TwoInstancesShareOnlyTheReservation(t *testing.T) {
	root := privateRoot(t, true)
	homeA, homeB := t.TempDir(), t.TempDir()
	a := fixtureOn(t, newSocraticaSessionAt(t, homeA, root, sharedDataHome(t)))

	sections := loadExamplePreparation(t)
	id := a.call(map[string]any{"action": "start", "client": "codex", "mode": "specify", "prompt": "Quiero exportar mi historial " + privateSentinel})["flow_id"].(string)
	a.call(map[string]any{"action": "answer", "flow_id": id, "questions": []any{map[string]any{"text": "¿Sobre " + privateSentinel + "?", "options": []string{"Sí", "No"}}}})
	a.call(map[string]any{"action": "answer", "flow_id": id, "skip_rest": true})
	for i, name := range []string{"intent", "scope", "stories"} {
		a.set(id, i, name, sections[name])
	}
	var review map[string]any
	for _, stage := range []string{"intent", "scope", "stories", "validation"} {
		review = a.check(id, 3, stage)
	}
	if review["original"] == "" || !strings.Contains(review["original"].(string), privateSentinel) {
		t.Fatalf("the owning instance must read its own payload: %v", review["original"])
	}
	dispatch := a.call(map[string]any{"action": "approve", "flow_id": id, "expected_revision": review["revision"]})["dispatch_id"].(string)

	// Nothing durable holds the private text: not the database, WAL, locks, nor the home.
	if hits := durableText(t, privateSentinel, root, homeA, sharedDataHome(t)); len(hits) != 0 {
		t.Fatalf("private text found on disk: %v", hits)
	}

	// Another instance of the same project sees a reservation and nothing else.
	b := fixtureOn(t, newSocraticaSessionAt(t, homeB, root, sharedDataHome(t)))
	seen := b.call(map[string]any{"action": "get", "flow_id": id})
	if seen["private_state_unavailable"] != true || seen["original"] != "" || seen["proposed"] != "" || seen["approved_prompt"] != "" || seen["preparation"] != nil {
		t.Fatalf("instance B saw private content: %v", seen)
	}
	if actions := stringsOf(seen["allowed_actions"]); len(actions) != 2 || !contains(actions, "get") || !contains(actions, "cancel") {
		t.Fatalf("allowed actions of a reservation = %v", actions)
	}
	// One pending flow per project and agent, across processes.
	again := b.call(map[string]any{"action": "start", "client": "codex", "mode": "specify", "prompt": "otra necesidad"})
	if again["recovered"] != true || again["flow_id"] != id || again["private_state_unavailable"] != true {
		t.Fatalf("a second pending flow was created or the reservation was not recovered: %v", again)
	}
	// Nothing that needs the content can be rebuilt from input.
	for name, args := range map[string]map[string]any{
		"set":          {"action": "prepare", "flow_id": id, "expected_revision": 3, "preparation": map[string]any{"operation": "set", "section": "intent", "value": sections["intent"]}},
		"validate":     {"action": "prepare", "flow_id": id, "expected_revision": 3, "preparation": map[string]any{"operation": "validate", "stage": "intent"}},
		"approve":      {"action": "approve", "flow_id": id, "expected_revision": 3},
		"ack":          {"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": "n1"},
		"resume":       {"action": "resume", "flow_id": id},
		"force_resend": {"action": "force_resend", "flow_id": id, "dispatch_id": dispatch},
		"report":       {"action": "report", "flow_id": id, "dispatch_id": dispatch, "outcome": map[string]any{"status": "failed", "message": "x"}},
	} {
		if msg := b.fails(args); !strings.Contains(msg, "private_state_unavailable") {
			t.Errorf("%s from the other instance = %q", name, msg)
		}
	}
	// The owner still has everything, and nothing was consumed by B's attempts.
	if got := a.call(map[string]any{"action": "get", "flow_id": id}); got["phase"] != "approved" || got["execution_granted"] != false || got["dispatch_id"] != dispatch {
		t.Fatalf("B's refused attempts changed the owner's flow: %v", got)
	}
	// Explicit cancel of the reservation from the other instance frees the slot,
	// and the owner learns it: its payload is useless now.
	cancelled := b.call(map[string]any{"action": "cancel", "flow_id": id})
	if cancelled["phase"] != "cancelled" || cancelled["original"] != "" {
		t.Fatalf("cancel of the reservation = %v", cancelled)
	}
	if msg := a.fails(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": "n1"}); msg == "" {
		t.Fatal("the owner could still ack a cancelled reservation")
	}
	fresh := b.call(map[string]any{"action": "start", "client": "codex", "mode": "specify", "prompt": "otra necesidad"})
	if fresh["flow_id"] == id || fresh["recovered"] == true {
		t.Fatalf("explicit new start after the cancel = %v", fresh)
	}
	if hits := durableText(t, privateSentinel, root, homeA, homeB, sharedDataHome(t)); len(hits) != 0 {
		t.Fatalf("private text found on disk at the end: %v", hits)
	}
}

func stringsOf(v any) []string {
	var out []string
	list, _ := v.([]any)
	for _, x := range list {
		out = append(out, x.(string))
	}
	return out
}

// Losing the instance that holds the payload loses the content, never turns
// into a recoverable authorization.
func TestSpecifyPrivate_LosingTheInstanceDropsContentNotControl(t *testing.T) {
	root := privateRoot(t, true)
	home := t.TempDir()
	first := newSocraticaSessionAt(t, home, root, sharedDataHome(t))
	a := fixtureOn(t, first)
	id, _ := a.toReview("Quiero exportar mi historial " + privateSentinel)
	dispatch := a.call(map[string]any{"action": "approve", "flow_id": id, "expected_revision": 3})["dispatch_id"].(string)
	if err := first.session.Close(); err != nil {
		t.Log("closing the first instance:", err)
	}

	second := fixtureOn(t, newSocraticaSessionAt(t, home, root, sharedDataHome(t)))
	got := second.call(map[string]any{"action": "get", "flow_id": id})
	if got["private_state_unavailable"] != true || got["approved_prompt"] != "" || got["execution_granted"] != false {
		t.Fatalf("after the loss = %v", got)
	}
	if msg := second.fails(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": "n1"}); !strings.Contains(msg, "private_state_unavailable") {
		t.Fatalf("ack after the loss = %q", msg)
	}
	if msg := second.fails(map[string]any{"action": "approve", "flow_id": id, "expected_revision": 3}); !strings.Contains(msg, "private_state_unavailable") {
		t.Fatalf("approve after the loss = %q", msg)
	}
	// The slot is blocked until the person decides; then a fresh flow can start.
	if again := second.call(map[string]any{"action": "start", "client": "codex", "mode": "specify", "prompt": "nueva"}); again["flow_id"] != id {
		t.Fatalf("a new flow replaced the reservation: %v", again["flow_id"])
	}
	second.call(map[string]any{"action": "cancel", "flow_id": id})
	fresh := second.call(map[string]any{"action": "start", "client": "codex", "mode": "specify", "prompt": "nueva necesidad"})
	if fresh["flow_id"] == id || fresh["recovered"] == true || fresh["phase"] != "captured" {
		t.Fatalf("explicit new start after the cancel = %v", fresh)
	}
	if hits := durableText(t, privateSentinel, root, home, sharedDataHome(t)); len(hits) != 0 {
		t.Fatalf("private text found on disk after the loss: %v", hits)
	}
}

// The policy is fixed at the start: flipping the setting later neither dumps
// the payload to disk nor makes another process see it.
func TestSpecifyPrivate_TogglingTheSettingDoesNotMoveThePayload(t *testing.T) {
	root := privateRoot(t, true)
	home := t.TempDir()
	a := fixtureOn(t, newSocraticaSessionAt(t, home, root, sharedDataHome(t)))
	id, _ := a.toReview("Quiero exportar mi historial " + privateSentinel)
	writePrivate(t, root, false)
	a.call(map[string]any{"action": "approve", "flow_id": id, "expected_revision": 3})

	db, err := persistence.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var policy string
	var histories int
	if err = db.QueryRow("SELECT payload_policy FROM socratica_flows WHERE id=?", id).Scan(&policy); err != nil || policy != "ephemeral" {
		t.Fatalf("payload_policy = %q, %v", policy, err)
	}
	if err = db.QueryRow("SELECT COUNT(*) FROM socratica_history").Scan(&histories); err != nil || histories != 0 {
		t.Fatalf("history rows = %d, %v", histories, err)
	}
	if hits := durableText(t, privateSentinel, root, home, sharedDataHome(t)); len(hits) != 0 {
		t.Fatalf("the payload reached the disk after the toggle: %v", hits)
	}
	// A process started with the setting off still treats the flow as a reservation.
	c := fixtureOn(t, newSocraticaSessionAt(t, t.TempDir(), root, sharedDataHome(t)))
	if got := c.call(map[string]any{"action": "get", "flow_id": id}); got["private_state_unavailable"] != true {
		t.Fatalf("instance C = %v", got)
	}
}

// Control: the residue scanner does find the text of a persistent flow, so a
// clean scan of a private one means something.
func TestSpecifyPrivate_ScannerFindsPersistentText(t *testing.T) {
	root := privateRoot(t, false)
	home := t.TempDir()
	a := fixtureOn(t, newSocraticaSessionAt(t, home, root, sharedDataHome(t)))
	a.call(map[string]any{"action": "start", "client": "codex", "mode": "specify", "prompt": "Quiero exportar " + privateSentinel})
	if hits := durableText(t, privateSentinel, root, sharedDataHome(t)); len(hits) == 0 {
		t.Fatal("the scanner found nothing in a persistent flow: it cannot prove the private case")
	}
}
