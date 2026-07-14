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

func (r *MemoryRepository) ListAll(project string) ([]domain.Memory, error) {
	return ListAllMemories(r.db, project)
}

func (r *MemoryRepository) ImportMemory(m *domain.Memory) (int64, error) {
	return ImportMemory(r.db, m)
}

func (r *MemoryRepository) Search(project, query string, limit int) ([]domain.Memory, error) {
	return SearchMemories(r.db, project, query, limit)
}

func (r *MemoryRepository) Delete(project string, id int64) (bool, error) {
	return DeleteMemory(r.db, project, id)
}

func (r *MemoryRepository) SecondsSinceLastSave(project string) (int64, bool, error) {
	return SecondsSinceLastSave(r.db, project)
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

func (r *SessionRepository) SetLastPrompt(project, prompt string) error {
	return SetSessionLastPrompt(r.db, project, prompt)
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

func (r *RelationRepository) ListAll(project string) ([]domain.Relation, error) {
	return ListAllRelations(r.db, project)
}

func (r *RelationRepository) ImportRelation(rel *domain.Relation) (int64, error) {
	return ImportRelation(r.db, rel)
}

var _ ports.RelationRepository = (*RelationRepository)(nil)

type SettingsRepository struct{}

func NewSettingsRepository() ports.SettingsRepository {
	return &SettingsRepository{}
}

func (r *SettingsRepository) Read(root string) ports.SettingsData {
	s := ReadSettings(root)
	return ports.SettingsData{
		AutoApprove:       s.AutoApprove,
		AutoApproveTools:  s.AutoApproveTools,
		CodeGraphDisabled: s.CodeGraphDisabled,
		CodeGraphCommand:  s.CodeGraphCommand,
		Budget:            s.Budget,
		CompactThreshold:  s.CompactThreshold,
		DedupWindowDays:   s.DedupWindowDays,
	}
}

func (r *SettingsRepository) Write(root string, s ports.SettingsData) error {
	return WriteSettings(root, Settings{
		AutoApprove:       s.AutoApprove,
		AutoApproveTools:  s.AutoApproveTools,
		CodeGraphDisabled: s.CodeGraphDisabled,
		CodeGraphCommand:  s.CodeGraphCommand,
		Budget:            s.Budget,
		CompactThreshold:  s.CompactThreshold,
		DedupWindowDays:   s.DedupWindowDays,
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

func (r *ProjectRepository) Key(root string) string {
	return ProjectKey(root)
}

func (r *ProjectRepository) MigrateLegacy(root string, force bool) (bool, error) {
	return MigrateLegacy(root, force)
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

type CodeGraphRepository struct {
	db *sql.DB
}

func NewCodeGraphRepository(db *sql.DB) ports.CodeGraphRepository {
	return &CodeGraphRepository{db: db}
}

func (r *CodeGraphRepository) FileHashes(project string) (map[string]string, error) {
	return FileHashesQuery(r.db, project)
}

func (r *CodeGraphRepository) ReplaceFile(project, path, hash string, nodes []domain.CodeNode) ([]domain.CodeNode, error) {
	return ReplaceFileNodes(r.db, project, path, hash, nodes)
}

func (r *CodeGraphRepository) DeleteFile(project, path string) error {
	return DeleteCodeFile(r.db, project, path)
}

func (r *CodeGraphRepository) InsertEdges(project, srcPath string, edges []domain.CodeEdge) error {
	return InsertCodeEdges(r.db, project, srcPath, edges)
}

func (r *CodeGraphRepository) SearchNodes(project, query string, limit int) ([]domain.CodeNode, error) {
	return SearchCodeNodes(r.db, project, query, limit)
}

func (r *CodeGraphRepository) NodesByName(project, name string) ([]domain.CodeNode, error) {
	return NodesByName(r.db, project, name)
}

func (r *CodeGraphRepository) UpsertPackageNode(project, importPath string) (domain.CodeNode, error) {
	return UpsertPackageNode(r.db, project, importPath)
}

func (r *CodeGraphRepository) Neighbors(project string, nodeID int64, kind domain.CodeEdgeKind, direction string, depth int) ([]domain.CodeNode, []domain.CodeEdge, error) {
	return Neighbors(r.db, project, nodeID, kind, direction, depth)
}

func (r *CodeGraphRepository) Status(project string) (domain.GraphStatus, error) {
	return CodeGraphStatus(r.db, project)
}

var _ ports.CodeGraphRepository = (*CodeGraphRepository)(nil)
