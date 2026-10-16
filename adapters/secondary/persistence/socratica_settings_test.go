package persistence

import (
	"mem/domain"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadSocraticaSettings_InvalidValue(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(SettingsPath(root)), 0700); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"socratica_history_limit":0}`, `{"socratica_query_timeout_ms":"soon"}`, `{"socratica_generators":["unknown"]}`} {
		if err := os.WriteFile(SettingsPath(root), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadSocraticaSettings(root); err == nil {
			t.Fatal("invalid setting accepted")
		}
	}
}

func TestSettings_SocraticaRoundTrip(t *testing.T) {
	root := t.TempDir()
	repo := NewSettingsRepository()
	s := repo.Read(root)
	n, timeout, query := 7, 60, 500
	s.SocraticaSettings = domain.SocraticaSettings{Interception: map[string]bool{"claude": true}, InterceptionUnverified: map[string]bool{"claude": true}, HistoryLimit: &n, Private: true, Generators: []string{"codex"}, GeneratorTimeoutSeconds: &timeout, QueryTimeoutMS: &query}
	if err := repo.Write(root, s); err != nil {
		t.Fatal(err)
	}
	next := repo.Read(root)
	next.Theme = "light"
	if err := repo.Write(root, next); err != nil {
		t.Fatal(err)
	}
	got := repo.Read(root)
	if !reflect.DeepEqual(got.SocraticaSettings, s.SocraticaSettings) {
		t.Fatalf("got %+v want %+v", got.SocraticaSettings, s.SocraticaSettings)
	}
}
