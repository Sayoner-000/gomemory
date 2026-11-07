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
		if len(line) > 2 && line[0] >= '1' && line[0] <= '9' && line[1] == '.' {
			literalRules = append(literalRules, line)
		}
	}
	var bodies []string
	common := []string{"**Motor primero.**", "**Sin motor no hay avance.**", "**Las mutaciones van por el motor.**", "**Preguntar antes que suponer.**", "**Aprobación citada.**", "**La casilla no es la prueba.**", "**Siguiente paso.**", "**Privacidad.**", "**La Constitución y el Playbook mandan.**"}
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
	// Tabla v2: | comando | flags | escribe | efecto |.
	var mutating []string
	for _, line := range strings.Split(string(engine), "\n") {
		// «\\|» es una barra escapada dentro de una celda, no un separador.
		cells := strings.Split(strings.ReplaceAll(line, `\|`, "/"), "|")
		if len(cells) < 5 || !strings.HasPrefix(strings.TrimSpace(cells[1]), "`") || !strings.HasPrefix(strings.TrimSpace(cells[3]), "sí") {
			continue
		}
		mutating = append(mutating, strings.Fields(strings.Trim(strings.TrimSpace(cells[1]), "`"))[0])
	}
	if len(mutating) < 8 {
		t.Fatalf("la tabla de comandos del contrato no se leyó bien: %v", mutating)
	}
	for _, command := range mutating {
		if !regexp.MustCompile("`(?:mem hcd )?" + command + "[ `]").MatchString(all) {
			t.Errorf("ninguna SKILL.md conduce el comando mutante %q", command)
		}
	}
}

// HCD-051: /hcd-approve ofrece la revisión splash antes de pedir la cita, con
// la invocación de cada agente, la regla del Blocker (Q2) y el aviso cuando
// splash no está disponible (Q3).
func TestHCDSkillsApproveOfreceSplash(t *testing.T) {
	data, err := os.ReadFile("../../../infrastructure/templates/hcd/hcd-approve/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, want := range []string{
		"Revisión splash antes de aprobar",
		"Ofrece",
		"`invoke_subagent` con `TypeName: \"splash\"`",
		"`spawn_agent(agent_type: \"splash\", fork_turns: \"none\")`",
		"Blocker",
		"descarta de forma explícita",
		"Major y Nit son informativos",
		"`mem setup`",
		"la aprobación sigue sin la revisión",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("hcd-approve/SKILL.md no contiene %q", want)
		}
	}
	revision, pregunta := strings.Index(body, "Revisión splash antes de aprobar"), strings.Index(body, "¿Apruebas la definición")
	if revision < 0 || pregunta < 0 || revision > pregunta {
		t.Errorf("la revisión splash debe ir antes de pedir la aprobación (revisión en %d, pregunta en %d)", revision, pregunta)
	}
}
