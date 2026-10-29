package ports

import (
	"context"

	"mem/domain"
)

type HCDRepository interface {
	NextGovernanceID(context.Context, string) (string, error)
	SaveException(context.Context, domain.Exception) error
	ListExceptions(context.Context, string) ([]domain.Exception, error)
	GetNormativeRevision(context.Context, string, string) (domain.NormativeRevision, error)
	ListUrgentRevisions(context.Context) ([]domain.NormativeRevision, error)
	SaveAdoption(context.Context, domain.PlaybookAdoption) error
	SaveExecution(context.Context, domain.Execution) error
	GetExecution(context.Context, string, string) (domain.Execution, error)
	ListExecutions(context.Context, string) ([]domain.Execution, error)
	SaveEvidence(context.Context, domain.Evidence) error
	ListEvidence(context.Context, string) ([]domain.Evidence, error)
	NextBlockID(context.Context) (string, error)
	SaveBlock(context.Context, domain.Block) error
	ListBlocks(context.Context, string) ([]domain.Block, error)
	ListHCDUnits(context.Context) ([]domain.Unit, error)
	ListApprovals(context.Context, string) ([]domain.Approval, error)
	LastVerificationRevision(context.Context, string) (string, error)
	HCDActivityAt(context.Context, string) (string, error)
	GetUnit(context.Context, string) (domain.Unit, error)
	CreateUnit(context.Context, domain.Unit) error
	UpdateUnit(context.Context, domain.Unit, int64) error
	NextUnitID(context.Context) (string, error)
	RecordOperation(context.Context, string, string, string, string, string) (string, bool, error)
	SaveNormativeRevision(context.Context, string, string, string, string, string, bool) error
	NextDecisionID(context.Context, string) (string, error)
	MarkDecisionSynced(context.Context, string, string, int64) error
	UpdateDecision(context.Context, domain.HCDDecision) error
	SaveDecision(context.Context, domain.HCDDecision) error
	ListDecisions(context.Context, string) ([]domain.HCDDecision, error)
	NextClarificationID(context.Context, string) (string, error)
	SaveClarification(context.Context, domain.Clarification) error
	GetClarification(context.Context, string, string) (domain.Clarification, error)
	ListOpenClarifications(context.Context, string) ([]domain.Clarification, error)
	SaveApprovalPrompt(context.Context, domain.ApprovalPrompt) error
	GetApprovalPrompt(context.Context, string, string) (domain.ApprovalPrompt, error)
	ConsumeApprovalPrompt(context.Context, string, string) error
	NextApprovalID(context.Context, string) (string, error)
	SaveApproval(context.Context, domain.Approval) error
	AddLink(context.Context, domain.Link) error
	ListLinks(context.Context, string) ([]domain.Link, error)
	SetFocus(context.Context, domain.Focus) error
	GetFocus(context.Context) (domain.Focus, error)
	ClearFocus(context.Context) error
	ListHeartbeats(context.Context, string) ([]domain.Heartbeat, error)
	RecordHeartbeat(context.Context, domain.Heartbeat) error
	NextMigrationID(context.Context) (string, error)
	SaveMigration(context.Context, domain.Migration) error
	GetMigration(context.Context, string) (domain.Migration, error)
	ListMigrations(context.Context) ([]domain.Migration, error)
}
