package cli

import (
	"os"
	"regexp"
	"testing"
)

// HCD-022 C-05: los comandos del ciclo de vida escriben su salida humana por
// console (humanf/humanln/console.Printf) para que Layout les dé la misma
// estructura que mem doctor; fmt.Print* directo a stdout se la salta.
func TestCicloDeVida_SinFmtPrint(t *testing.T) {
	direct := regexp.MustCompile(`\bfmt\.Print(f|ln)?\(`)
	for _, p := range []string{
		"cmd_mcp_setup.go", "cmd_install.go", "cmd_uninstall.go", "cmd_update.go", "cmd_install_cleanup.go",
		"../setup/claude_code_setup.go", "../setup/opencode_setup.go",
	} {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, loc := range direct.FindAllIndex(data, -1) {
			t.Errorf("%s: %q escribe directo a stdout; usa humanf/humanln o console.Printf", p, data[loc[0]:loc[1]])
		}
	}
}
