package native

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/domain"
)

// toolOutputFixtureStdout lee el stdout de un fixture real de salida de
// herramienta (tests/contract/testdata/tool_output, feature 035).
func toolOutputFixtureStdout(t testing.TB, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "tests", "contract", "testdata", "tool_output", name))
	if err != nil {
		t.Fatal(err)
	}
	var ev struct {
		ToolResponse struct {
			Stdout string `json:"stdout"`
		} `json:"tool_response"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		t.Fatal(err)
	}
	return ev.ToolResponse.Stdout
}

// T042 (feature 035, FR-009): el código Go que no compila por sí solo —un
// fragmento leído con awk o sed— se reconoce como código, no como prosa.
func TestClassify_GoSinCompilarEsCodigo(t *testing.T) {
	for _, f := range []string{"claude-bash-awk-go.json", "claude-bash-sed-go.json"} {
		typ, lang := classify(toolOutputFixtureStdout(t, f))
		if typ != domain.ContentCode || lang != domain.LangGo {
			t.Errorf("%s: classify = %s/%s, se esperaba code/go", f, typ, lang)
		}
	}
	fragmento := "func (r *Repo) Get(id string) (*Item, error) {\n\tif id == \"\" {\n\t\treturn nil, nil\n\t}\n\titem, err := r.load(id)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn item, nil\n}\n"
	if typ, lang := classify(fragmento); typ != domain.ContentCode || lang != domain.LangGo {
		t.Errorf("fragmento Go sin package: %s/%s", typ, lang)
	}
}

func TestClassify_CodigoIndentadoSinLenguajeClaroEsCodigo(t *testing.T) {
	bloque := strings.Repeat("    procesar(entrada) {\n        valor = leer(entrada);\n        escribir(valor);\n    }\n", 4)
	if typ, _ := classify(bloque); typ != domain.ContentCode {
		t.Errorf("un bloque indentado con llaves y punto y coma es código, got %s", typ)
	}
}

func TestClassify_ListaIndentadaSigueSiendoProsa(t *testing.T) {
	prosa := "Notas de la reunión de hoy con el equipo de producto:\n" +
		"  - revisar la propuesta de precios antes del viernes\n" +
		"  - preparar la demo para el cliente nuevo de la región\n" +
		"  - confirmar con soporte las fechas de la migración\n" +
		"  - enviar el resumen a dirección con los riesgos abiertos\n"
	if typ, _ := classify(prosa); typ != domain.ContentProse {
		t.Errorf("una lista indentada sin puntuación de código es prosa, got %s", typ)
	}
}
