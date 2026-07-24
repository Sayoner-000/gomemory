package domain

// SyncOrigin indica de qué lado se originó un par memoria↔bloque ADR: evita
// bucles de resincronización (nunca reimportar lo exportado, nunca
// reexportar lo importado como si fuera nuevo).
type SyncOrigin string

const (
	SyncOriginGomemory SyncOrigin = "gomemory"
	SyncOriginProvider SyncOrigin = "provider"
)

// ValidSyncOrigin valida el origen; un valor desconocido cae a "provider"
// (el más conservador: no asumir que gomemory es dueño de algo que no
// reconoce, para no sobrescribirlo por error).
func ValidSyncOrigin(s string) SyncOrigin {
	switch SyncOrigin(s) {
	case SyncOriginGomemory, SyncOriginProvider:
		return SyncOrigin(s)
	default:
		return SyncOriginProvider
	}
}

// SyncStatus es el resultado del último intento de sincronización (en
// cualquier sentido) de un par memoria↔bloque.
type SyncStatus string

const (
	SyncStatusOK               SyncStatus = "ok"
	SyncStatusPending          SyncStatus = "pending"
	SyncStatusFailed           SyncStatus = "failed"
	SyncStatusConflictResolved SyncStatus = "conflict_resolved"
)

// ValidSyncStatus valida el status; un valor desconocido cae a "pending"
// (no asumir éxito sobre un dato que no se reconoce).
func ValidSyncStatus(s string) SyncStatus {
	switch SyncStatus(s) {
	case SyncStatusOK, SyncStatusPending, SyncStatusFailed, SyncStatusConflictResolved:
		return SyncStatus(s)
	default:
		return SyncStatusPending
	}
}

// ADRSyncRecord relaciona una memoria de gomemory (architecture/decision)
// con el bloque que le corresponde dentro del documento único de ADR del
// proveedor externo (ver ADRDocument). Uno de los dos identificadores
// (MemoryID o BlockKey) siempre está presente; MemoryID puede ser nil solo
// transitoriamente durante una importación en curso.
type ADRSyncRecord struct {
	ID           int64
	Project      string
	MemoryID     *int64
	Provider     string
	Section      string
	BlockKey     string
	Origin       SyncOrigin
	Status       SyncStatus
	ContentHash  string
	LastSyncedAt string
	CreatedAt    string
}
