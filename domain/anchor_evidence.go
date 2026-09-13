package domain

import (
	"path"
	"path/filepath"
	"strings"
)

type AnchorGrade string

const (
	AnchorLive            AnchorGrade = "live"
	AnchorMoved           AnchorGrade = "moved"
	AnchorMovedAmbiguous  AnchorGrade = "moved_ambiguous"
	AnchorOrphanCandidate AnchorGrade = "orphan_candidate"
	AnchorUnverifiable    AnchorGrade = "unverifiable"
)

type AnchorStat string

const (
	StatMissing AnchorStat = "missing"
	StatFile    AnchorStat = "file"
	StatOther   AnchorStat = "other"
)

// AnchorEvidence es una hipótesis de mantenimiento, nunca una orden de borrado.
type AnchorEvidence struct {
	Grade           AnchorGrade
	Path, Candidate string
	Matches         int
}

func NormalizeAnchor(value, root string) (string, bool) {
	if strings.TrimSpace(value) == "" {
		return "", false
	}
	if filepath.IsAbs(value) {
		rel, err := filepath.Rel(root, value)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", false
		}
		value = rel
	}
	clean := path.Clean(filepath.ToSlash(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

func GradeAnchor(rel string, stat AnchorStat, indexed []string) AnchorEvidence {
	evidence := AnchorEvidence{Grade: AnchorUnverifiable, Path: rel}
	if stat == StatFile {
		evidence.Grade = AnchorLive
		return evidence
	}
	if stat != StatMissing {
		return evidence
	}
	base := path.Base(rel)
	for _, candidate := range indexed {
		candidate = path.Clean(candidate)
		if candidate != rel && path.Base(candidate) == base {
			evidence.Matches++
			evidence.Candidate = candidate
		}
	}
	switch evidence.Matches {
	case 0:
		evidence.Grade = AnchorOrphanCandidate
	case 1:
		evidence.Grade = AnchorMoved
	default:
		evidence.Grade = AnchorMovedAmbiguous
		evidence.Candidate = ""
	}
	return evidence
}
