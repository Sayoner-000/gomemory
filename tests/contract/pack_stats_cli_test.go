package main

import (
	"strings"
	"testing"

	"mem/adapters/primary/cli"
)

const fixturePackJSON = `{
  "Items": [
    {"ID": "memory:1", "Content": "algo crítico", "Priority": 0, "Tokens": 10, "RawTokens": 10}
  ],
  "Budget": 500,
  "RawTokenCount": 20,
  "TokenCount": 10,
  "CompressionRate": 0.5,
  "Stats": {
    "RawTokens": 20, "FinalTokens": 10, "SavedTokens": 10, "CompressionRatio": 0.5,
    "ItemsRetrieved": 3, "ItemsDuplicate": 1, "ItemsCritical": 1, "ItemsRelevant": 0,
    "ItemsOptional": 0, "ItemsDiscarded": 1
  }
}`

func TestParseContextPackInput_ValidJSON(t *testing.T) {
	pack, err := cli.ParseContextPackInput(strings.NewReader(fixturePackJSON))
	if err != nil {
		t.Fatalf("ParseContextPackInput: %v", err)
	}
	if pack.Budget != 500 {
		t.Errorf("Budget = %d, se esperaba 500", pack.Budget)
	}
	if pack.Stats.ItemsDuplicate != 1 {
		t.Errorf("Stats.ItemsDuplicate = %d, se esperaba 1", pack.Stats.ItemsDuplicate)
	}
	if len(pack.Items) != 1 || pack.Items[0].ID != "memory:1" {
		t.Errorf("Items = %+v, se esperaba un item memory:1", pack.Items)
	}
}

func TestParseContextPackInput_InvalidJSON(t *testing.T) {
	_, err := cli.ParseContextPackInput(strings.NewReader("no es json"))
	if err == nil {
		t.Fatal("se esperaba error con JSON inválido")
	}
}

func TestFormatContextStats_MatchesFixture(t *testing.T) {
	pack, err := cli.ParseContextPackInput(strings.NewReader(fixturePackJSON))
	if err != nil {
		t.Fatalf("ParseContextPackInput: %v", err)
	}
	out := cli.FormatContextStats(pack.Stats)

	for _, want := range []string{
		"Raw tokens:          20",
		"Final tokens:        10",
		"Ahorrados:            10",
		"Items críticos:       1",
		"Duplicados removidos: 1",
		"Descartados:          1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stats no contiene %q; salida:\n%s", want, out)
		}
	}
}
