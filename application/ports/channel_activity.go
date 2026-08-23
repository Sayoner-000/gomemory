package ports

import "time"

// ChannelActivityLog registra si un canal de inyección sigue vivo.
//
// Es un puerto porque la decisión de reportar un canal como inactivo vive en la
// capa de aplicación, que no importa infraestructura (constitución, principio I).
type ChannelActivityLog interface {
	// RecordFired anota que el canal acaba de ejercerse.
	RecordFired(agent, scope, kind string) error
	// RecordError anota un fallo al ejercerlo, conservando el último uso correcto.
	RecordError(agent, scope, kind, msg string) error
	// Last devuelve la última actividad conocida. ok=false: nunca se ejerció ni falló.
	Last(agent, scope, kind string) (fired time.Time, lastError string, ok bool)
	// SessionsSince cuenta las sesiones abiertas desde un instante, para
	// distinguir inactividad por falta de trabajo de un canal que no responde.
	SessionsSince(since time.Time) int
}
