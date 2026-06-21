package domain

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
