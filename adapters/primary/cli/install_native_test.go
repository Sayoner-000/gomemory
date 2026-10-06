package cli

import (
	"bytes"
	"strings"
	"testing"

	"mem/adapters/primary/console"
)

func TestNativeInstallerConsumesActualEventsWithoutLegacyLogs(t *testing.T) {
	var out bytes.Buffer
	flow := console.NewFlow(&out, console.Env{Width: 80, Getenv: func(string) string { return "" }}, "Instalar")
	input := `{"contract_version":1,"type":"start","name":"Instalación"}
{"contract_version":1,"type":"step","name":"Memoria","status":"ok"}
{"contract_version":1,"type":"step","name":"MCP","status":"warn","manual":"mem setup-mcp"}
{"contract_version":1,"type":"complete","status":"warn","exit_code":0}
`
	result, err := consumeNativeInstall(strings.NewReader(input), flow)
	if err != nil || result.Status != "warn" {
		t.Fatalf("resultado: %+v %v", result, err)
	}
	flow.End(result.Status)
	for _, want := range []string{"✓ Memoria", "⚠ MCP", "mem setup-mcp", "avisos"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("falta %q: %s", want, out.String())
		}
	}
}

func TestNativeInstallerRejectsIncompleteOrFalseSuccess(t *testing.T) {
	for _, input := range []string{
		`{"contract_version":1,"type":"start"}` + "\n",
		`{"contract_version":2,"type":"complete","status":"ok"}` + "\n",
		`{"contract_version":1,"type":"step","name":"MCP","status":"warn"}` + "\n" + `{"contract_version":1,"type":"complete","status":"ok","exit_code":0}` + "\n",
		`{"contract_version":1,"type":"complete","status":"fail","exit_code":0}` + "\n",
	} {
		flow := console.NewFlow(&bytes.Buffer{}, console.Env{Width: 80}, "Instalar")
		if _, err := consumeNativeInstall(strings.NewReader(input), flow); err == nil {
			t.Errorf("protocolo inválido aceptado: %s", input)
		}
	}
}

func TestNativeNDJSONRejectsBlankLines(t *testing.T) {
	complete := `{"contract_version":1,"type":"complete","status":"ok","exit_code":0}` + "\n"
	for _, blank := range []string{"\n", "\r\n", " \t\n"} {
		for _, input := range []string{blank + complete, complete + blank, complete + " "} {
			flow := console.NewFlow(&bytes.Buffer{}, console.Env{Width: 80}, "Instalar")
			if _, err := consumeNativeInstall(strings.NewReader(input), flow); err == nil {
				t.Errorf("línea en blanco aceptada: %q", input)
			}
		}
	}
}
