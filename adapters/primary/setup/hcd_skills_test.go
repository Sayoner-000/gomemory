package setup

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestHCDSkillsShareRulesAndCoverMVPCommands(t *testing.T) {
	root := filepath.Clean("../../../infrastructure/templates/hcd")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	contract, err := os.ReadFile("../../../specs/045-hcd-protocol/contracts/skills.md")
	if err != nil {
		t.Fatal(err)
	}
	section := strings.SplitN(strings.SplitN(string(contract), "## 2. Reglas comunes", 2)[1], "## 3.", 2)[0]
	var literalRules []string
	for _, line := range strings.Split(section, "\n") {
		if len(line) > 2 && line[0] >= '1' && line[0] <= '8' && line[1] == '.' {
			literalRules = append(literalRules, line)
		}
	}
	var bodies []string
	common := []string{"**Motor primero.**", "**Sin motor no hay avance.**", "**Las mutaciones van por el motor.**", "**Preguntar antes que suponer**", "**Aprobación citada**", "**Detención localizada**", "**Siguiente paso.**", "**Privacidad.**"}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name(), "SKILL.md")
		data, e := os.ReadFile(path)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			t.Fatal(e)
		}
		body := string(data)
		bodies = append(bodies, body)
		for _, rule := range append(common, literalRules...) {
			if !strings.Contains(body, rule) {
				t.Errorf("%s lacks common rule %q", path, rule)
			}
		}
		for _, forbidden := range []string{"SPECKIT", ".claude/", ".codex/", ".opencode/", ".agents/"} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s contains prohibited external write target %q", path, forbidden)
			}
		}
	}
	all := strings.Join(bodies, "\n")
	// FR-045 / skills.md §6: cada comando mutante del contrato aparece, entre comillas
	// invertidas, en al menos una SKILL.md. `heartbeat` es interno (lo llama el hook).
	engine, err := os.ReadFile("../../../specs/045-hcd-protocol/contracts/engine-cli.md")
	if err != nil {
		t.Fatal(err)
	}
	mutating := []string{"exception", "playbook", "migrate"} // incrementos posteriores: fuera de la tabla
	for _, line := range strings.Split(string(engine), "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 4 || !strings.HasPrefix(strings.TrimSpace(cells[1]), "`") || !strings.HasPrefix(strings.TrimSpace(cells[2]), "sí") {
			continue
		}
		if name := strings.Fields(strings.Trim(strings.TrimSpace(cells[1]), "`"))[0]; name != "heartbeat" {
			mutating = append(mutating, name)
		}
	}
	if len(mutating) < 15 {
		t.Fatalf("la tabla de comandos del contrato no se leyó bien: %v", mutating)
	}
	for _, command := range mutating {
		if !regexp.MustCompile("`(?:mem hcd )?" + command + "[ `]").MatchString(all) {
			t.Errorf("ninguna SKILL.md conduce el comando mutante %q", command)
		}
	}
}
