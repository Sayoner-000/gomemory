package ports

type SettingsData struct {
	AutoApprove      bool     `json:"auto_approve"`
	AutoApproveTools []string `json:"auto_approve_tools"`
}

type SettingsRepository interface {
	Read(root string) SettingsData
	Write(root string, s SettingsData) error
	ApplyAutoApprove(root string, s SettingsData)
}
