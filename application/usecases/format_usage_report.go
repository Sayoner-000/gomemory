package usecases

import (
	"fmt"
	"strings"

	"mem/domain"
)

// FormatUsageReport renderiza un domain.UsageReport en texto plano legible
// por personas — traducción de la forma legible por máquina que manda el
// contrato, nunca al revés (feature 020, FR-012, contracts/usage-report.md).
//
// Vive en application/usecases (no en un adaptador primario) para que tanto
// la línea de comandos como la interfaz interactiva lo reutilicen sin
// duplicar el formato: `adapters/primary/cli` ya importa
// `adapters/primary/tui` (para lanzarla), así que tui no puede importar cli
// sin crear un ciclo. Mismo patrón que formatCodeArchitecture en
// build_context.go, que existe por la misma razón (feature 018).
//
// scope es "session", "all" o "empty" (los mismos tres valores que scope en
// contracts/usage-report.md) — se recibe como string plano en vez de un tipo
// del adaptador CLI, para no acoplar esta función a ningún canal concreto.
func FormatUsageReport(report domain.UsageReport, scope string) string {
	var b strings.Builder

	header := fmt.Sprintf("Uso de contexto — proyecto %s", report.Project)
	switch {
	case scope == "session" && report.SessionID != "":
		header += fmt.Sprintf(" · sesión %s", report.SessionID)
	case scope == "all":
		header += " · todas las sesiones"
	}
	b.WriteString(header + "\n")
	b.WriteString("Conteo aproximado neutral (~4 caracteres por token). Las cifras son comparables\n")
	b.WriteString("contra sí mismas, no contra la facturación de ningún proveedor.\n\n")

	if scope == "empty" || report.Calls == 0 {
		b.WriteString("Sin actividad registrada todavía.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "Llamadas:              %d\n", report.Calls)
	fmt.Fprintf(&b, "Línea base:        %d tokens\n", report.BaselineTokens)
	fmt.Fprintf(&b, "Emitido:           %d tokens\n", report.EmittedTokens)
	fmt.Fprintf(&b, "Ahorro:            %d tokens  (%.2f%%)\n", report.Saved(), report.ReductionRatio()*100)

	if report.SchemaOperations > 0 {
		fmt.Fprintf(&b, "\nDescriptores publicados: %d tokens en %d operaciones\n", report.SchemaTokens, report.SchemaOperations)
	}

	if ratio, ok := report.WindowRatio(); ok {
		fmt.Fprintf(&b, "\nHuella evitada:    %.2f%% de una ventana de %d tokens  (estimado)\n", ratio*100, report.WindowTokens)
	}

	if len(report.ByOperation) > 0 {
		b.WriteString("\nPor operación\n")
		for _, bucket := range report.ByOperation {
			fmt.Fprintf(&b, "  %-18s %d llamada(s)   %d → %d\n", bucket.Key, bucket.Calls, bucket.BaselineTokens, bucket.EmittedTokens)
		}
	}
	if len(report.ByChannel) > 0 {
		b.WriteString("\nPor canal\n")
		for _, bucket := range report.ByChannel {
			fmt.Fprintf(&b, "  %-18s %d llamada(s)   %d → %d\n", bucket.Key, bucket.Calls, bucket.BaselineTokens, bucket.EmittedTokens)
		}
	}

	return b.String()
}
