package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractLearnings_TablaDeEncabezados(t *testing.T) {
	cases := []struct {
		name   string
		header string
	}{
		{"español con dos puntos", "## Aprendizajes clave:"},
		{"español sin dos puntos", "## Aprendizajes Clave"},
		{"español genérico", "### Aprendizajes"},
		{"inglés con dos puntos", "## Key Learnings:"},
		{"inglés singular", "## Key Learning"},
		{"inglés genérico", "### Learnings"},
	}
	body := "\n1. El caché se invalida al escribir la configuración del proyecto\n" +
		"2. Los hooks de Codex exigen JSON en el evento SubagentStop\n"

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExtractLearnings(c.header + body)
			if len(got) != 2 {
				t.Fatalf("esperaba 2 ítems, got %d: %v", len(got), got)
			}
			if !strings.Contains(got[0], "caché") {
				t.Errorf("primer ítem inesperado: %q", got[0])
			}
		})
	}
}

func TestExtractLearnings_TomaLaUltimaSeccion(t *testing.T) {
	text := `Resultado inicial.

## Key Learnings
1. Este es el primer intento, descartado, con suficiente longitud para pasar el filtro

Más trabajo.

## Aprendizajes clave
1. Este es el segundo intento, el que cuenta, con longitud suficiente para pasar
`
	got := ExtractLearnings(text)
	if len(got) != 1 {
		t.Fatalf("esperaba 1 ítem de la ÚLTIMA sección, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "segundo intento") {
		t.Errorf("debía tomar la última sección, got %q", got[0])
	}
}

func TestExtractLearnings_CortaEnElSiguienteEncabezado(t *testing.T) {
	text := `## Aprendizajes clave
1. Ítem válido con longitud suficiente para superar el filtro de palabras

## Próximos pasos
1. Esto no es un aprendizaje y no debe aparecer en la lista de arriba
`
	got := ExtractLearnings(text)
	if len(got) != 1 {
		t.Fatalf("esperaba 1 ítem, got %d: %v", len(got), got)
	}
	if strings.Contains(got[0], "Próximos") || strings.Contains(got[0], "Esto no es") {
		t.Errorf("no debía incluir contenido de la sección siguiente: %q", got[0])
	}
}

func TestExtractLearnings_ViñetasYNumerados(t *testing.T) {
	numerado := ExtractLearnings("## Aprendizajes\n1. Primer ítem con longitud suficiente para el filtro\n2. Segundo ítem con longitud suficiente para el filtro\n")
	if len(numerado) != 2 {
		t.Fatalf("numerado: esperaba 2, got %d: %v", len(numerado), numerado)
	}
	viñetas := ExtractLearnings("## Aprendizajes\n- Primer ítem con longitud suficiente para el filtro\n* Segundo ítem con longitud suficiente para el filtro\n")
	if len(viñetas) != 2 {
		t.Fatalf("viñetas: esperaba 2, got %d: %v", len(viñetas), viñetas)
	}
}

func TestExtractLearnings_LimpiaMarkdownBasico(t *testing.T) {
	got := ExtractLearnings("## Aprendizajes\n1. El **caché** se invalida con `SetFoo` y también *esto*, con longitud suficiente\n")
	if len(got) != 1 {
		t.Fatalf("esperaba 1 ítem, got %d: %v", len(got), got)
	}
	if strings.ContainsAny(got[0], "*`") {
		t.Errorf("debía limpiar el markdown básico: %q", got[0])
	}
	if !strings.Contains(got[0], "caché") || !strings.Contains(got[0], "SetFoo") || !strings.Contains(got[0], "esto") {
		t.Errorf("debía conservar el texto sin el marcado: %q", got[0])
	}
}

func TestExtractLearnings_DescartaItemsCortos(t *testing.T) {
	got := ExtractLearnings("## Aprendizajes\n1. corto\n2. Este ítem sí tiene longitud y palabras suficientes para contar\n")
	if len(got) != 1 {
		t.Fatalf("esperaba que el ítem corto se descartara, got %d: %v", len(got), got)
	}
}

func TestExtractLearnings_SinSeccionDevuelveNil(t *testing.T) {
	got := ExtractLearnings("Texto normal sin ninguna sección de aprendizajes.\n1. Esto tampoco cuenta.\n")
	if got != nil {
		t.Errorf("sin sección de aprendizajes debía devolver nil, got %v", got)
	}
	if !reflect.DeepEqual(got, []string(nil)) {
		t.Errorf("tipo inesperado: %#v", got)
	}
}
