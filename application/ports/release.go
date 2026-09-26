package ports

import "context"

// ReleasePort consulta las releases publicadas de gomemory (feature 034).
type ReleasePort interface {
	// Latest devuelve la última release estable. Con etag, un servidor que
	// responde "sin cambios" devuelve notModified=true y tag vacío.
	Latest(ctx context.Context, etag string) (tag, newEtag string, notModified bool, err error)
	// Checksum devuelve el SHA-256 publicado del asset en checksums.txt.
	Checksum(ctx context.Context, tag, asset string) (string, error)
}
