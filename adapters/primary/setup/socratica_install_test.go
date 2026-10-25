package setup

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var socraticaWrappersDePrueba = map[string]string{
	"socratica.md":    "---\ndescription: Socratica refine\n---\n\nAplica la habilidad `socratica` en modo refine a: $ARGUMENTS\n",
	"socratic-q.md":   "---\ndescription: Socratica scrutiny\n---\n\nAplica la habilidad `socratica` en modo scrutiny a: $ARGUMENTS\n",
	"socratica-hu.md": "---\ndescription: Socratica specify\n---\n\nAplica la habilidad `socratica` en modo specify a: $ARGUMENTS\n",
}

const socraticaSkillDePrueba = "---\nname: socratica\ndescription: Refina.\n---\n\n## Seguridad y precedencia\n"

// listarArchivos devuelve las rutas de archivo bajo home, relativas a él.
func listarArchivos(t *testing.T, home string) []string {
	t.Helper()
	var out []string
	_ = filepath.Walk(home, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(home, p)
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// TestInstallSocraticaSkill (T033, FR-023, FR-025).
func TestInstallSocraticaSkill(t *testing.T) {
	home := homeFalso(t, ".claude", ".codex", ".opencode", filepath.Join(".config", "opencode"))
	escritos, err := InstallSocraticaSkill(socraticaSkillDePrueba, socraticaWrappersDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(".claude", "commands", "socratic-q.md"),
		filepath.Join(".claude", "commands", "socratica-hu.md"),
		filepath.Join(".claude", "skills", "socratica", "SKILL.md"),
		filepath.Join(".codex", "skills", "socratica", "SKILL.md"),
		filepath.Join(".config", "opencode", "commands", "socratic-q.md"),
		filepath.Join(".config", "opencode", "commands", "socratica-hu.md"),
		filepath.Join(".config", "opencode", "commands", "socratica.md"),
		filepath.Join(".config", "opencode", "skills", "socratica", "SKILL.md"),
	}
	// El instalador no escribe nada más que la habilidad y los envoltorios.
	if got := listarArchivos(t, home); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("archivos escritos:\n%s\nse esperaban:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if len(escritos) != len(want) {
		t.Fatalf("escritos = %v", escritos)
	}
	for _, w := range []string{
		filepath.Join(".claude", "commands", "socratic-q.md"),
		filepath.Join(".claude", "commands", "socratica-hu.md"),
		filepath.Join(".config", "opencode", "commands", "socratica.md"),
		filepath.Join(".config", "opencode", "commands", "socratic-q.md"),
		filepath.Join(".config", "opencode", "commands", "socratica-hu.md"),
	} {
		data, err := os.ReadFile(filepath.Join(home, w))
		if err != nil || strings.Contains(string(data), "## Seguridad y precedencia") || !strings.Contains(string(data), "$ARGUMENTS") {
			t.Errorf("%s no es un envoltorio fino: %q %v", w, data, err)
		}
	}
	if again, err := InstallSocraticaSkill(socraticaSkillDePrueba, socraticaWrappersDePrueba); err != nil || len(again) != 0 {
		t.Fatalf("la segunda ejecución escribió %v (%v)", again, err)
	}
}

func TestInstallSocraticaSkill_SoloAgentesPresentes(t *testing.T) {
	home := homeFalso(t, ".codex")
	if _, err := InstallSocraticaSkill(socraticaSkillDePrueba, socraticaWrappersDePrueba); err != nil {
		t.Fatal(err)
	}
	if got := listarArchivos(t, home); len(got) != 1 || got[0] != filepath.Join(".codex", "skills", "socratica", "SKILL.md") {
		t.Fatalf("creó configuración de agentes ausentes: %v", got)
	}
}

// TestGlobalGeneratedArtifacts_IncluyeSocratica: la desinstalación retira la
// habilidad y los tres envoltorios (memoria 501).
func TestGlobalGeneratedArtifacts_IncluyeSocratica(t *testing.T) {
	got := map[string]bool{}
	for _, p := range GlobalGeneratedArtifacts() {
		got[filepath.Join(p...)] = true
	}
	for _, want := range []string{
		filepath.Join(".claude", "skills", "socratica"),
		filepath.Join(".codex", "skills", "socratica"),
		filepath.Join(".config", "opencode", "skills", "socratica"),
		// Destino de versiones anteriores: la desinstalación lo sigue limpiando.
		filepath.Join(".opencode", "skills", "socratica"),
		filepath.Join(".claude", "commands", "socratic-q.md"),
		filepath.Join(".config", "opencode", "commands", "socratica.md"),
		filepath.Join(".config", "opencode", "commands", "socratic-q.md"),
		// mode=specify (feature 042): Claude Code and OpenCode only.
		filepath.Join(".claude", "commands", "socratica-hu.md"),
		filepath.Join(".config", "opencode", "commands", "socratica-hu.md"),
	} {
		if !got[want] {
			t.Errorf("falta %s en GlobalGeneratedArtifacts", want)
		}
	}
}

// Codex has no verified command directory: the new alias is never written there,
// and an absent agent is never created for it (feature 042).
func TestInstallSocraticaSkill_AliasSpecifyNoLlegaACodexNiCreaAgentes(t *testing.T) {
	home := homeFalso(t, ".claude", ".codex")
	if _, err := InstallSocraticaSkill(socraticaSkillDePrueba, socraticaWrappersDePrueba); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(listarArchivos(t, home), "\n")
	for _, ausente := range []string{filepath.Join(".codex", "commands"), ".opencode", filepath.Join(".config", "opencode")} {
		if strings.Contains(got, ausente) {
			t.Errorf("se creó %s para un agente que no corresponde:\n%s", ausente, got)
		}
	}
	if !strings.Contains(got, filepath.Join(".claude", "commands", "socratica-hu.md")) {
		t.Errorf("Claude Code no recibió el alias:\n%s", got)
	}
}

// El cuerpo real distribuido es un envoltorio fino: nombra la habilidad y el
// modo, conserva el argumento y no copia reglas del recorrido.
func TestEnvoltorioSocraticaHU_EsFinoYApuntaAlModoSpecify(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "infrastructure", "templates", "socratica", "commands", "socratica-hu.md"))
	if err != nil {
		t.Fatal(err)
	}
	cuerpo := string(data)
	for _, want := range []string{"---\ndescription: ", "habilidad `socratica`", "modo specify", "$ARGUMENTS"} {
		if !strings.Contains(cuerpo, want) {
			t.Errorf("el envoltorio no contiene %q:\n%s", want, cuerpo)
		}
	}
	for _, regla := range []string{"expected_revision", "ack_nonce", "approve", "preparation", "prepare(", "## "} {
		if strings.Contains(cuerpo, regla) {
			t.Errorf("el envoltorio duplica reglas del recorrido (%q)", regla)
		}
	}
	if len(cuerpo) > 400 {
		t.Errorf("el envoltorio no es fino: %d bytes", len(cuerpo))
	}
}
