package types

type MemoryType string

const (
	Learning     MemoryType = "learning"
	Decision     MemoryType = "decision"
	Architecture MemoryType = "architecture"
	Bugfix       MemoryType = "bugfix"
	Pattern      MemoryType = "pattern"
	Discovery    MemoryType = "discovery"
)

func ValidMemoryType(s string) MemoryType {
	switch MemoryType(s) {
	case Learning, Decision, Architecture, Bugfix, Pattern, Discovery:
		return MemoryType(s)
	default:
		return Learning
	}
}

type RelationType string

const (
	Related       RelationType = "related"
	Compatible    RelationType = "compatible"
	Scoped        RelationType = "scoped"
	ConflictsWith RelationType = "conflicts_with"
	Supersedes    RelationType = "supersedes"
	NotConflict   RelationType = "not_conflict"
)

func ValidRelationType(s string) RelationType {
	switch RelationType(s) {
	case Related, Compatible, Scoped, ConflictsWith, Supersedes, NotConflict:
		return RelationType(s)
	default:
		return Related
	}
}

type Memory struct {
	ID        int64      `json:"id"`
	Project   string     `json:"project"`
	SessionID string     `json:"session_id,omitempty"`
	Type      MemoryType `json:"type"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Filepath  string     `json:"filepath,omitempty"`
	CreatedAt string     `json:"created_at"`
	UpdatedAt string     `json:"updated_at"`
}

type Session struct {
	ID        string  `json:"id"`
	Project   string  `json:"project"`
	Summary   string  `json:"summary"`
	CreatedAt string  `json:"created_at"`
	EndedAt   *string `json:"ended_at,omitempty"`
}

type Relation struct {
	ID          int64        `json:"id"`
	Project     string       `json:"project"`
	MemoryIDA   int64        `json:"memory_id_a"`
	MemoryIDB   int64        `json:"memory_id_b"`
	Relation    RelationType `json:"relation"`
	Confidence  float64      `json:"confidence"`
	Reasoning   string       `json:"reasoning"`
	CreatedAt   string       `json:"created_at"`
}
