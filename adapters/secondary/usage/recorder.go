// Package usage contiene el adaptador secundario que traduce las emisiones
// medidas de gomemory (ports.UsageRecorder) a filas persistidas
// (ports.UsageRepository), etiquetadas con el canal por el que salieron.
package usage

import (
	"mem/application/ports"
	"mem/domain"
)

// recorder implementa ports.UsageRecorder. La etiqueta de canal y el proyecto
// se fijan al construirlo — nunca viajan en Record() — porque cada proceso de
// gomemory es exactamente un canal (mcp/cli/tui). Esto es lo que permite que
// añadir un canal nuevo sea "construir un recorder con otra etiqueta" y nada
// más (feature 020, FR-017).
type recorder struct {
	repo    ports.UsageRepository
	project string
	channel string
	// session resuelve el identificador de sesión activa EN EL MOMENTO de
	// registrar, no en el momento de construir: la sesión puede empezar
	// después de que el recorder ya exista (p. ej. el servidor MCP arranca
	// antes de auto-iniciar sesión).
	session func() string
}

// NewRecorder construye un ports.UsageRecorder atado a un canal. channel es
// una etiqueta libre: NO se valida contra ninguna lista (FR-004).
func NewRecorder(repo ports.UsageRepository, project, channel string, session func() string) ports.UsageRecorder {
	return &recorder{repo: repo, project: project, channel: channel, session: session}
}

// Record es fire-and-forget: un fallo de persistencia se descarta en
// silencio (FR-006). Medir nunca puede impedir emitir.
func (r *recorder) Record(operation string, baselineTokens, emittedTokens int) {
	if r == nil || r.repo == nil {
		return
	}
	sessionID := ""
	if r.session != nil {
		sessionID = r.session()
	}
	rec := domain.UsageRecord{
		Project:        r.project,
		SessionID:      sessionID,
		Operation:      operation,
		Channel:        r.channel,
		BaselineTokens: baselineTokens,
		EmittedTokens:  emittedTokens,
	}
	_ = r.repo.Record(rec)
}
