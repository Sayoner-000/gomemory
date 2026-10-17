package generator

import (
	"mem/application/ports"
	"mem/domain"
	"strings"
)

func (f *FirstAvailable) ExplainFailure(raw string) ports.SocraticFailure {
	// The message is for display: provider stderr can carry control sequences.
	message := domain.StripTerminalControls(domain.ProtectText(raw))
	if strings.Contains(raw, "FreeTierError") || strings.Contains(raw, "free tier can only be used from within OpenCode") || strings.Contains(raw, "OpenCode rechazó la generación con el modelo gratuito") {
		return ports.SocraticFailure{Message: "OpenCode rechazó la generación con el modelo gratuito (403). Reintentar no resuelve esta restricción. Elige otro harness o configura en OpenCode un proveedor y modelo que permitan generar texto."}
	}
	if strings.Contains(raw, `"type":"provider.auth"`) || strings.Contains(raw, "Not logged in") || strings.Contains(raw, "configura la autenticación del harness") {
		return ports.SocraticFailure{Message: "El proveedor rechazó la solicitud. Configura la autenticación del harness o elige otro antes de continuar."}
	}
	if strings.Contains(raw, "invalid_json_schema") || strings.Contains(raw, "not a valid JSON Schema") || strings.Contains(raw, "esquema de respuesta no compatible") {
		return ports.SocraticFailure{Message: "El harness rechazó el esquema de respuesta. Se requiere una versión compatible; elige otro harness para continuar."}
	}
	return ports.SocraticFailure{Message: message, Retryable: domain.RetryableGenerationFailure(raw)}
}
