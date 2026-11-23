package setup

import (
	"os"
	"strings"
	"testing"
)

func TestHCDVerificationScheduling(t *testing.T) {
	for _, name := range []string{"hcd-implement", "hcd-verify"} {
		b, err := os.ReadFile("../../../infrastructure/templates/hcd/" + name + "/SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		for _, required := range []string{"No ejecutes la misma comprobación fuera y dentro del motor", "No revalides tareas anteriores solo por caducidad", "Antes del cierre"} {
			if !strings.Contains(string(b), required) {
				t.Errorf("%s: falta %q", name, required)
			}
		}
	}
}
