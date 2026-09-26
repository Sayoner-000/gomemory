package domain

import "time"

// UpdateCheck es la caché de la última versión publicada (feature 034, FR-028).
// Vive en el almacén global: es del usuario, no de un proyecto.
type UpdateCheck struct {
	Latest    string    `json:"latest"`
	CheckedAt time.Time `json:"checked_at"`
	ETag      string    `json:"etag"`
	// LastError es el último fallo de consulta. Lo muestra doctor; nunca se
	// convierte en aviso.
	LastError string `json:"last_error,omitempty"`
}

// Stale indica si la caché superó UpdateCheckTTL.
func (c UpdateCheck) Stale(now time.Time) bool {
	return now.Sub(c.CheckedAt) > UpdateCheckTTL
}

// RecordSuccess anota una consulta con éxito. latest vacío es un 304: la
// versión conocida sigue vigente.
func (c UpdateCheck) RecordSuccess(now time.Time, latest, etag string) UpdateCheck {
	if latest != "" {
		c.Latest = latest
	}
	if etag != "" {
		c.ETag = etag
	}
	c.CheckedAt, c.LastError = now, ""
	return c
}

// RecordFailure anota un fallo: conserva lo conocido y solo mueve el momento
// de la consulta, para reintentar pasado el intervalo y no en cada sesión.
func (c UpdateCheck) RecordFailure(now time.Time, err error) UpdateCheck {
	c.CheckedAt, c.LastError = now, err.Error()
	return c
}

// UpdateNotice registra qué versión se anunció y en qué sesión (FR-030: como
// mucho una vez por versión y sesión).
type UpdateNotice struct {
	Version string `json:"version"`
	Session string `json:"session"`
}
