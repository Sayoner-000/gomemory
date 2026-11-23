package usecases_test

import (
	"mem/application/ports"
	"mem/domain"
	"testing"
)

type hcd059SummaryStub struct{}

func (hcd059SummaryStub) ListContextSummaries(string) ([]domain.ContextMemorySummary, error) {
	return []domain.ContextMemorySummary{{ID: 1, Type: domain.Learning, SessionID: "session", DisplayTitle: "title", Filepath: "file.go"}}, nil
}
func TestHCD059SummaryPortContract(t *testing.T) {
	var p ports.ContextMemorySummaryLister = hcd059SummaryStub{}
	rows, err := p.ListContextSummaries("perf")
	if err != nil || len(rows) != 1 || rows[0].DisplayTitle != "title" {
		t.Fatal("invalid summary port")
	}
}
