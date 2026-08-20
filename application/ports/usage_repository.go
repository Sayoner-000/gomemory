package ports

import "mem/domain"

// UsageRepository persiste y consulta domain.UsageRecord. Sigue el manejo de
// errores del proyecto: la ausencia de filas devuelve un slice vacío, nunca un
// error de "no encontrado".
type UsageRepository interface {
	// Record inserta una emisión de contexto medida.
	Record(rec domain.UsageRecord) error
	// BySession devuelve los registros de una sesión. Vacío, no error, si no
	// hay ninguno.
	BySession(project, sessionID string) ([]domain.UsageRecord, error)
	// Sessions devuelve los identificadores de sesión con registros, del más
	// reciente al más antiguo.
	Sessions(project string, limit int) ([]string, error)
	// Totals devuelve todos los registros del proyecto, para el ámbito
	// "todas las sesiones".
	Totals(project string) ([]domain.UsageRecord, error)
}
