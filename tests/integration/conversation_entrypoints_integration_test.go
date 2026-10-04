package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

func readConversationFile(t *testing.T, target string) (map[string]any, bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(target, ".memory", ".conversation"))
	if err != nil {
		return nil, false
	}
	var c map[string]any
	if json.Unmarshal(raw, &c) != nil {
		return nil, false
	}
	return c, true
}

// T057 (feature 035, FR-017): los tres puntos de entrada de una conversación.
func TestConversation_SessionStartConIdDeOpenCode(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	huella := filepath.Join(target, ".memory", ".footprint")
	_ = os.WriteFile(huella, []byte("999"), 0o644)

	runMem(t, bin, target, "session", "start", "--conversation=ses_opencode_1")
	c, ok := readConversationFile(t, target)
	if !ok || c["id"] != "ses_opencode_1" {
		t.Fatalf("mem session start --conversation debe registrar la conversación: %v", c)
	}
	if _, err := os.Stat(huella); !os.IsNotExist(err) {
		t.Error("una conversación nueva reinicia el estado por turno")
	}
}

func TestConversation_MCPSinConversacionCreaUnaLocal(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "mcp")
	cmd.Dir = target
	cmd.Stdin = strings.NewReader("") // EOF inmediato: el servidor arranca y termina
	_ = cmd.Run()
	c, ok := readConversationFile(t, target)
	if !ok || !strings.HasPrefix(c["id"].(string), "local-") {
		t.Fatalf("el MCP sin conversación registrada debe crear una local: %v", c)
	}

	runHookWithStdin(t, bin, target, "session-start", `{"session_id":"host-1"}`)
	cmd = exec.Command(bin, "mcp")
	cmd.Dir = target
	cmd.Stdin = strings.NewReader("")
	_ = cmd.Run()
	if c, _ := readConversationFile(t, target); c["id"] != "host-1" {
		t.Errorf("con una conversación ya registrada, el MCP no la toca: %v", c)
	}
}

func TestConversation_ArranqueSinIdDespuesDelCierreAbreOtra(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	runHookWithStdin(t, bin, target, "session-start", `{}`)
	first, ok := readConversationFile(t, target)
	if !ok {
		t.Fatal("el primer arranque sin id debe crear una conversación local")
	}
	runHookWithStdin(t, bin, target, "session-end", `{}`)
	if _, ok := readConversationFile(t, target); ok {
		t.Fatal("session-end debe cerrar la conversación registrada")
	}
	runHookWithStdin(t, bin, target, "session-start", `{}`)
	second, ok := readConversationFile(t, target)
	if !ok || second["id"] == first["id"] {
		t.Fatalf("el segundo arranque sin id debe crear otra conversación: %v, %v", first, second)
	}
}

func TestConversation_DosArranquesSinIdNoCompartenEstado(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	runHookWithStdin(t, bin, target, "session-start", `{}`)
	first, ok := readConversationFile(t, target)
	if !ok {
		t.Fatal("falta la conversación local")
	}
	huella := filepath.Join(target, ".memory", ".footprint")
	_ = os.WriteFile(huella, []byte("123"), 0o600)
	runHookWithStdin(t, bin, target, "session-start", `{}`)
	second, ok := readConversationFile(t, target)
	if !ok || second["id"] == first["id"] {
		t.Fatalf("cada inicio sin id debe crear una conversación distinta: %v, %v", first, second)
	}
	if _, err := os.Stat(huella); !os.IsNotExist(err) {
		t.Fatal("la segunda conversación heredó el estado de la primera")
	}
}

func TestConversation_ReanudarSinIdConservaEstado(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	runHookWithStdin(t, bin, target, "session-start", `{}`)
	first, _ := readConversationFile(t, target)
	runHookWithStdin(t, bin, target, "session-start", `{"source":"resume"}`)
	second, _ := readConversationFile(t, target)
	if second["id"] != first["id"] {
		t.Fatalf("resume sin id debe conservar la conversación: %v, %v", first, second)
	}
}

func TestConversation_SessionEndCLISinFlagCierraConversacion(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	runMem(t, bin, target, "session", "start")
	first, ok := readConversationFile(t, target)
	if !ok {
		t.Fatal("session start debe crear conversación local")
	}
	runMem(t, bin, target, "session", "end", "-s", "terminada")
	if _, ok := readConversationFile(t, target); ok {
		t.Fatal("session end de la CLI debe cerrar la conversación")
	}
	runMem(t, bin, target, "session", "start")
	second, ok := readConversationFile(t, target)
	if !ok || second["id"] == first["id"] {
		t.Fatalf("end + start sin flag debe abrir otra conversación: %v, %v", first, second)
	}
}
