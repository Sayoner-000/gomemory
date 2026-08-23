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

	// La lista se deduplica porque una degradación es una propiedad del agente y
	// del tipo de canal, no del ámbito: la misma limitación declarada en proyecto
	// y en usuario es una sola, y repetirla no aporta información. Los canales sí
	// conservan una fila por ámbito, que es donde el ámbito distingue el estado.
	var degradations []string
	vistas := map[string]bool{}
	for _, c := range channels {
		if c.State != domain.StateNotApplicable || c.Detail == "" {
			continue
		}
		linea := c.Agent + " (" + string(c.Kind) + "): " + c.Detail
		if vistas[linea] {
			continue
		}
		vistas[linea] = true
		degradations = append(degradations, linea)
	}

	return domain.CoverageReport{Channels: channels, Degradations: degradations}
}
