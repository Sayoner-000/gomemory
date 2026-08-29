package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// skillDePrueba imita la skill real en lo único que el instalador mira: es un
// cuerpo completo con su propio frontmatter. A diferencia del método de
// planificación atómica, aquí gomemory NO inyecta frontmatter por agente — la
// skill es agnóstica y se distribuye verbatim.
const skillDePrueba = `---
name: adversarial-consensus-review
description: Revisión adversarial por consenso.
---

# Adversarial Consensus Review

Cuerpo del protocolo.
`

// TestInstallAdversarialReviewSkill_AlcanzaLosTresAgentes cubre FR-044: la guía
// de participación debe llegar a los tres agentes que exponen un directorio de
// habilidades, no solo a Claude Code.
func TestInstallAdversarialReviewSkill_AlcanzaLosTresAgentes(t *testing.T) {
	home := homeFalso(t, ".claude", ".codex", ".opencode")

	escritos, err := InstallAdversarialReviewSkill(skillDePrueba)
	if err != nil {
		t.Fatalf("InstallAdversarialReviewSkill: %v", err)
	}

	esperados := []string{
		filepath.Join(home, ".claude", "skills", "adversarial-consensus-review", "SKILL.md"),
		filepath.Join(home, ".codex", "skills", "adversarial-consensus-review", "SKILL.md"),
		filepath.Join(home, ".opencode", "skills", "adversarial-consensus-review", "SKILL.md"),
	}
	for _, p := range esperados {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("falta %s: %v", p, err)
			continue
		}
		if string(data) != skillDePrueba {
			t.Errorf("%s: el contenido no es la skill verbatim", p)
		}
	}
	if len(escritos) != len(esperados) {
		t.Errorf("escritos = %d rutas, se esperaban %d: %v", len(escritos), len(esperados), escritos)
	}
}

// TestInstallAdversarialReviewSkill_NoCreaConfigDeAgentesNoUsados: crear
// ~/.codex para quien no usa Codex es ensuciar su directorio personal. Mismo
// criterio que InstallAtomicPlanGlobal.
func TestInstallAdversarialReviewSkill_NoCreaConfigDeAgentesNoUsados(t *testing.T) {
	home := homeFalso(t, ".claude")

	if _, err := InstallAdversarialReviewSkill(skillDePrueba); err != nil {
		t.Fatalf("InstallAdversarialReviewSkill: %v", err)
	}

	presente := filepath.Join(home, ".claude", "skills", adversarialReviewSkillName, "SKILL.md")
	if _, err := os.Stat(presente); err != nil {
		t.Errorf("no se escribió la skill del agente que sí existe: %v", err)
	}
	for _, ausente := range []string{".codex", ".opencode"} {
		if _, err := os.Stat(filepath.Join(home, ausente)); err == nil {
			t.Errorf("se creó %s para un agente que la persona no usa", ausente)
		}
	}
}

// TestInstallAdversarialReviewSkill_Idempotente: reinstalar no debe reescribir
// un archivo ya al día. El segundo paso no reporta ninguna ruta escrita.
func TestInstallAdversarialReviewSkill_Idempotente(t *testing.T) {
	homeFalso(t, ".claude", ".codex", ".opencode")

	if _, err := InstallAdversarialReviewSkill(skillDePrueba); err != nil {
		t.Fatalf("primera instalación: %v", err)
	}
	escritos, err := InstallAdversarialReviewSkill(skillDePrueba)
	if err != nil {
		t.Fatalf("segunda instalación: %v", err)
	}
	if len(escritos) != 0 {
		t.Errorf("la reinstalación reescribió %v; debía ser idempotente", escritos)
	}
}

// TestInstallAdversarialReviewSkill_ActualizaVersionPrevia: si la skill cambia
// entre versiones de gomemory, la instalada debe quedar al día. Es el caso que
// distingue "idempotente" de "no vuelve a escribir nunca".
func TestInstallAdversarialReviewSkill_ActualizaVersionPrevia(t *testing.T) {
	home := homeFalso(t, ".claude")
	dest := filepath.Join(home, ".claude", "skills", "adversarial-consensus-review", "SKILL.md")

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("preparar destino: %v", err)
	}
	if err := os.WriteFile(dest, []byte("versión antigua\n"), 0o644); err != nil {
		t.Fatalf("escribir versión antigua: %v", err)
	}

	if _, err := InstallAdversarialReviewSkill(skillDePrueba); err != nil {
		t.Fatalf("InstallAdversarialReviewSkill: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("leer destino: %v", err)
	}
	if string(data) != skillDePrueba {
		t.Errorf("la skill no se actualizó a la versión vigente")
	}
}

// TestInstallAdversarialReviewSkill_SinCuerpoNoEscribe: si la plantilla
// embebida faltara, el instalador degrada sin crear un archivo vacío que el
// agente cargaría como una skill rota.
func TestInstallAdversarialReviewSkill_SinCuerpoNoEscribe(t *testing.T) {
	home := homeFalso(t, ".claude", ".codex", ".opencode")

	escritos, err := InstallAdversarialReviewSkill("   \n  ")
	if err != nil {
		t.Fatalf("InstallAdversarialReviewSkill: %v", err)
	}
	if len(escritos) != 0 {
		t.Errorf("escribió %v con un cuerpo vacío", escritos)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "adversarial-consensus-review")); err == nil {
		t.Error("creó el directorio de la skill sin tener contenido que escribir")
	}
}

