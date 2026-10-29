package domain

type HCDDecision struct {
	ID, UnitID, Kind, Status, Summary, Revision string
	Refs                                        []string
	Source, MemorySync                          string
	MemoryID                                    int64
}

type Clarification struct {
	ID, UnitID, TargetRef, Question, Recommendation, Answer, Status string
	Options                                                         []string
	Blocking                                                        bool
}

type Block struct {
	ID, UnitID, TargetRef, Reason, Status, Resolution string
	Critical                                          bool
}

type Approval struct {
	ID, UnitID, Object, Revision, Origin, Quote, Client, ApprovedAt string
}

type ApprovalPrompt struct {
	ID, UnitID, Object, Revision, AskedAt string
	Options                               []string
	Consumed                              bool
}

type Link struct {
	UnitID, FromKind, FromID, ToKind, ToID, Relation string
}

type Focus struct {
	UnitID, Client, SessionID, SetAt, SetBy string
}

type Heartbeat struct {
	ID, UnitID, SessionID, Client, Stage string
	StateRevision                        int64
	Files, Drift                         []string
	SeenAt                               string
}

type HCDActiveContext struct {
	Unit           Unit
	Stage          Stage
	NextAction     NextStep
	Clarifications []Clarification
	Decisions      []HCDDecision
	Drift          []string
}

type HCDApproachCandidate struct {
	Folder, Name, Origin   string
	HasDefinition, HasSpec bool
}
