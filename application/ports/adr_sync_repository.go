package ports

import "mem/domain"

// ADRSyncRepository persiste el estado de sincronización memoria↔bloque ADR
// (feature 010, Historia 2). Ver domain.ADRSyncRecord.
type ADRSyncRepository interface {
	Insert(r *domain.ADRSyncRecord) (int64, error)
	GetByMemory(project string, memoryID int64) (*domain.ADRSyncRecord, error)
	GetByBlockKey(project, provider, blockKey string) (*domain.ADRSyncRecord, error)
	UpdateStatus(id int64, status domain.SyncStatus, contentHash string) error
	// ListByProject devuelve todos los registros del proyecto (más recientes
	// primero) — usado por `mem adr-sync status`.
	ListByProject(project string) ([]domain.ADRSyncRecord, error)
}