// TestInstallAgentSkill_RespetaElFiltroDeAgentes: las dos habilidades que ya
// distribuye gomemory (atomic-decomposition y constitution) llegan a Claude por
// otra vía ya entregada. Instalarlas también por este canal daría DOS dueños al
// mismo archivo, que es como empiezan las divergencias. El filtro permite que
// cada artefacto tenga un solo dueño por agente.
func TestInstallAgentSkill_RespetaElFiltroDeAgentes(t *testing.T) {
	home := homeFalso(t, ".claude", ".codex", ".opencode")

	escritos, err := InstallAgentSkill("atomic-decomposition", skillDePrueba, "codex", "opencode")
	if err != nil {
		t.Fatalf("InstallAgentSkill: %v", err)
	}
	if len(escritos) != 2 {
		t.Errorf("escribió %d rutas, se esperaban 2: %v", len(escritos), escritos)
	}

	for _, esperado := range []string{".codex", ".opencode"} {
		p := filepath.Join(home, esperado, "skills", "atomic-decomposition", "SKILL.md")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("falta %s: %v", p, err)
		}
	}
	excluido := filepath.Join(home, ".claude", "skills", "atomic-decomposition")
	if _, err := os.Stat(excluido); err == nil {
		t.Error("escribió en Claude pese a estar excluido: el archivo tendría dos dueños")
	}
}

// TestInstallAgentSkill_SinFiltroAlcanzaTodos: omitir el filtro mantiene el
// comportamiento de la guía adversarial, que sí es dueña única en los tres.
func TestInstallAgentSkill_SinFiltroAlcanzaTodos(t *testing.T) {
	homeFalso(t, ".claude", ".codex", ".opencode")

	escritos, err := InstallAgentSkill("cualquiera", skillDePrueba)
	if err != nil {
		t.Fatalf("InstallAgentSkill: %v", err)
	}
	if len(escritos) != 3 {
		t.Errorf("escribió %d rutas, se esperaban 3: %v", len(escritos), escritos)
	}
}

// TestInstallExistingSkillsForAgents_CierraElHuecoSinDuplicarDueño verifica el
// reparto exacto: el método de descomposición evita Claude (ya lo recibe por
// InstallAtomicPlanGlobal, hacia la MISMA ruta global), mientras que la
// constitución sí lo incluye porque su instalador es de ámbito de proyecto y
// deja la ruta global de Claude sin dueño.
func TestInstallExistingSkillsForAgents_CierraElHuecoSinDuplicarDueño(t *testing.T) {
	home := homeFalso(t, ".claude", ".codex", ".opencode")

	if _, err := InstallExistingSkillsForAgents("MÉTODO ATÓMICO", "CONSTITUCIÓN"); err != nil {
		t.Fatalf("InstallExistingSkillsForAgents: %v", err)
	}

	debenExistir := []string{
		filepath.Join(home, ".codex", "skills", "atomic-decomposition", "SKILL.md"),
		filepath.Join(home, ".opencode", "skills", "atomic-decomposition", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "constitution", "SKILL.md"),
		filepath.Join(home, ".codex", "skills", "constitution", "SKILL.md"),
		filepath.Join(home, ".opencode", "skills", "constitution", "SKILL.md"),
	}
	for _, p := range debenExistir {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("falta %s: %v", p, err)
		}
	}

	noDebeExistir := filepath.Join(home, ".claude", "skills", "atomic-decomposition")
	if _, err := os.Stat(noDebeExistir); err == nil {
		t.Error("escribió atomic-decomposition en Claude: InstallAtomicPlanGlobal ya es su dueño")
	}
}

// TestInstallExistingSkillsForAgents_CuerpoAusenteNoEscribe: si una plantilla
// falta, esa habilidad se omite sin arrastrar a la otra.
func TestInstallExistingSkillsForAgents_CuerpoAusenteNoEscribe(t *testing.T) {
	home := homeFalso(t, ".codex")

	escritos, err := InstallExistingSkillsForAgents("", "CONSTITUCIÓN")
	if err != nil {
		t.Fatalf("InstallExistingSkillsForAgents: %v", err)
	}
	if len(escritos) != 1 {
		t.Errorf("escribió %d rutas, se esperaba solo la constitución: %v", len(escritos), escritos)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "skills", "atomic-decomposition")); err == nil {
		t.Error("creó atomic-decomposition sin tener método que escribir")
	}
}

// TestSkillEmbebida_EsAgnosticaDeProveedor cubre FR-045 sobre el artefacto real
// que se distribuye: la guía no puede acoplarse a gomemory ni a un agente
// concreto, porque se instala igual en los tres.
func TestSkillEmbebida_EsAgnosticaDeProveedor(t *testing.T) {
	origen := filepath.Join("..", "..", "..", "infrastructure", "templates",
		adversarialReviewSkillName, "SKILL.md")
	data, err := os.ReadFile(origen)
	if err != nil {
		t.Fatalf("leer la skill embebida: %v", err)
	}
	cuerpo := string(data)

	prohibidos := []string{
		"gomemory", "save_memory", "get_context", "mem review",
		"Claude Code", "OpenCode", "Codex",
	}
	for _, prohibido := range prohibidos {
		if strings.Contains(cuerpo, prohibido) {
			t.Errorf("la skill menciona %q: dejaría de ser agnóstica de proveedor (FR-045)", prohibido)
		}
	}
	if !strings.HasPrefix(cuerpo, "---\n") {
		t.Error("la skill no empieza con frontmatter: los agentes no la descubrirían")
	}
}
