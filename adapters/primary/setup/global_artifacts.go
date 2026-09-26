package setup

import "path/filepath"

// globalSkillNames son las habilidades que gomemory deposita en el ámbito de
// usuario (InstallAgentSkill, InstallAtomicPlanGlobal).
var globalSkillNames = []string{"atomic-decomposition", "constitution", adversarialReviewSkillName}

// GlobalGeneratedArtifacts devuelve, relativas al HOME, las rutas que gomemory
// escribe como artefacto completo en el ámbito de usuario: el directorio de
// cada habilidad y cada envoltorio de comando. Se derivan de las mismas tablas
// que usa la instalación (skillTargets, globalTargets), para que la
// desinstalación de sistema no pueda olvidar ninguno (feature 034, FR-011).
//
// Nunca incluye un directorio contenedor (~/.claude/skills): aloja también
// habilidades de la persona y de otras herramientas.
func GlobalGeneratedArtifacts() [][]string {
	seen := map[string]bool{}
	var out [][]string
	add := func(p []string) {
		k := filepath.Join(p...)
		if !seen[k] {
			seen[k] = true
			out = append(out, p)
		}
	}
	for _, t := range skillTargets {
		for _, name := range globalSkillNames {
			add(append(append(append([]string{}, t.dir...), t.skills...), name))
		}
	}
	for _, t := range globalTargets {
		if len(t.wrapper) == 0 {
			continue
		}
		w := append(append([]string{}, t.dir...), t.wrapper...)
		// Una habilidad es un directorio con su SKILL.md: se retira el
		// directorio entero. Un comando es un archivo suelto.
		if filepath.Base(filepath.Join(w...)) == "SKILL.md" {
			w = w[:len(w)-1]
		}
		add(w)
	}
	for _, t := range globalTargets {
		if t.agent != "opencode" {
			continue
		}
		// El envoltorio de la constitución convive con el del método en el
		// mismo directorio de comandos.
		add(append(append([]string{}, t.dir...), "commands", "constitution.md"))
	}
	return out
}
