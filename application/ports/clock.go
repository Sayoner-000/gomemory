package ports

import "time"

// ClockPort da la hora actual. Se inyecta para que la caducidad de los
// originales y la ventana del ajuste adaptativo sean deterministas en pruebas
// (constitución §10). No hace I/O, así que no lleva context.Context.
type ClockPort interface {
	Now() time.Time
}
