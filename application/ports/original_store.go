package ports

import (
	"context"
	"time"
)

// OriginalMeta describe un original guardado por el motor nativo de compresión
// (feature 033, data-model.md). No incluye el contenido.
type OriginalMeta struct {
	Ref          string
	Project      string
	ContentType  string
	Compressor   string
	RawBytes     int
	CreatedAt    time.Time
	LastAccessAt time.Time
	ExpiresAt    time.Time
}

// OriginalStoreRepository guarda los bloques originales que el motor comprimió
// con pérdida, para devolverlos íntegros por su referencia (FR-012, FR-013).
// Deduplica por huella: dos contenidos iguales comparten una sola fila.
type OriginalStoreRepository interface {
	// Put guarda content y devuelve su referencia: normalmente los 12 primeros
	// hex de su sha256, o 16 si esos 12 ya pertenecen a otro contenido.
	Put(ctx context.Context, project, content, contentType, compressor string) (ref string, err error)
	// Get devuelve el original exacto. found=false si no existe o caducó; un
	// Get válido renueva la caducidad.
	Get(ctx context.Context, ref string) (content string, meta OriginalMeta, found bool, err error)
	// Purge borra los caducados y aplica el tope de espacio (LRU).
	Purge(ctx context.Context) (removed int, freedBytes int64, err error)
	// Usage devuelve el espacio ocupado (bytes comprimidos) y el número de refs.
	Usage(ctx context.Context) (bytes int64, count int, err error)
}
