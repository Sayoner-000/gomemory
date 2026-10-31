package domain

import "testing"

func TestPinnedDocs_IncluyeBaseline(t *testing.T) {
	d, ok := PinnedDocByAlias("baseline")
	if !ok || d.TopicKey != TopicUniversalBaseline || d.Template != "universal-agent-instructions.md" {
		t.Fatalf("baseline = %+v, %v", d, ok)
	}
}
