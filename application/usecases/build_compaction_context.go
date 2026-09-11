package usecases

import (
	"fmt"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// compactionEntryExtractChars es el largo del extracto por memoria en el
// contexto de compactación (feature 030, research.md R7).
const compactionEntryExtractChars = 300

// compactionSummaryExtractChars acota el resumen previo de la sesión que se
// antepone a las entradas.
const compactionSummaryExtractChars = 600

// compactionMemoriesFetchLimit es el máximo de memorias de la sesión que se
// consultan antes de aplicar el tope de presupuesto. La sesión rara vez tiene
// más de unas pocas decenas de memorias propias; un techo generoso evita una
// consulta sin límite sin restringir el caso real.
const compactionMemoriesFetchLimit = 200

// BuildCompactionContext arma el bloque de memoria de la sesión activa que se
// entrega al compresor (capacidad C1) y/o al agente que reanuda tras
// compactar (capacidad C2) — feature 030, US1.
//
// Simplificación deliberada frente a data-model.md: NO incluye el último
// prompt de la sesión. domain.Session no expone ese campo (ActiveSession no
// lo selecciona) y añadirlo obligaría a tocar ports.SessionQuerier, que tiene
// dobles de prueba en adapters/primary/tui/tui_usage_test.go y
// adapters/primary/cli/cmd_save_test.go — la constitución prohíbe modificar
// tests existentes sin autorización. El resumen previo y las memorias de la
// sesión ya cubren el propósito (recuperar qué se hizo), así que se pospone.
//
// budget es el tope TOTAL en caracteres para la salida (ya calculado por el
// llamador como una fracción del presupuesto de arranque). budget<=0
// significa sin límite, igual que Builder.Budget.
func BuildCompactionContext(sessions ports.SessionQuerier, mems ports.SessionMemoryLister, project string, budget int) (string, error) {
	active, err := sessions.Active(project)
	if err != nil {
		return "", err
	}
	if active == nil {
		return "", nil
	}

	entries, err := mems.ListBySession(project, active.ID, compactionMemoriesFetchLimit)
	if err != nil {
		return "", err
	}
	entries = prioritizeActionableFirst(entries)

	var b strings.Builder
	b.WriteString(domain.CompactionContextHeader)
	b.WriteString("\n\n")

	if summary := strings.TrimSpace(active.Summary); summary != "" {
		fmt.Fprintf(&b, "Resumen previo: %s\n\n", domain.Extract(summary, compactionSummaryExtractChars))
	}

	if len(entries) == 0 {
		b.WriteString(domain.CompactionEmptySessionNote)
		b.WriteString("\n")
		return b.String(), nil
	}

	// Reserva de espacio para la nota de omitidas, igual en espíritu a
	// budgetReserve de usecases.Builder: sin margen, la última línea antes de
	// agotar el presupuesto podría dejar la salida justo en el límite y sin
	// espacio para avisar que se recortó.
	const omittedNoteReserve = 120

	incluidas := 0
	for _, m := range entries {
		linea := formatCompactionEntry(m)
		if budget > 0 && b.Len()+len(linea) > budget-omittedNoteReserve {
			break
		}
		b.WriteString(linea)
		incluidas++
	}

	if omitidas := len(entries) - incluidas; omitidas > 0 {
		fmt.Fprintf(&b, domain.CompactionOmittedNoteFormat+"\n", omitidas)
	}

	return b.String(), nil
}

// formatCompactionEntry renderiza una memoria como viñeta compacta con
// puntero al detalle íntegro (mismo patrón que el modo índice de Builder).
func formatCompactionEntry(m domain.Memory) string {
	extracto := domain.Extract(m.Content, compactionEntryExtractChars)
	return fmt.Sprintf("- [%s] **%s**: %s (get_memory %d)\n", m.Type, m.Title, extracto, m.ID)
}

// prioritizeActionableFirst reordena las memorias de la sesión: decisiones y
// bugfixes primero, el resto después — cada grupo conserva su orden relativo
// original (de la más reciente a la más antigua, tal como lo entrega
// SessionMemoryLister). FR-006: prioriza lo accionable ante un recorte por
// presupuesto.
func prioritizeActionableFirst(mems []domain.Memory) []domain.Memory {
	out := make([]domain.Memory, 0, len(mems))
	var resto []domain.Memory
	for _, m := range mems {
		if m.Type == domain.Decision || m.Type == domain.Bugfix {
			out = append(out, m)
		} else {
			resto = append(resto, m)
		}
	}
	return append(out, resto...)
}
