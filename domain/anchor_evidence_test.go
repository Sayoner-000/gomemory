package domain

import "testing"

func TestNormalizeAnchor(t *testing.T) {
	for _, tc := range []struct {
		path, root, want string
		ok               bool
	}{
		{"", "/r", "", false}, {"/otro/x.go", "/r", "", false}, {"/r/a/b.go", "/r", "a/b.go", true},
		{"a/b.go", "/r", "a/b.go", true}, {"./a/b.go", "/r", "a/b.go", true}, {"../x.go", "/r", "", false},
	} {
		got, ok := NormalizeAnchor(tc.path, tc.root)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("NormalizeAnchor(%q) = (%q,%t)", tc.path, got, ok)
		}
	}
}

func TestGradeAnchor(t *testing.T) {
	if got := GradeAnchor("a/b.go", StatFile, nil); got.Grade != AnchorLive {
		t.Fatal(got)
	}
	if got := GradeAnchor("a/b.go", StatOther, nil); got.Grade != AnchorUnverifiable {
		t.Fatal(got)
	}
	if got := GradeAnchor("a/b.go", StatMissing, []string{"z/b.go"}); got.Grade != AnchorMoved || got.Candidate != "z/b.go" {
		t.Fatal(got)
	}
	if got := GradeAnchor("a/b.go", StatMissing, []string{"z/b.go", "y/b.go"}); got.Grade != AnchorMovedAmbiguous || got.Matches != 2 {
		t.Fatal(got)
	}
	if got := GradeAnchor("a/b.go", StatMissing, nil); got.Grade != AnchorOrphanCandidate {
		t.Fatal(got)
	}
	if got := GradeAnchor("a/b.go", StatMissing, []string{"a/b.go"}); got.Grade != AnchorOrphanCandidate {
		t.Fatal(got)
	}
}
