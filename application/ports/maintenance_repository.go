package ports

// StorageStats es un snapshot de tamaño/cantidad del almacén de memoria,
// usado para decidir y verificar el efecto de purgar/compactar.
type StorageStats struct {
	ProjectMemoryCount int64
	TotalMemoryCount   int64
	FileSizeBytes      int64
}

// PurgeFilter acota qué memorias borra una operación de purga o garbage
// collection. Project se ignora si All es true.
type PurgeFilter struct {
	Project       string
	All           bool
	Type          string
	OlderThanDays int
}

// MaintenanceRepository agrupa las operaciones administrativas sobre el
// almacén de memoria (purga, compactación, estadísticas). Se mantiene
// separado de MemoryRepository porque mezcla borrado masivo y reclamo de
// espacio en disco, operaciones que ningún consumidor de lectura/escritura
// normal (MCP, TUI de guardado, comandos cotidianos) debería poder invocar
// por accidente.
type MaintenanceRepository interface {
	Stats(project string) (StorageStats, error)
	Purge(filter PurgeFilter) (deleted int64, err error)
	Compact() (beforeBytes int64, afterBytes int64, err error)
}
