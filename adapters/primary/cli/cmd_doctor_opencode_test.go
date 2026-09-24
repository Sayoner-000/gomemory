package cli

import (
	"strings"
	"testing"

	"mem/adapters/primary/setup"
	"mem/domain"
)

// TestPrintDoctorOpenCode cubre US4 de la feature 032 (contrato C-4): el doctor
// explica qué versión de OpenCode hay, si el plugin instalado carga en ella y
// qué hacer, y separa los plugins ajenos de los problemas de gomemory.
func TestPrintDoctorOpenCode(t *testing.T) {
	casos := []struct {
		nombre   string
		st       setup.OpenCodeInstallStatus
		quiere   []string
		noQuiere []string
	}{
		{
			nombre:   "dual en v2",
			st:       setup.OpenCodeInstallStatus{Version: "2.0.16", Major: 2, PluginShape: setup.PluginShapeDual},
			quiere:   []string{"OpenCode: v2.0.16 detectado", "plugin gomemory: dual (v1+v2) ✅"},
			noQuiere: []string{"❌", "mem setup-mcp"},
		},
		{
			nombre: "solo v1 en v2",
			st:     setup.OpenCodeInstallStatus{Version: "2.0.16", Major: 2, PluginShape: setup.PluginShapeV1Only},
			quiere: []string{"plugin gomemory: solo v1 ❌", "mem setup-mcp --scope global --agents opencode"},
		},
		{
			nombre: "versión desconocida",
			st:     setup.OpenCodeInstallStatus{PluginShape: setup.PluginShapeDual},
			quiere: []string{"OpenCode: versión no detectada", "dual (v1+v2) ✅"},
		},
		{
			nombre: "ajeno y artefacto",
			st: setup.OpenCodeInstallStatus{Version: "2.0.16", Major: 2, PluginShape: setup.PluginShapeDual,
				ForeignV1Plugins: []string{"cbm-augment.ts"}, StaleTestArtifact: true},
			quiere: []string{"plugin ajeno con forma v1 en OpenCode 2.x", "cbm-augment.ts", "no es de gomemory",
				"artefacto de pruebas en la carpeta de plugins: gomemory.test.mjs"},
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			out := captureStdout(t, func() { printDoctorOpenCode(c.st) })
			for _, q := range c.quiere {
				if !strings.Contains(out, q) {
					t.Errorf("falta %q en:\n%s", q, out)
				}
			}
			for _, q := range c.noQuiere {
				if strings.Contains(out, q) {
					t.Errorf("no debía aparecer %q en:\n%s", q, out)
				}
			}
		})
	}
}

// TestPrintDoctorOpenCode_SinOpenCodeNoImprimeNada: en una máquina sin OpenCode
// ni plugin, la sección no aparece.
func TestPrintDoctorOpenCode_SinOpenCodeNoImprimeNada(t *testing.T) {
	out := captureStdout(t, func() {
		printDoctorOpenCode(setup.OpenCodeInstallStatus{PluginShape: setup.PluginShapeMissing})
	})
	if strings.TrimSpace(out) != "" {
		t.Errorf("sin OpenCode la sección debía omitirse; salió:\n%s", out)
	}
}

// TestOpenCodeProblem: un plugin que no carga en la versión detectada cuenta
// como problema (para --strict y el JSON); un plugin ajeno no.
func TestOpenCodeProblem(t *testing.T) {
	if !openCodeProblem(setup.OpenCodeInstallStatus{Major: 2, PluginShape: setup.PluginShapeV1Only}) {
		t.Error("solo v1 en OpenCode 2.x debía contar como problema")
	}
	if openCodeProblem(setup.OpenCodeInstallStatus{Major: 2, PluginShape: setup.PluginShapeDual, ForeignV1Plugins: []string{"x.ts"}}) {
		t.Error("un plugin ajeno no es un problema de gomemory")
	}
	if openCodeProblem(setup.OpenCodeInstallStatus{PluginShape: setup.PluginShapeMissing}) {
		t.Error("sin OpenCode ni plugin no hay problema que reportar")
	}
}

// TestPrintDoctorHuman_CuentaElProblemaDeOpenCode: con el plugin v1 en
// OpenCode 2.x el inspector marca outdated sus canales, así que la salida
// humana cuenta el problema, propone la reinstalación y no declara "Sin
// problemas" (hallazgos de ACR sobre la feature 032).
func TestPrintDoctorHuman_CuentaElProblemaDeOpenCode(t *testing.T) {
	report := domain.CoverageReport{Channels: []domain.ActivationChannel{{
		Arm: domain.ArmGomemory, Agent: "opencode", Scope: domain.ScopeUser, Kind: domain.KindPlanEntry,
		State: domain.StateOutdated, Detail: "plugin solo v1: OpenCode 2.0.16 no lo carga",
	}}}
	out := captureStdout(t, func() { printDoctorHuman(report, &Deps{}) })
	if !strings.Contains(out, "1 problema(s)") {
		t.Errorf("el encabezado debía contar el problema de OpenCode:\n%s", out)
	}
	if strings.Contains(out, "Sin problemas") {
		t.Errorf("no debía declarar 'Sin problemas' con un problema de OpenCode:\n%s", out)
	}
	if !strings.Contains(out, "mem setup-mcp --scope global --agents opencode") {
		t.Errorf("debía proponer la reinstalación del plugin:\n%s", out)
	}
}

// TestPrintDoctorOpenCode_V1SinVersionNoEsVerde: sin versión detectada, un
// plugin solo v1 no se muestra como sano.
func TestPrintDoctorOpenCode_V1SinVersionNoEsVerde(t *testing.T) {
	out := captureStdout(t, func() {
		printDoctorOpenCode(setup.OpenCodeInstallStatus{PluginShape: setup.PluginShapeV1Only})
	})
	if strings.Contains(out, "✅") || !strings.Contains(out, "mem setup-mcp --scope global --agents opencode") {
		t.Errorf("solo v1 sin versión detectada debía pedir reinstalar, no mostrarse sano:\n%s", out)
	}
}
