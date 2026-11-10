package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHCD057SocraticSkills(t *testing.T) {
	root := filepath.Clean("../../../infrastructure/templates/hcd")

	hcdData, err := os.ReadFile(filepath.Join(root, "hcd", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	hcdBody := string(hcdData)

	defineData, err := os.ReadFile(filepath.Join(root, "hcd-define", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	defineBody := string(defineData)

	for _, tc := range []struct {
		name, target, body string
		required           []string
	}{
		{
			name:   "hcd-define connects socratic exam",
			target: "hcd-define",
			body:   defineBody,
			required: []string{
				"review --kind socratic",
				"tres preguntas",
				"cero preguntas",
				"modo plan",
			},
		},
		{
			name:   "hcd connects socratic exam and plan mode deferral",
			target: "hcd",
			body:   hcdBody,
			required: []string{
				"review",
				"socratic",
				"modo plan",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, req := range tc.required {
				if !strings.Contains(strings.ToLower(tc.body), strings.ToLower(req)) {
					t.Errorf("%s missing required guidance: %q", tc.target, req)
				}
			}
		})
	}
}
