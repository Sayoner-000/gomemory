package persistence

import (
	"fmt"
	"mem/application/ports"
	"mem/domain"
)

var _ ports.ContextMemorySummaryLister = (*MemoryRepository)(nil)

// ListContextSummaries keeps full-history context metadata without materializing
// bodies of titled memories. Empty titles still require the legacy body fallback.
func (r *MemoryRepository) ListContextSummaries(project string) ([]domain.ContextMemorySummary, error) {
	rows, err := r.db.Query(`SELECT id, type, COALESCE(session_id,''), COALESCE(title,''), COALESCE(filepath,''), CASE WHEN COALESCE(title,'') = '' THEN content ELSE '' END FROM memories WHERE project = ? ORDER BY id ASC`, project)
	if err != nil {
		return nil, fmt.Errorf("list context summaries: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]domain.ContextMemorySummary, 0)
	var fallback []int
	for rows.Next() {
		var m domain.Memory
		var typ string
		if err := rows.Scan(&m.ID, &typ, &m.SessionID, &m.Title, &m.Filepath, &m.Content); err != nil {
			return nil, fmt.Errorf("scan context summary: %w", err)
		}
		hadTitle := m.Title != ""
		finishScannedMemory(&m, typ)
		s := domain.ContextMemorySummary{ID: m.ID, Type: m.Type, SessionID: m.SessionID, Filepath: m.Filepath, DisplayTitle: m.Title}
		if m.Title == "" {
			if hadTitle {
				fallback = append(fallback, len(out))
			} else {
				s.DisplayTitle = contextFallbackTitle(m.Content)
			}
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read context summaries: %w", err)
	}
	// Close before fallback queries: the connection pool may contain one connection.
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close context summaries: %w", err)
	}
	for _, i := range fallback {
		var content string
		if err := r.db.QueryRow(`SELECT content FROM memories WHERE project = ? AND id = ?`, project, out[i].ID).Scan(&content); err != nil {
			return nil, fmt.Errorf("read context title fallback: %w", err)
		}
		out[i].DisplayTitle = contextFallbackTitle(domain.StripTerminalControls(content))
	}
	return out, nil
}

func contextFallbackTitle(content string) string {
	r := []rune(content)
	if len(r) > 60 {
		return string(r[:57]) + "..."
	}
	return string(r)
}
