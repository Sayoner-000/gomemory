package domain

import (
	"fmt"
	"strings"
	"time"
)

type TargetType string

const (
	TargetCommit         TargetType = "commit"
	TargetDiff           TargetType = "diff"
	TargetFileSet        TargetType = "file_set"
	TargetSpec           TargetType = "spec"
	TargetPlan           TargetType = "plan"
	TargetConfig         TargetType = "config"
	TargetArchitecture   TargetType = "architecture"
	TargetMigration      TargetType = "migration"
	TargetAPIContract    TargetType = "api_contract"
	TargetImplementation TargetType = "implementation"
)

type Target struct {
	Type      TargetType
	Revision  string
	Scope     []string
	CreatedAt time.Time
	digest    string
}

func NewTarget(targetType TargetType, revision, digest string, scope []string) (Target, error) {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return Target{}, fmt.Errorf("target digest is required")
	}
	return Target{
		Type:      targetType,
		Revision:  revision,
		Scope:     append([]string(nil), scope...),
		CreatedAt: time.Now(),
		digest:    digest,
	}, nil
}

func (t Target) Digest() string {
	return t.digest
}

func (t Target) ValidateDigest(candidate string) error {
	if strings.TrimSpace(candidate) != t.digest {
		return fmt.Errorf("target changed")
	}
	return nil
}

type ReviewStatus string

const (
	ReviewFrozen            ReviewStatus = "frozen"
	ReviewAwaitingReviewers ReviewStatus = "awaiting_reviewers"
	ReviewConsensusReady    ReviewStatus = "consensus_ready"
	ReviewFixing            ReviewStatus = "fixing"
	ReviewRejudging         ReviewStatus = "rejudging"
	ReviewApproved          ReviewStatus = "approved"
	ReviewEscalated         ReviewStatus = "escalated"
	ReviewIncomplete        ReviewStatus = "incomplete"
)

func (s ReviewStatus) Valid() bool {
	switch s {
	case ReviewFrozen, ReviewAwaitingReviewers, ReviewConsensusReady, ReviewFixing,
		ReviewRejudging, ReviewApproved, ReviewEscalated, ReviewIncomplete:
		return true
	default:
		return false
	}
}

func (s ReviewStatus) Terminal() bool {
	return s == ReviewApproved || s == ReviewEscalated || s == ReviewIncomplete
}

func (s ReviewStatus) CanTransitionTo(next ReviewStatus) bool {
	if s.Terminal() || !next.Valid() {
		return false
	}
	switch s {
	case ReviewFrozen:
		return next == ReviewAwaitingReviewers
	case ReviewAwaitingReviewers:
		return next == ReviewConsensusReady || next == ReviewIncomplete
	case ReviewConsensusReady:
		return next == ReviewFixing || next == ReviewApproved || next == ReviewEscalated || next == ReviewIncomplete
	case ReviewFixing:
		return next == ReviewRejudging || next == ReviewEscalated || next == ReviewIncomplete
	case ReviewRejudging:
		return next == ReviewFixing || next == ReviewApproved || next == ReviewEscalated || next == ReviewIncomplete
	default:
		return false
	}
}

type IndependenceLevel string

const (
	IndependenceFull     IndependenceLevel = "full"
	IndependenceDegraded IndependenceLevel = "degraded"
)

type Review struct {
	ID                 string
	Project            string
	Target             Target
	MaxFixRounds       int
	AutoFixSeverities  []Severity
	IndependenceLevel  IndependenceLevel
	IndependenceReason string
	Round              int
	Status             ReviewStatus
	Verdict            Verdict
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
