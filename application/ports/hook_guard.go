package ports

import "mem/domain"

// HookGuardRecorder cuenta los eventos de las protecciones de hooks (feature
// 035, FR-028/029): invocaciones duplicadas descartadas, salidas recortadas por
// presupuesto y salidas excluidas de la compresión. Sin ese rastro, una
// regresión (volver a truncar, volver a duplicar) solo se descubría cuando el
// agente ya se había equivocado. Lo implementa el mismo repositorio concreto
// que ChannelActivityLog, sin ampliar ese puerto.
type HookGuardRecorder interface {
	// RecordGuard suma un evento del día. detail nunca lleva contenido del
	// usuario: solo cifras o el nombre de una herramienta.
	RecordGuard(agent, kind, detail string) error
	// GuardSince agrega los eventos de los últimos days días por agente y tipo.
	GuardSince(days int) ([]domain.GuardCount, error)
}
