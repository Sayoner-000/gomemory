package domain

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// Presupuesto de las salidas que gomemory inyecta por hooks (feature 035,
// research.md R3). Claude Code no inyecta entera una salida que supera
// HookInlineContextMaxChars: la guarda en un archivo y el modelo solo ve una
// vista previa de unos 2 KB. Medido en vivo: 17 777 caracteres en session-start
// y 12 944 en el primer prompt en modo plan, de los que solo llegaba la primera
// línea. Ajustar aquí es lo que garantiza que lo crítico llegue completo.

// Prioridades de sección: mayor número, más importante. bootstrap y protocol
// nunca se recortan; lo demás se recorta empezando por la prioridad menor.
const (
	HookPriorityBootstrap = 100
	HookPriorityProtocol  = 90
	HookPriorityOctopus   = 80
	HookPriorityPlan      = 70
	HookPriorityMemory    = 50
	HookPriorityNotice    = 40
)

// HookTrimNotice cierra toda salida recortada: le dice al agente cómo pedir lo
// que no cupo en lugar de dejarlo creer que ya lo tiene todo.
const HookTrimNotice = "… contexto recortado: llama get_context() para el resto"

const hookSectionSep = "\n\n"

// HookSection es un fragmento de una salida de hook.
type HookSection struct {
	Name      string
	Priority  int
	Text      string
	Trimmable bool
}

// HookTrim informa un recorte: la sección, su tamaño original y lo emitido
// (0 = descartada entera). Solo cifras: nunca contenido.
type HookTrim struct {
	Name      string
	OrigRunes int
	EmitRunes int
}

// FitHookOutput une las secciones en orden de prioridad y, si superan
// maxRunes, recorta las recortables desde la de menor prioridad. Una sección
// recortada se corta por párrafos y la salida termina con HookTrimNotice. Las
// secciones no recortables salen siempre enteras: su tamaño acotado lo
// garantizan las pruebas, no este recorte.
func FitHookOutput(sections []HookSection, maxRunes int) (string, []HookTrim) {
	ordered := make([]HookSection, 0, len(sections))
	for _, s := range sections {
		if s.Text != "" {
			ordered = append(ordered, s)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Priority > ordered[j].Priority })

	parts := make([]string, len(ordered))
	for i, s := range ordered {
		parts[i] = s.Text
	}
	if utf8.RuneCountInString(strings.Join(parts, hookSectionSep)) <= maxRunes {
		return strings.Join(parts, hookSectionSep), nil
	}

	// Lo fijo (no recortable) se reserva primero; el aviso de recorte también.
	budget := maxRunes - utf8.RuneCountInString(hookSectionSep+HookTrimNotice)
	for _, s := range ordered {
		if !s.Trimmable {
			budget -= utf8.RuneCountInString(s.Text) + utf8.RuneCountInString(hookSectionSep)
		}
	}

	var trims []HookTrim
	kept := make([]string, 0, len(ordered))
	for _, s := range ordered {
		if !s.Trimmable {
			kept = append(kept, s.Text)
			continue
		}
		n := utf8.RuneCountInString(s.Text)
		room := budget - utf8.RuneCountInString(hookSectionSep)
		if n <= room {
			kept = append(kept, s.Text)
			budget -= n + utf8.RuneCountInString(hookSectionSep)
			continue
		}
		cut := cutAtParagraph(s.Text, room)
		emit := utf8.RuneCountInString(cut)
		if cut != "" {
			kept = append(kept, cut)
		}
		trims = append(trims, HookTrim{Name: s.Name, OrigRunes: n, EmitRunes: emit})
		budget = 0
	}
	if len(trims) == 0 {
		// Solo había secciones fijas: no se recortó nada que pedir aparte.
		return strings.Join(kept, hookSectionSep), nil
	}
	return strings.Join(kept, hookSectionSep) + hookSectionSep + HookTrimNotice, trims
}

// cutAtParagraph devuelve el prefijo más largo de s, de como mucho maxRunes
// runas, que termina en un límite de párrafo; si ningún párrafo cabe entero,
// el límite de línea. Nunca parte una palabra ni un carácter multibyte: si no
// cabe ni una línea, devuelve "".
func cutAtParagraph(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	prefix := s
	if utf8.RuneCountInString(s) > maxRunes {
		prefix = string([]rune(s)[:maxRunes])
	}
	if i := strings.LastIndex(prefix, "\n\n"); i > 0 {
		return strings.TrimRight(prefix[:i], "\n")
	}
	if i := strings.LastIndex(prefix, "\n"); i > 0 {
		return prefix[:i]
	}
	return ""
}

// Tipos de evento de las protecciones de hooks (FR-028). Se registran sin
// contenido: solo el tipo, el agente y un detalle numérico o el nombre de la
// herramienta.
const (
	GuardDuplicateDropped   = "duplicate_dropped"
	GuardBudgetTrimmed      = "budget_trimmed"
	GuardToolOutputExcluded = "tool_output_excluded"
)

// GuardCount es el agregado de un tipo de evento para un agente en una ventana.
type GuardCount struct {
	Agent      string
	Kind       string
	Count      int
	LastDetail string
}
