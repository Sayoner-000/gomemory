package ports

import "mem/domain"

// ContextMemorySummaryLister is optional; it does not extend MemoryRepository.
// Results cover the whole project in ascending ID order. Consumers must fall
// back to the full-history port if summaries cannot be read.
type ContextMemorySummaryLister interface {
	ListContextSummaries(project string) ([]domain.ContextMemorySummary, error)
}
