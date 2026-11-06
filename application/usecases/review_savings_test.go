package usecases

import (
	"errors"
	"testing"

	"mem/domain"
)

// HCD-035: el ahorro de una revisión enlazada se mide con el contenido del
// alcance en la revisión del objetivo y el TokenCounter.

type fakeRevisionFiles struct {
	files     map[string]string
	err       error
	rev       string
	gotScope  []string
	callCount int
}

func (f *fakeRevisionFiles) TextFilesAt(rev string, scope []string) (map[string]string, error) {
	f.callCount++
	f.rev, f.gotScope = rev, scope
	return f.files, f.err
}

type runeCounter struct{}

func (runeCounter) Count(s string) int { return len([]rune(s)) }

func TestReviewSinceSavings(t *testing.T) {
	review := &domain.Review{SinceReviewID: "acr_prev", DeltaSource: domain.DeltaSourceGit, DeltaScope: []string{"b.go"},
		Target: domain.Target{Revision: "abc", Scope: nil}}
	files := &fakeRevisionFiles{files: map[string]string{"a.go": "aaaaaaaaaa", "b.go": "bb"}}

	got := ReviewSinceSavings(files, runeCounter{}, review, "xyz")
	want := domain.SinceSavings{Measurable: true, ScopeFiles: 2, DeltaFiles: 1, ScopeTokens: 12, DeltaTokens: 2, InheritedTokens: 3, AvoidedTokens: 7}
	if got != want || files.rev != "abc" || len(files.gotScope) != 1 || files.gotScope[0] != "." {
		t.Fatalf("got %+v (rev %q scope %v), want %+v", got, files.rev, files.gotScope, want)
	}

	// Sin delta de git, sin enlace o con fallo del port: no medible y con motivo.
	for name, c := range map[string]struct {
		review *domain.Review
		files  *fakeRevisionFiles
	}{
		"delta de scope": {&domain.Review{SinceReviewID: "p", DeltaSource: domain.DeltaSourceScope}, &fakeRevisionFiles{}},
		"sin enlace":     {&domain.Review{}, &fakeRevisionFiles{}},
		"port falla":     {review, &fakeRevisionFiles{err: errors.New("no es commit")}},
	} {
		s := ReviewSinceSavings(c.files, runeCounter{}, c.review, "")
		if s.Measurable || s.Reason == "" {
			t.Errorf("%s: %+v", name, s)
		}
	}
	if s := ReviewSinceSavings(nil, runeCounter{}, review, ""); s.Measurable || s.Reason == "" {
		t.Errorf("sin port: %+v", s)
	}
}
