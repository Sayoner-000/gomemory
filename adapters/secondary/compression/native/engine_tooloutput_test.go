package native

import (
	"fmt"
	"strings"
	"testing"

	"mem/application/ports"
	"mem/domain"
)

var toolOpts = ports.CompressionOptions{Level: ports.CompressionMax, ToolOutput: true}

// esSubsecuenciaLiteral comprueba que cada línea de out que no es una marca de
// gomemory existe idéntica en in, en el mismo orden (indentación incluida).
func esSubsecuenciaLiteral(in, out string) (string, bool) {
	orig := strings.Split(in, "\n")
	i := 0
	for _, l := range strings.Split(out, "\n") {
		if domain.IsMarkerLine(strings.TrimSpace(l)) || strings.Contains(l, domain.MarkerTag) {
			continue
		}
		for i < len(orig) && orig[i] != l {
			i++
		}
		if i == len(orig) {
			return l, false
		}
		i++
	}
	return "", true
}

// T043 (feature 035, FR-010…FR-012): en modo salida de herramienta el motor no
// altera nada que el agente pueda necesitar literal.
func TestEngineToolOutput_ProsaIntacta(t *testing.T) {
	in := strings.Repeat("Esta es una frase de diagnóstico que se repite para que el compresor de prosa tenga algo que quitar. ", 30)
	r, _ := NewEngine(newMemStore(), nil, nil, "p").Compress(in, toolOpts)
	if r.Content != in {
		t.Errorf("la prosa de una salida de herramienta debe salir idéntica")
	}
}

func TestEngineToolOutput_CodigoConservaIndentacion(t *testing.T) {
	in := toolOutputFixtureStdout(t, "claude-bash-awk-go.json")
	r, _ := NewEngine(newMemStore(), nil, nil, "p").Compress(in, toolOpts)
	if l, ok := esSubsecuenciaLiteral(in, r.Content); !ok {
		t.Fatalf("línea alterada o inventada (indentación incluida): %q", l)
	}
	if strings.Contains(r.Content, "frases omitidas") {
		t.Error("una sentencia de código nunca se sustituye por «frases omitidas»")
	}
}

func jsonArray(n int) string {
	var b strings.Builder
	b.WriteString("{\"results\": [\n")
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(",\n")
		}
		fmt.Fprintf(&b, `  {"node": "Funcion%02d", "file": "adapters/primary/cli/archivo_%02d.go", "start_line": %d, "label": "Function"}`, i, i, 10+i)
	}
	b.WriteString("\n]}")
	return b.String()
}

func TestEngineToolOutput_ResultadosModeradosCompletos(t *testing.T) {
	in := jsonArray(30)
	r, _ := NewEngine(newMemStore(), nil, nil, "p").Compress(in, toolOpts)
	if strings.Contains(r.Content, "omitidos") || r.Content != in {
		t.Errorf("30 resultados deben llegar completos")
	}
	r, _ = NewEngine(newMemStore(), nil, nil, "p").Compress(jsonArray(60), toolOpts)
	if !strings.Contains(r.Content, "omitidos") {
		t.Errorf("un array de 60 elementos sí se resume")
	}
}

func TestEngineSinToolOutput_SinCambios(t *testing.T) {
	in := jsonArray(30)
	a, _ := NewEngine(newMemStore(), nil, nil, "p").Compress(in, maxOpts)
	if !strings.Contains(a.Content, "omitidos") {
		t.Errorf("fuera del modo salida de herramienta el umbral de arrays no cambia")
	}
}
