package domain

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
