package cli

// ResolveHint resuelve un mensaje o sugerencia usando el catálogo i18n para el idioma dado.
func ResolveHint(lang, key string, params ...map[string]string) string {
	cat := getCatalog()
	if lang == "" {
		lang = "es"
	}
	return cat.Get(lang, key, params...)
}

// GetSaveNudge devuelve el mensaje de recordatorio de fin de turno en el idioma configurado.
func GetSaveNudge(lang string) string {
	return ResolveHint(lang, "save_nudge")
}

// GetPlanModeReminder devuelve el recordatorio de modo plan en el idioma configurado.
func GetPlanModeReminder(lang string) string {
	return ResolveHint(lang, "plan_mode_reminder")
}
