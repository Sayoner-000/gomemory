// Package clock contiene la implementación real de ports.ClockPort.
package clock

import "time"

// SystemClock devuelve la hora del sistema en UTC.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }
