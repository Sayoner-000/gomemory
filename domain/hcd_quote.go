package domain

import "strings"

func ValidQuote(quote, revision string) bool {
	clean := strings.TrimSpace(ProtectText(quote))
	if clean == "" {
		return false
	}
	if len(revision) == 12 && strings.Contains(strings.ToLower(clean), strings.ToLower(revision)) {
		return true
	}
	normalized := strings.Join(strings.Fields(strings.ToLower(clean)), " ")
	switch normalized {
	case "aprobar", "apruebo", "sí aprobar", "si aprobar", "sí, aprobar", "si, aprobar", "sí apruebo", "si apruebo", "sí, apruebo", "si, apruebo":
		return true
	default:
		return false
	}
}
