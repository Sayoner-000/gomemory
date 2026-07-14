package ports

type SettingsData struct {
	AutoApprove      bool     `json:"auto_approve"`
	AutoApproveTools []string `json:"auto_approve_tools"`
	// CodeGraphDisabled apaga el proveedor de grafo de código externo
	// (codebase-memory-mcp). Ausente/false = auto-detección activada.
	CodeGraphDisabled bool `json:"code_graph_disabled,omitempty"`
	// CodeGraphCommand apunta a otro binario del proveedor. Vacío = default.
	CodeGraphCommand string `json:"code_graph_command,omitempty"`
	// Budget es el techo blando (en CARACTERES emitidos) de get_context. <=0 =
	// sin límite (opt-in). Reduce la huella persistente del contexto de arranque.
	Budget int `json:"budget,omitempty"`
	// CompactThreshold es la huella (en caracteres emitidos por gomemory en la
	// sesión) a partir de la cual el hook de fin de turno sugiere compactar. <=0 =
	// desactivado.
	CompactThreshold int `json:"compact_threshold,omitempty"`
	// DedupWindowDays es la ventana (días) para consolidar memorias equivalentes
	// (mismo proyecto+tipo+título) en vez de crear filas nuevas. <=0 = sin dedup
	// por identidad.
	DedupWindowDays int `json:"dedup_window_days,omitempty"`
}

type SettingsRepository interface {
	Read(root string) SettingsData
	Write(root string, s SettingsData) error
	ApplyAutoApprove(root string, s SettingsData)
}
