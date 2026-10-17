package domain

import (
	"testing"
	"time"
)

func TestSocraticaCapabilities(t *testing.T) {
	for _, client := range []string{"claude", "codex", "opencode"} {
		c := CapabilitiesForSocratica(client)
		if c.NamedCommand || c.Interception || c.Clipboard || c.InputRestore || c.Pane {
			t.Fatalf("unverified capability: %s", client)
		}
	}
	if DefaultHistoryLimit != 20 || PendingMaxAge != 7*24*time.Hour || TerminalRetention != 24*time.Hour || ContinuationWindow != 30*time.Minute || LowInfoMaxWords != 3 || DefaultGeneratorTimeout != 120*time.Second || DefaultQueryTimeout != 5*time.Second {
		t.Fatal("incorrect defaults")
	}
}

func TestValidateSocraticaSettings(t *testing.T) {
	s, err := ValidateSocraticaSettings(SocraticaSettings{})
	if err != nil || *s.HistoryLimit != 20 || *s.GeneratorTimeoutSeconds != 120 || *s.QueryTimeoutMS != 5000 || len(s.Generators) != 3 {
		t.Fatal(s, err)
	}
	for _, n := range []int{-1, 0, 1001} {
		_, err = ValidateSocraticaSettings(SocraticaSettings{HistoryLimit: &n})
		if err == nil {
			t.Fatalf("accepted history_limit=%d", n)
		}
	}
	for _, n := range []int{0, -1} {
		_, err = ValidateSocraticaSettings(SocraticaSettings{GeneratorTimeoutSeconds: &n})
		if err == nil {
			t.Fatal("accepted invalid timeout")
		}
	}
	n := 99
	if _, err = ValidateSocraticaSettings(SocraticaSettings{QueryTimeoutMS: &n}); err == nil {
		t.Fatal("accepted query timeout")
	}
	if _, err = ValidateSocraticaSettings(SocraticaSettings{Generators: []string{"unknown"}}); err == nil {
		t.Fatal("accepted unknown generator")
	}
	if _, err = ValidateSocraticaSettings(SocraticaSettings{Interception: map[string]bool{"unknown": true}}); err == nil {
		t.Fatal("accepted unknown client")
	}
}
