package persistence

import (
	"encoding/json"
	"errors"
	"mem/domain"
	"os"
)

// This strict reader is separate from legacy ReadSettings, whose signature
// cannot report invalid input. Socratica must never silently normalize it.
func ReadSocraticaSettings(root string) (domain.SocraticaSettings, error) {
	var s domain.SocraticaSettings
	raw, err := os.ReadFile(SettingsPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return domain.ValidateSocraticaSettings(s)
	}
	if err != nil {
		return s, err
	}
	if err = json.Unmarshal(raw, &s); err != nil {
		key := "socratica settings"
		var typed *json.UnmarshalTypeError
		if errors.As(err, &typed) {
			key = typed.Field
		}
		return s, &domain.ErrInvalidSocraticaSettings{Key: key, Value: "invalid JSON type or syntax"}
	}
	return domain.ValidateSocraticaSettings(s)
}
