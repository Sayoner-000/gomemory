package persistence

import (
	"database/sql"

	"mem/application/ports"
	"mem/domain"
)

type MemoryRepository struct {
	db *sql.DB
}

func NewMemoryRepository(db *sql.DB) ports.MemoryRepository {
	return &MemoryRepository{db: db}
}

func (r *MemoryRepository) Insert(m *domain.Memory) (int64, error) {
	return InsertMemory(r.db, m)
}

func (r *MemoryRepository) List(project string, limit int) ([]domain.Memory, error) {
	return ListMemories(r.db, project, limit)
}

func (r *MemoryRepository) Search(project, query string, limit int) ([]domain.Memory, error) {
	return SearchMemories(r.db, project, query, limit)
}

func (r *MemoryRepository) Delete(project string, id int64) (bool, error) {
	return DeleteMemory(r.db, project, id)
}

var _ ports.MemoryRepository = (*MemoryRepository)(nil)

type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) ports.SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Start(project string) (*domain.Session, error) {
	return StartSession(r.db, project)
}

func (r *SessionRepository) End(id, summary string) error {
	return EndSession(r.db, id, summary)
}

func (r *SessionRepository) Active(project string) (*domain.Session, error) {
	return ActiveSession(r.db, project)
}

func (r *SessionRepository) Recent(project string, limit int) ([]domain.Session, error) {
	return RecentSessions(r.db, project, limit)
}

var _ ports.SessionRepository = (*SessionRepository)(nil)

type RelationRepository struct {
	db *sql.DB
}

func NewRelationRepository(db *sql.DB) ports.RelationRepository {
	return &RelationRepository{db: db}
}

func (r *RelationRepository) Insert(rel *domain.Relation) (int64, error) {
	return InsertRelation(r.db, rel)
}

func (r *RelationRepository) Update(id int64, relation domain.RelationType, confidence float64, reasoning string) error {
	return UpdateRelation(r.db, id, relation, confidence, reasoning)
}

func (r *RelationRepository) Get(project string, id int64) (*domain.Relation, error) {
	return GetRelation(r.db, project, id)
}

func (r *RelationRepository) GetByPair(project string, memIDA, memIDB int64) (*domain.Relation, error) {
	return GetRelationByPair(r.db, project, memIDA, memIDB)
}

func (r *RelationRepository) List(project string, limit int) ([]domain.Relation, error) {
	return ListRelations(r.db, project, limit)
}

var _ ports.RelationRepository = (*RelationRepository)(nil)

type SettingsRepository struct{}

func NewSettingsRepository() ports.SettingsRepository {
	return &SettingsRepository{}
}

func (r *SettingsRepository) Read(root string) ports.SettingsData {
	s := ReadSettings(root)
	return ports.SettingsData{
		AutoApprove:      s.AutoApprove,
		AutoApproveTools: s.AutoApproveTools,
	}
}

func (r *SettingsRepository) Write(root string, s ports.SettingsData) error {
	return WriteSettings(root, Settings{
		AutoApprove:      s.AutoApprove,
		AutoApproveTools: s.AutoApproveTools,
	})
}

func (r *SettingsRepository) ApplyAutoApprove(root string, s ports.SettingsData) {
	ApplyAutoApprove(root, Settings{
		AutoApprove:      s.AutoApprove,
		AutoApproveTools: s.AutoApproveTools,
	})
}

var _ ports.SettingsRepository = (*SettingsRepository)(nil)

type ProjectRepository struct{}

func NewProjectRepository() ports.ProjectRepository {
	return &ProjectRepository{}
}

func (r *ProjectRepository) FindRoot() (string, error) {
	return FindRoot()
}

func (r *ProjectRepository) Init(root string) error {
	if err := EnsureDir(root); err != nil {
		return err
	}
	db, err := Open(root)
	if err != nil {
		return err
	}
	return db.Close()
}

func (r *ProjectRepository) EnsureDir(root string) error {
	return EnsureDir(root)
}

func (r *ProjectRepository) MemDir() string {
	return MemDir
}

func (r *ProjectRepository) DbPath(root string) string {
	return DbPath(root)
}

var _ ports.ProjectRepository = (*ProjectRepository)(nil)

type MaintenanceRepository struct {
	db     *sql.DB
	dbPath string
}

func NewMaintenanceRepository(db *sql.DB, dbPath string) ports.MaintenanceRepository {
	return &MaintenanceRepository{db: db, dbPath: dbPath}
}

func (r *MaintenanceRepository) Stats(project string) (ports.StorageStats, error) {
	projectCount, totalCount, err := StatsQuery(r.db, r.dbPath, project)
	if err != nil {
		return ports.StorageStats{}, err
	}
	size, err := FileSize(r.dbPath)
	if err != nil {
		return ports.StorageStats{}, err
	}
	return ports.StorageStats{
		ProjectMemoryCount: projectCount,
		TotalMemoryCount:   totalCount,
		FileSizeBytes:      size,
	}, nil
}

func (r *MaintenanceRepository) Purge(filter ports.PurgeFilter) (int64, error) {
	return PurgeMemories(r.db, filter)
}

func (r *MaintenanceRepository) Compact() (int64, int64, error) {
	return CompactDB(r.db, r.dbPath)
}

var _ ports.MaintenanceRepository = (*MaintenanceRepository)(nil)
