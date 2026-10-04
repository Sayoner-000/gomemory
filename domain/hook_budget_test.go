package domain

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFitHookOutput_CabeEnteroYOrdenaPorPrioridad(t *testing.T) {
	sections := []HookSection{
		{Name: "memory", Priority: HookPriorityMemory, Text: "memoria", Trimmable: true},
		{Name: "bootstrap", Priority: HookPriorityBootstrap, Text: "bootstrap"},
		{Name: "protocol", Priority: HookPriorityProtocol, Text: "protocolo"},
	}
	got, trims := FitHookOutput(sections, 1000)
	if got != "bootstrap\n\nprotocolo\n\nmemoria" {
		t.Errorf("salida inesperada: %q", got)
	}
	if len(trims) != 0 {
		t.Errorf("no debía recortar nada: %+v", trims)
	}
}

func TestFitHookOutput_OmiteSeccionesVacias(t *testing.T) {
	got, _ := FitHookOutput([]HookSection{
		{Name: "bootstrap", Priority: HookPriorityBootstrap, Text: "a"},
		{Name: "octopus", Priority: HookPriorityOctopus, Text: "", Trimmable: true},
		{Name: "memory", Priority: HookPriorityMemory, Text: "b", Trimmable: true},
	}, 100)
	if got != "a\n\nb" {
		t.Errorf("una sección vacía no debe dejar separadores: %q", got)
	}
}

func TestFitHookOutput_RecortaPrimeroLaPrioridadMenor(t *testing.T) {
	octopus := strings.Repeat("o", 300)
	memory := strings.Repeat("parrafo de memoria.\n\n", 200)
	sections := []HookSection{
		{Name: "bootstrap", Priority: HookPriorityBootstrap, Text: strings.Repeat("b", 200)},
		{Name: "protocol", Priority: HookPriorityProtocol, Text: strings.Repeat("p", 200)},
		{Name: "octopus", Priority: HookPriorityOctopus, Text: octopus, Trimmable: true},
		{Name: "memory", Priority: HookPriorityMemory, Text: memory, Trimmable: true},
	}
	const max = 2000
	got, trims := FitHookOutput(sections, max)

	if n := utf8.RuneCountInString(got); n > max {
		t.Fatalf("la salida supera el tope: %d > %d", n, max)
	}
	if !strings.Contains(got, octopus) {
		t.Error("una sección de mayor prioridad que cabe no debe recortarse")
	}
	if !strings.HasSuffix(got, HookTrimNotice) {
		t.Errorf("un recorte debe terminar con el aviso para pedir el resto: %q", got[len(got)-80:])
	}
	if len(trims) != 1 || trims[0].Name != "memory" || trims[0].EmitRunes >= trims[0].OrigRunes {
		t.Errorf("debía informar el recorte de memory con tamaños: %+v", trims)
	}
}

func TestFitHookOutput_CortaPorParrafosNoAMitadDePalabra(t *testing.T) {
	memory := "primer parrafo completo\n\nsegundo parrafo completo\n\n" + strings.Repeat("x", 500)
	got, _ := FitHookOutput([]HookSection{
		{Name: "bootstrap", Priority: HookPriorityBootstrap, Text: "b"},
		{Name: "memory", Priority: HookPriorityMemory, Text: memory, Trimmable: true},
	}, 200)
	if !strings.Contains(got, "segundo parrafo completo") {
		t.Errorf("debía conservar los párrafos que caben enteros: %q", got)
	}
	if strings.Contains(got, "xxx") {
		t.Errorf("no debía dejar un párrafo cortado a medias: %q", got)
	}
}

func TestFitHookOutput_MideEnRunas(t *testing.T) {
	memory := strings.Repeat("ñandú áéíóú ", 400)
	got, _ := FitHookOutput([]HookSection{
		{Name: "bootstrap", Priority: HookPriorityBootstrap, Text: "inicio"},
		{Name: "memory", Priority: HookPriorityMemory, Text: memory, Trimmable: true},
	}, 1000)
	if n := utf8.RuneCountInString(got); n > 1000 {
		t.Fatalf("el tope se mide en runas: %d > 1000", n)
	}
	if !utf8.ValidString(got) {
		t.Fatal("el recorte no debe partir un carácter multibyte")
	}
}

func TestFitHookOutput_NuncaCortaLoNoRecortable(t *testing.T) {
	protocol := strings.Repeat("regla ", 400) // 2 400 runas, más que el tope
	got, _ := FitHookOutput([]HookSection{
		{Name: "protocol", Priority: HookPriorityProtocol, Text: protocol},
		{Name: "memory", Priority: HookPriorityMemory, Text: "memoria", Trimmable: true},
	}, 1000)
	if !strings.Contains(got, protocol) {
		t.Error("una sección no recortable debe salir entera aunque supere el tope")
	}
}

func TestFitHookOutput_DescartaEnteraLaSeccionQueYaNoCabe(t *testing.T) {
	sections := []HookSection{
		{Name: "bootstrap", Priority: HookPriorityBootstrap, Text: strings.Repeat("b", 150)},
		{Name: "memory", Priority: HookPriorityMemory, Text: strings.Repeat("m", 400), Trimmable: true},
		{Name: "notice", Priority: HookPriorityNotice, Text: "aviso menor", Trimmable: true},
	}
	got, trims := FitHookOutput(sections, 300)
	if strings.Contains(got, "aviso menor") {
		t.Errorf("la sección de menor prioridad debía descartarse: %q", got)
	}
	var notice *HookTrim
	for i := range trims {
		if trims[i].Name == "notice" {
			notice = &trims[i]
		}
	}
	if notice == nil || notice.EmitRunes != 0 {
		t.Errorf("debía informar notice como descartada (EmitRunes=0): %+v", trims)
	}
}
