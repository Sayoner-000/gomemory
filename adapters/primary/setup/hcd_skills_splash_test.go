package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHCD057SplashSkill(t *testing.T) {
	root := filepath.Clean("../../../infrastructure/templates/hcd")
	approveData, err := os.ReadFile(filepath.Join(root, "hcd-approve", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(approveData)

	required := []string{
		"review --kind splash",
		"--action accept",
		"--action skip",
		"--action start",
		"--action complete",
		"--action dismiss",
		"aislamiento por instrucciones",
	}

	for _, req := range required {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(req)) {
			t.Errorf("hcd-approve/SKILL.md missing required splash guidance: %q", req)
		}
	}
}
