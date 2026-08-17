package usecases

import (
	"mem/application/ports"
	"mem/domain"
)

// BuildActivationReport compone el reporte de cobertura de los dos brazos
// (feature 019, Historia 3): recorre los canales que inspector reporta para
// root, ordena de forma determinista y separa las degradaciones declaradas
// (StateNotApplicable con motivo) del resto — que sí cuentan como problema si
// están rotos. `mem doctor` es el único consumidor de este caso de uso; el
// script de regresión y el reporte se alimentan de la MISMA fuente, así un
// agente nuevo aparece en ambos sin tocarlos (FR-A4, SC-A2).
func BuildActivationReport(inspector ports.ActivationInspector, root string) domain.CoverageReport {
	channels := inspector.Inspect(root)
	domain.SortChannels(channels)

	var degradations []string
	for _, c := range channels {
		if c.State == domain.StateNotApplicable && c.Detail != "" {
			degradations = append(degradations, c.Agent+" ("+string(c.Kind)+"): "+c.Detail)
		}
	}

	return domain.CoverageReport{Channels: channels, Degradations: degradations}
}
