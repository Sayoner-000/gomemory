package domain

import "time"

// Envoltorios del reloj usados SOLO por las pruebas de rendimiento. El código de
// producción de la política no consulta el reloj: hacerlo rompería su pureza.
func testingNow() time.Time                  { return time.Now() }
func testingSince(t time.Time) time.Duration { return time.Since(t) }
