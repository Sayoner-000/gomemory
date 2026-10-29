package hcdfiles

import (
	"mem/domain"
	"strings"
)

func ParseEvidence(data []byte) (domain.Evidence, error) {
	var v domain.Evidence
	for _, line := range strings.Split(domain.ProtectText(string(data)), "\n") {
		if strings.TrimSpace(line) == "---" {
			break
		}
		k, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(k) {
		case "Cubre":
			v.Covers = splitValues(value)
		case "Resultado":
			v.Result = value
		case "Procedimiento":
			v.Procedure = value
		case "Entorno":
			v.Environment = value
		case "Sujeto":
			if value != "" {
				v.SubjectRef = []string{value}
			}
		case "Inspeccionable":
			if value != "" {
				v.InspectRef = []string{value}
			}
		case "Ejecutor":
			v.Executor = value
		}
	}
	// Nombra cada campo que falta: un `usage` sin detalle deja al agente
	// probando formatos a ciegas.
	var checks []domain.PendingCheck
	for _, f := range []struct {
		name    string
		missing bool
	}{{"Procedimiento", v.Procedure == ""}, {"Entorno", v.Environment == ""}, {"Sujeto", len(v.SubjectRef) == 0}, {"Resultado", v.Result != "pass" && v.Result != "fail"}} {
		if f.missing {
			checks = append(checks, domain.PendingCheck{Code: "EVIDENCE-FIELDS", Field: f.name, Message: "falta «" + f.name + ":» en una línea propia antes de ---"})
		}
	}
	if len(checks) > 0 {
		return v, &domain.HCDValidationError{Checks: checks}
	}
	return v, nil
}
