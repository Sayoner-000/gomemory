package cli

import (
	"os"
	"strings"
	"testing"

	"mem/domain"
)

// Regresión de un fallo REAL detectado ejecutando el binario, no la suite: el
// paquete flag detiene el parseo en el primer argumento que no empieza por "-".
// Con `route "objetivo" --class investigation --read-only`, todas las banderas
// quedaban dentro de fs.Args() y conservaban su valor por defecto en silencio.
// La política recibía una entrada válida pero equivocada y decidía INLINE
// siempre, sin que ninguna prueba de dominio pudiera verlo: su entrada estaba
// perfectamente bien formada.
func TestParseOctopusRouteFlags_BanderasDespuesDelObjetivo(t *testing.T) {
	req, asJSON, err := ParseOctopusRouteFlags([]string{
		"Determinar si la limpieza por expiración compite con el refresco",
		"--class", "investigation",
		"--read-only",
		"--files", "a.go,b.go",
		"--complexity", "high",
		"--id", "T042",
		"--json",
	})
	if err != nil {
		t.Fatalf("ParseOctopusRouteFlags: %v", err)
	}

	if req.Unit.Objective != "Determinar si la limpieza por expiración compite con el refresco" {
		t.Errorf("el objetivo se contaminó con las banderas: %q", req.Unit.Objective)
	}
	if req.Unit.Class != domain.ClassInvestigation {
		t.Errorf("Class = %q, esperaba %q", req.Unit.Class, domain.ClassInvestigation)
	}
	if req.Unit.ID != "T042" {
		t.Errorf("ID = %q, esperaba T042", req.Unit.ID)
	}
	if !req.Unit.Scope.ReadOnly {
		t.Error("--read-only no se aplicó")
	}
	if len(req.Unit.Scope.Files) != 2 {
		t.Errorf("Files = %v, esperaba 2 archivos", req.Unit.Scope.Files)
	}
	if req.Unit.Complexity != domain.LevelHigh {
		t.Errorf("Complexity = %v, esperaba high", req.Unit.Complexity)
	}
	if !asJSON {
		t.Error("--json no se aplicó")
	}
}

func TestParseOctopusRouteFlags_SinObjetivo(t *testing.T) {
	casos := [][]string{
		{},
		{"--class", "investigation"},
	}
	for _, args := range casos {
		if _, _, err := ParseOctopusRouteFlags(args); err == nil {
			t.Errorf("args=%v: se esperaba error por objetivo ausente", args)
		}
	}
}

// Las capacidades del runtime se declaran, no se detectan: --subagents=false
// debe llegar tal cual a la política.
func TestParseOctopusRouteFlags_CapacidadesDeclaradas(t *testing.T) {
	req, _, err := ParseOctopusRouteFlags([]string{"algo", "--subagents=false", "--max-parallel", "2"})
	if err != nil {
		t.Fatalf("ParseOctopusRouteFlags: %v", err)
	}
	if req.Capabilities.Subagents {
		t.Error("--subagents=false no se aplicó")
	}
	if req.Capabilities.MaxParallel != 2 {
		t.Errorf("MaxParallel = %d, esperaba 2", req.Capabilities.MaxParallel)
	}
}

func TestParseOctopusRouteFlags_DependenciasYResueltas(t *testing.T) {
	req, _, err := ParseOctopusRouteFlags([]string{"algo", "--deps", "T002, T003", "--resolved", "T002"})
	if err != nil {
		t.Fatalf("ParseOctopusRouteFlags: %v", err)
	}
	if len(req.Unit.Dependencies) != 2 {
		t.Fatalf("Dependencies = %v, esperaba 2", req.Unit.Dependencies)
	}
	if req.Unit.Dependencies[1] != "T003" {
		t.Errorf("los espacios alrededor de la coma deberían recortarse: %q", req.Unit.Dependencies[1])
	}
	if !req.Resolved["T002"] {
		t.Error("--resolved no pobló el conjunto")
	}
}

// La salida legible nunca presenta una estimación como medición (FR-033).
func TestRenderRouteDecision_MarcaLasEstimaciones(t *testing.T) {
	out := RenderRouteDecision(domain.RouteDecision{
		WorkUnitID:    "T004",
		Route:         domain.RouteDelegate,
		Reason:        domain.ReasonIsolatableInvestigation,
		ContextBudget: 2200,
		OutputBudget:  900,
		EstimatedCost: domain.CostEstimate{ContextTokens: 2200, OutputTokens: 900},
		Estimated:     true,
	}, nil)

	if !strings.Contains(out, "(estimado)") {
		t.Error("una cifra estimada debe declararse como tal")
	}
	if !strings.Contains(out, domain.ReasonIsolatableInvestigation.Text()) {
		t.Error("la salida debe incluir el texto de la razón")
	}
	if !strings.Contains(out, "Presupuesto de contexto: 2200") {
		t.Error("una ruta delegada debe mostrar su presupuesto de contexto")
	}
}

