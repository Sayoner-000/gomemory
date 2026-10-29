package main

import (
	"encoding/json"
	hcdcli "mem/adapters/primary/cli/hcd"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestHCDPlaybookExportImportAdopt(t *testing.T) {
	bin := buildMemBinary(t)
	root, home := t.TempDir(), t.TempDir()
	call := func(args ...string) []byte {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "GOMEMORY_NO_CODE_REFRESH=1")
		out, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("%v: %v %s", args, e, out)
		}
		return out
	}
	decode := func(data []byte) hcdcli.Response {
		t.Helper()
		var r hcdcli.Response
		if e := json.Unmarshal(data, &r); e != nil || !r.OK {
			t.Fatalf("response=%s %v", data, e)
		}
		return r
	}
	u := decode(call("hcd", "start", "--kind", "fix", "--title", "Prueba gobierno", "--implementer", "dev", "--json", "--client", "codex"))
	proposal := call("docs", "export", "playbook")
	path := filepath.Join(t.TempDir(), "proposal.md")
	if e := os.WriteFile(path, append(proposal, []byte("\nNota de revisión: verificación de adopción explícita.\n")...), 0600); e != nil {
		t.Fatal(e)
	}
	call("docs", "import", "playbook", path)
	before := decode(call("hcd", "show", u.UnitID, "--json"))
	if before.PlaybookRevision != u.PlaybookRevision {
		t.Fatal("import silently adopted revision")
	}
	view := decode(call("hcd", "playbook", "show", u.UnitID, "--json"))
	raw, e := json.Marshal(view.Data)
	if e != nil {
		t.Fatal(e)
	}
	var impact struct{ FromRevision, ToRevision, FromContent, Content string }
	if e = json.Unmarshal(raw, &impact); e != nil {
		t.Fatal(e)
	}
	if impact.FromRevision == impact.ToRevision || impact.FromContent == impact.Content {
		t.Fatal("no adoption impact")
	}
	adopted := decode(call("hcd", "playbook", "adopt", u.UnitID, "--revision", impact.ToRevision, "--quote", "apruebo", "--reason", "import", "--actor", "dev", "--client", "codex", "--expected-revision", strconv.FormatInt(before.Revision.State, 10), "--operation-id", "adopt-import", "--json"))
	if adopted.PlaybookRevision != impact.ToRevision {
		t.Fatalf("adopted=%+v", adopted)
	}
}
