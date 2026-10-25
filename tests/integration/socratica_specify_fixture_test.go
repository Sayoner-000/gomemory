package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// examplePreparationPath es el ejemplo completo del contrato de 042; los
// recorridos de specify lo usan como datos sintéticos, sin modelos reales.
var examplePreparationPath = filepath.Join("..", "..", "specs", "042-socratica-specify-preparation", "contracts", "example-preparation.json")

// loadExamplePreparation devuelve las secciones intent, scope y stories del
// ejemplo, listas para enviarse como value de prepare(set).
func loadExamplePreparation(t *testing.T) map[string]map[string]any {
	t.Helper()
	raw, err := os.ReadFile(examplePreparationPath)
	if err != nil {
		t.Fatal(err)
	}
	var sections map[string]map[string]any
	if err = json.Unmarshal(raw, &sections); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"intent", "scope", "stories"} {
		if sections[name] == nil {
			t.Fatalf("ejemplo sin sección %s", name)
		}
	}
	return sections
}

// specifyFixture es un proyecto desechable con el servidor MCP real y un
// agente Spec Kit falso que cuenta cuántas veces se le entrega una entrada.
type specifyFixture struct {
	*socraticaSession
	home    string
	counter string
	// workdir is the agent's project: where the synthetic generator writes. The
	// server never reads it; only the agent inspects it.
	workdir string
}

func newSpecifyFixture(t *testing.T) *specifyFixture {
	t.Helper()
	home := t.TempDir()
	return &specifyFixture{socraticaSession: newSocraticaSession(t, home), home: home, counter: filepath.Join(t.TempDir(), "speckit-invocations"), workdir: t.TempDir()}
}

// deliver simula que el agente invoca speckit-specify con prompt. El agente
// falso es este mismo binario de pruebas (TestHelperSpecKitAgent), así que no
// depende de un shell y corre también en Windows.
func (f *specifyFixture) deliver(prompt string) {
	f.t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperSpecKitAgent$")
	cmd.Env = append(os.Environ(), "GOMEMORY_TEST_SPECKIT_COUNTER="+f.counter)
	cmd.Stdin = strings.NewReader(prompt)
	if out, err := cmd.CombinedOutput(); err != nil {
		f.t.Fatalf("agente Spec Kit falso: %v\n%s", err, out)
	}
}

// invocations devuelve las entradas recibidas por el agente falso, en orden.
func (f *specifyFixture) invocations() []string {
	f.t.Helper()
	raw, err := os.ReadFile(f.counter)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		f.t.Fatal(err)
	}
	var got []string
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		var prompt string
		if err = json.Unmarshal([]byte(line), &prompt); err != nil {
			f.t.Fatal(err)
		}
		got = append(got, prompt)
	}
	return got
}

// TestHelperSpecKitAgent no es una prueba: es el agente falso que relanza
// specifyFixture.deliver. Añade la entrada recibida como una línea JSON.
func TestHelperSpecKitAgent(t *testing.T) {
	counter := os.Getenv("GOMEMORY_TEST_SPECKIT_COUNTER")
	if counter == "" {
		return
	}
	prompt, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(2)
	}
	line, _ := json.Marshal(string(prompt))
	file, err := os.OpenFile(counter, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		os.Exit(2)
	}
	_, err = file.Write(append(line, '\n'))
	if closeErr := file.Close(); err != nil || closeErr != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestSpecifyFixture_LoadsExampleAndCountsInvocations(t *testing.T) {
	sections := loadExamplePreparation(t)
	if actors, _ := sections["intent"]["actors"].([]any); len(actors) != 1 {
		t.Fatalf("actors = %v, want 1", sections["intent"]["actors"])
	}
	f := newSpecifyFixture(t)
	if got := f.invocations(); len(got) != 0 {
		t.Fatalf("invocaciones iniciales = %v, want 0", got)
	}
	f.deliver("primera\nentrada")
	f.deliver("segunda")
	if got := f.invocations(); len(got) != 2 || got[0] != "primera\nentrada" || got[1] != "segunda" {
		t.Fatalf("invocaciones = %q", got)
	}
}

// fails sends a call that must be refused and returns the error text.
func (f *specifyFixture) fails(args map[string]any) string {
	f.t.Helper()
	out, ok := f.raw(args)
	if ok {
		f.t.Fatalf("%v: expected an error, got %v", args, out)
	}
	return out["error"].(string)
}
