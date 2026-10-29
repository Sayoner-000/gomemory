package hcd

// ProtocolVersion 2 es el contrato de HCD v2 (feature 045, contracts/engine-cli.md).
const ProtocolVersion = 2

func ExitCode(code string) int {
	switch code {
	case "usage":
		return 2
	case "not_found":
		return 3
	case "not_allowed", "invalid_quote", "validation":
		return 5
	default:
		return 1
	}
}

type PendingCheck struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
}
type NextAction struct {
	Skill   string `json:"skill"`
	Command string `json:"command"`
	Why     string `json:"why"`
}
type Task struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Check  string `json:"check"`
	Status string `json:"status"`
}
type Finding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Ref      string `json:"ref"`
	Message  string `json:"message"`
}
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Response struct {
	ProtocolVersion int            `json:"protocol_version"`
	OK              bool           `json:"ok"`
	UnitID          string         `json:"unit_id"`
	Kind            string         `json:"kind"`
	Lane            string         `json:"lane"`
	State           string         `json:"state"`
	Stage           string         `json:"stage"`
	DocPath         string         `json:"doc_path"`
	DefinitionRev   string         `json:"definition_revision"`
	WorkRev         string         `json:"work_revision"`
	Approved        bool           `json:"approved"`
	Fingerprint     string         `json:"fingerprint"`
	Tasks           []Task         `json:"tasks"`
	PendingChecks   []PendingCheck `json:"pending_checks"`
	RedMissing      []string       `json:"red_missing"`
	Findings        []Finding      `json:"findings"`
	NextAction      NextAction     `json:"next_action"`
	Warnings        []string       `json:"warnings"`
	Error           *Error         `json:"error,omitempty"`
	Data            map[string]any `json:"data"`
}
