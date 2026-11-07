package ports

import "mem/domain"

// DecisionHistory guarda las fichas del historial de decisiones (HCD-046):
// una memoria por unidad HCD o por auditoría, enlazada con los archivos que
// cita para que el contexto de edición la encuentre.
type DecisionHistory interface {
	// UpsertDigest crea o actualiza la memoria por su TopicKey y deja files
	// como sus únicos archivos enlazados. inert la guarda sin sinapsis ni
	// exportación ADR (backfill). Devuelve el id de la memoria.
	UpsertDigest(m *domain.Memory, files []string, inert bool) (int64, error)
}

// HistoryReader lee las fichas del historial para mem hcd history y la tool
// MCP hcd_history.
type HistoryReader interface {
	// Digests devuelve todas las fichas del proyecto (unidades y auditorías).
	Digests(project string) ([]domain.Memory, error)
	// DigestsByFile devuelve las fichas enlazadas al archivo path.
	DigestsByFile(project, path string) ([]domain.Memory, error)
}

// HistoryMemories es la parte de la memoria que el backfill del historial
// necesita: comparar fichas existentes y retirar las memorias sueltas de pasos.
type HistoryMemories interface {
	ListAll(project string) ([]domain.Memory, error)
	ByTopicKey(project, topicKey string) (*domain.Memory, error)
	DeleteMany(project string, ids []int64) (int64, error)
}

// AuditSource lee los informes de auditoría del proyecto (docs/audits).
type AuditSource interface {
	// ListAudits devuelve las rutas relativas de los informes .md.
	ListAudits() ([]string, error)
	ReadAudit(rel string) ([]byte, error)
}
