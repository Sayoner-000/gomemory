package ports

import "mem/domain"

// OctopusRepository persiste las decisiones de enrutamiento y los reportes de
// ejecución que devuelve el runtime (feature 027).
//
// Es fire-and-forget por diseño: NINGÚN método devuelve error que deba detener
// un flujo. Medir jamás puede impedir enrutar ni ejecutar — mismo criterio que
// UsageRecorder. Un repositorio nil es válido en cualquier dependencia que lo
// reciba: sin él, Octopus funciona igual, solo que sin memoria de lo ocurrido
// (INV-AAR-015).
type OctopusRepository interface {
	// RecordDecision guarda una decisión recién emitida.
	RecordDecision(project, planID string, class domain.TaskClass, d domain.RouteDecision)
	// RecordReport completa la fila de una decisión con el consumo real. Un
	// reporte para una tarea sin decisión previa se ignora sin error.
	RecordReport(project string, r domain.ExecutionReport)
	// Evidence devuelve la evidencia agregada por clase de tarea. Un proyecto
	// sin historial devuelve un mapa vacío, no un error.
	Evidence(project string) map[domain.TaskClass]*domain.ClassEvidence
	// Stats devuelve los agregados de telemetría del proyecto.
	Stats(project string) domain.RoutingStats
	// History devuelve las últimas decisiones, opcionalmente filtradas por clase.
	History(project string, class domain.TaskClass, limit int) []domain.ExecutionRecord
}
