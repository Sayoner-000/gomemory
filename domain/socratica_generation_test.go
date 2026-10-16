package domain

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseGeneration(t *testing.T) {
	for _, tc := range []struct {
		name, input    string
		count, options int
		bad            bool
	}{
		{"empty", `{"questions":[]}`, 0, 0, false},
		{"valid", `{"questions":[{"text":"Audience?","options":["A","B"]}]}`, 1, 2, false},
		{"single option", `{"questions":[{"text":"Audience?","options":["A"]}]}`, 0, 0, false},
		{"trim options", `{"questions":[{"text":"Audience?","options":["A","B","C","D","E"]}]}`, 1, 4, false},
		{"wrapped", "Here is JSON: {\"questions\":[],\"proposed\":\"brace } in string\"} done", 0, 0, false},
		{"broken", `{"questions":`, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, err := ParseGeneration([]byte(tc.input))
			if (err != nil) != tc.bad {
				t.Fatalf("error=%v", err)
			}
			if tc.bad {
				return
			}
			if len(g.Questions) != tc.count {
				t.Fatalf("questions=%d", len(g.Questions))
			}
			if tc.count > 0 && len(g.Questions[0].Options) != tc.options {
				t.Fatal(g)
			}
		})
	}
	q := `{"text":"Q","options":["A","B"]}`
	g, err := ParseGeneration([]byte(fmt.Sprintf(`{"questions":[%s,%s,%s,%s]}`, q, q, q, q)))
	if err != nil || len(g.Questions) != 3 {
		t.Fatalf("%+v %v", g, err)
	}
	g, err = ParseGeneration([]byte(`{"scrutiny":{"assumptions":[{"text":"a","kind":"fact"},{"text":"b","kind":"hypothesis"},{"text":"c","kind":"fact"},{"text":"d","kind":"fact"}]}}`))
	if err != nil || g.Scrutiny == nil || len(g.Scrutiny.Assumptions) != 3 {
		t.Fatal(g, err)
	}
	if _, err = ParseGeneration([]byte(`{"scrutiny":{"assumptions":[{"text":"a","kind":"guess"}]}}`)); err == nil {
		t.Fatal("invalid assumption kind accepted")
	}
	if _, err = ParseGeneration([]byte(`{"questions":[{"text":"","options":["a","b"]}]}`)); err == nil {
		t.Fatal("empty question accepted")
	}
}

func TestWordCount(t *testing.T) {
	if WordCount("  hola\tmundo\n otra ") != 3 {
		t.Fatal("incorrect word count")
	}
	if WordCount(strings.Repeat(" ", 10)) != 0 {
		t.Fatal("whitespace counted")
	}
}
