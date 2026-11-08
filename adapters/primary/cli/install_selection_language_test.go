package cli

import (
	"testing"

	"mem/adapters/primary/console"
)

type fakeLangUI struct {
	console.UI
	selectedLang string
}

func (f *fakeLangUI) Select(title string, opts []console.Option) (string, error) {
	if len(opts) > 0 && (opts[0].Value == "es" || opts[0].Value == "en") {
		return f.selectedLang, nil
	}
	return opts[0].Value, nil
}

func (f *fakeLangUI) MultiSelect(title string, opts []console.Option) ([]string, error) {
	return []string{"claude"}, nil
}

func (f *fakeLangUI) Confirm(title string, def bool) (bool, error) {
	return true, nil
}

func TestAskInstallSelectionLanguage(t *testing.T) {
	fakeUI := &fakeLangUI{selectedLang: "en"}
	sel, err := askInstallSelection(fakeUI, []agentChoice{}, true, installSelection{})
	if err != nil {
		t.Fatalf("askInstallSelection falló: %v", err)
	}

	if sel.language != "en" {
		t.Errorf("se esperaba sel.language == 'en', se obtuvo %q", sel.language)
	}
}
