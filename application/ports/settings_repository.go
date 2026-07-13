package ports

type SettingsData struct {
	AutoApprove      bool     `json:"auto_approve"`
	AutoApproveTools []string `json:"auto_approve_tools"`
	// CodeGraphDisabled apaga el proveedor de grafo de código externo
	// (codebase-memory-mcp). Ausente/false = auto-detección activada.
	CodeGraphDisabled bool `json:"code_graph_disabled,omitempty"`
	// CodeGraphCommand apunta a otro binario del proveedor. Vacío = default.
	CodeGraphCommand string `json:"code_graph_command,omitempty"`
}

type SettingsRepository interface {
	Read(root string) SettingsData
	Write(root string, s SettingsData) error
	ApplyAutoApprove(root string, s SettingsData)
}
