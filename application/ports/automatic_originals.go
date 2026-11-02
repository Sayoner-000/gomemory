package ports

import "context"

// AutomaticOriginalScope uses the host's stable session identity, never the
// transient memory session created by a hook subprocess.
type AutomaticOriginalScope struct {
	Project   string
	Agent     string
	SessionID string
}

// AutomaticOriginalGrantsRepository persists access separately from original
// content, preserving project-scoped manual retrieval and content deduplication.
type AutomaticOriginalGrantsRepository interface {
	Grant(ctx context.Context, scope AutomaticOriginalScope, refs []string) error
	Allows(ctx context.Context, scope AutomaticOriginalScope, ref string) (bool, error)
}
