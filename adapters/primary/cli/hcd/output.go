package hcd

func ExitCode(code string) int {
	switch code {
	case "usage":
		return 2
	case "not_found":
		return 3
	case "revision_conflict", "operation_conflict":
		return 4
	case "not_allowed", "blocked", "atomicity", "no_executor", "already_approved", "invalid_quote", "constitution_conflict":
		return 5
	case "approach_mismatch":
		return 6
	case "uncertain":
		return 7
	default:
		return 1
	}
}

type PendingCheck struct {
	Code   string `json:"code"`
	Ref    string `json:"ref"`
	Detail string `json:"detail"`
}
type NextAction struct {
	Skill   string `json:"skill"`
	Command string `json:"command"`
	Why     string `json:"why"`
}
type Revision struct {
	State      int64  `json:"state"`
	Definition string `json:"definition"`
	Delivery   string `json:"delivery"`
	Approved   string `json:"approved"`
}
type Error struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Expected int64  `json:"expected,omitempty"`
	Actual   int64  `json:"actual,omitempty"`
}
type Response struct {
	ProtocolVersion  int            `json:"protocol_version"`
	OK               bool           `json:"ok"`
	UnitID           string         `json:"unit_id"`
	Approach         string         `json:"approach"`
	State            string         `json:"state"`
	Stage            string         `json:"stage"`
	Revision         Revision       `json:"revision"`
	PlaybookRevision string         `json:"playbook_revision"`
	AllowedActions   []string       `json:"allowed_actions"`
	PendingChecks    []PendingCheck `json:"pending_checks"`
	Blockers         []string       `json:"blockers"`
	NextAction       NextAction     `json:"next_action"`
	ExecutionID      string         `json:"execution_id"`
	MemorySyncStatus string         `json:"memory_sync_status"`
	OperationID      string         `json:"operation_id"`
	Error            *Error         `json:"error,omitempty"`
	Data             any            `json:"data"`
}