// Con ruta WAIT se enumeran las dependencias que bloquean.
func TestRenderRouteDecision_MuestraBloqueantes(t *testing.T) {
	out := RenderRouteDecision(domain.RouteDecision{
		WorkUnitID: "T005",
		Route:      domain.RouteWait,
		Reason:     domain.ReasonUnresolvedDependency,
		BlockedBy:  []string{"T002", "T003"},
	}, nil)
	if !strings.Contains(out, "T002, T003") {
		t.Errorf("faltan las dependencias bloqueantes en la salida:\n%s", out)
	}
}

// --- Historia 5: la configuración del proyecto acota la política ---

// Un tope configurado no puede relajarse pasando una bandera más alta: entre la
// línea de comandos y la configuración gana siempre el más restrictivo.
func TestCombinarPolitica_GanaElMasRestrictivo(t *testing.T) {
	casos := []struct {
		nombre          string
		bandera, ajuste int
		want            int
	}{
		{"solo bandera", 3, 0, 3},
		{"solo ajuste", 0, 2, 2},
		{"bandera más baja", 2, 5, 2},
		{"ajuste más bajo", 5, 2, 2},
		{"ninguno", 0, 0, 0},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := combinarPolitica(
				domain.PolicyOverrides{MaxSubagents: c.bandera},
				domain.PolicyOverrides{MaxSubagents: c.ajuste},
			)
			if got.MaxSubagents != c.want {
				t.Errorf("MaxSubagents = %d, esperaba %d", got.MaxSubagents, c.want)
			}
		})
	}
}

// Las banderas booleanas de la invocación se conservan al combinar: la
// configuración acota topes, no revoca decisiones puntuales del llamador.
func TestCombinarPolitica_ConservaLasBanderasDelLlamador(t *testing.T) {
	got := combinarPolitica(
		domain.PolicyOverrides{PreferInline: true, DelegationDisabled: true},
		domain.PolicyOverrides{MaxSubagents: 2},
	)
	if !got.PreferInline || !got.DelegationDisabled {
		t.Error("las banderas del llamador deben sobrevivir a la combinación")
	}
	if got.MaxSubagents != 2 {
		t.Errorf("MaxSubagents = %d, esperaba el tope configurado", got.MaxSubagents)
	}
}

// C-004 (ACR 027, reintento): antes de esta prueba, ningún punto de entrada
// real podía fijar AllowValidationReserve — la única forma de activarlo era
// inyectarlo a mano en un RouteInput de prueba. Prueba la superficie real:
// el flag de `mem octopus route`.
func TestParseOctopusRouteFlags_AutorizaReservaDeValidacion(t *testing.T) {
	req, _, err := ParseOctopusRouteFlags([]string{
		"investigar algo", "--allow-validation-reserve",
	})
	if err != nil {
		t.Fatalf("ParseOctopusRouteFlags: %v", err)
	}
	if !req.Policy.AllowValidationReserve {
		t.Error("--allow-validation-reserve debe reflejarse en PolicyOverrides.AllowValidationReserve")
	}
}

// Ausente = FALSE: el default sigue siendo "reserva intocable" (INV-AAR-006).
func TestParseOctopusRouteFlags_SinBanderaReservaProtegida(t *testing.T) {
	req, _, err := ParseOctopusRouteFlags([]string{"investigar algo"})
	if err != nil {
		t.Fatalf("ParseOctopusRouteFlags: %v", err)
	}
	if req.Policy.AllowValidationReserve {
		t.Error("sin la bandera, AllowValidationReserve debe seguir en false")
	}
}

// Mismo cierre para `mem octopus plan`, el otro punto de entrada CLI.
func TestParseOctopusPlanFlags_AutorizaReservaDeValidacion(t *testing.T) {
	_, overrides, _, _, err := ParseOctopusPlanFlags([]string{"--allow-validation-reserve"})
	if err != nil {
		t.Fatalf("ParseOctopusPlanFlags: %v", err)
	}
	if !overrides.AllowValidationReserve {
		t.Error("--allow-validation-reserve debe reflejarse en PolicyOverrides.AllowValidationReserve")
	}
}

// Regresión de ACR 029, hallazgo A-002: leerAlcance resolvía rutas relativas
// contra el cwd del proceso, no contra la raíz del proyecto. En `mem mcp
// --root <dir>` el cwd nunca cambia, así que el alcance declarado se leía
// vacío en silencio y el contexto medido caía a la frase del objetivo (~40
// tokens), cambiando la decisión de enrutamiento sin que nadie lo notara.
func TestLeerAlcance_ResuelveContraRootNoContraCwd(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(root+"/scope.go", []byte("package x // contenido de alcance"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got := leerAlcance(root, []string{"scope.go"})
	if !strings.Contains(got, "contenido de alcance") {
		t.Fatalf("leerAlcance(root, [\"scope.go\"]) = %q; se esperaba el contenido del archivo resuelto contra root", got)
	}

	absPath := root + "/scope.go"
	got = leerAlcance("/otra/raiz/que/no/existe", []string{absPath})
	if !strings.Contains(got, "contenido de alcance") {
		t.Fatalf("una ruta absoluta debe respetarse tal cual, sin unirla a root: got %q", got)
	}
}
