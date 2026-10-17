package domain

import "fmt"

// Pointer fields distinguish absent keys from explicitly invalid zero values.
type SocraticaSettings struct {
	Interception            map[string]bool `json:"socratica_interception,omitempty"`
	InterceptionUnverified  map[string]bool `json:"socratica_interception_unverified,omitempty"`
	HistoryLimit            *int            `json:"socratica_history_limit,omitempty"`
	Private                 bool            `json:"socratica_private,omitempty"`
	Generators              []string        `json:"socratica_generators,omitempty"`
	GeneratorTimeoutSeconds *int            `json:"socratica_generator_timeout_seconds,omitempty"`
	QueryTimeoutMS          *int            `json:"socratica_query_timeout_ms,omitempty"`
}
type ErrInvalidSocraticaSettings struct {
	Key   string
	Value any
}

func (e *ErrInvalidSocraticaSettings) Error() string {
	return fmt.Sprintf("invalid socratica setting %s: %v", e.Key, e.Value)
}
func IsSocraticaClient(s string) bool { return s == "claude" || s == "codex" || s == "opencode" }
func ValidateSocraticaSettings(s SocraticaSettings) (SocraticaSettings, error) {
	integer := func(key string, p **int, def, min, max int) error {
		if *p == nil {
			v := def
			*p = &v
			return nil
		}
		v := **p
		if v < min || (max > 0 && v > max) {
			return &ErrInvalidSocraticaSettings{key, v}
		}
		return nil
	}
	if err := integer("socratica_history_limit", &s.HistoryLimit, DefaultHistoryLimit, 1, 1000); err != nil {
		return s, err
	}
	if err := integer("socratica_generator_timeout_seconds", &s.GeneratorTimeoutSeconds, int(DefaultGeneratorTimeout.Seconds()), 1, 0); err != nil {
		return s, err
	}
	if err := integer("socratica_query_timeout_ms", &s.QueryTimeoutMS, int(DefaultQueryTimeout.Milliseconds()), 100, 0); err != nil {
		return s, err
	}
	if s.Generators == nil {
		s.Generators = []string{"claude", "codex", "opencode"}
	}
	for _, g := range s.Generators {
		if !IsSocraticaClient(g) {
			return s, &ErrInvalidSocraticaSettings{"socratica_generators", g}
		}
	}
	for _, m := range []struct {
		k string
		v map[string]bool
	}{{"socratica_interception", s.Interception}, {"socratica_interception_unverified", s.InterceptionUnverified}} {
		for client := range m.v {
			if !IsSocraticaClient(client) {
				return s, &ErrInvalidSocraticaSettings{m.k, client}
			}
		}
	}
	return s, nil
}
