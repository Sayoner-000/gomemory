package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"mem/adapters/primary/setup"
	"mem/application/usecases"
	"mem/domain"
)

// doctorState reúne lo que `mem doctor` diagnostica. Se calcula una vez para
// el informe y otra tras `--fix`, para comparar el antes y el después.
type doctorState struct {
	deps             *Deps
	root, home       string
	report           domain.CoverageReport
	openCode         setup.OpenCodeInstallStatus
	problems         int
	compression      doctorCompressionJSON
	binary           doctorBinaryJSON
	dupHooks         []string
	codeGraphMissing []string
}

// Puntos de sustitución para las pruebas: el diagnóstico, el ejecutor de
// subprocesos, la retirada de hooks duplicados y la entrada de la persona.
var (
	collectDoctor               = collectDoctorState
	runFixCommand               = execFixCommand
	dedupClaudeHooks            = setup.DedupClaudeProjectHooks
	fixConfirmIn      io.Reader = os.Stdin
	fixCommandTimeout           = 5 * time.Minute
)

// collectDoctorState ejecuta todas las inspecciones del doctor.
func collectDoctorState(deps *Deps, root string) doctorState {
	st := doctorState{deps: deps, root: root}
	st.home, _ = os.UserHomeDir()
	st.report = usecases.BuildActivationReport(setup.NewActivationInspector(), root)
	st.openCode = inspectOpenCode()
	// Un plugin de OpenCode que no carga ya cuenta en report.Problems(): el
	// inspector marca outdated los canales que sostiene (contrato 019).
	st.problems = st.report.Problems()
	if deps.InstinctGlobalErr != nil {
		st.report.Degradations = append(st.report.Degradations, "instincts globales no disponibles ("+
			domain.StripTerminalControls(deps.InstinctGlobalErr.Error())+"): mem expand instincts y mem sanitize fallarán hasta reparar instincts.db")
	}
	st.compression = buildDoctorCompression(deps, root)
	st.problems += len(st.compression.Problems)
	st.binary = buildDoctorBinary(deps, root)
	if st.home != "" && root != "" {
		st.dupHooks = setup.DuplicateClaudeHookSubs(st.home, root)
		if deps.SettingsRepo != nil && codeGraphOn(deps.SettingsRepo.Read(root)) {
			st.codeGraphMissing = codeGraphMissing(codeGraphMCPState(st.home, root))
		}
	}
	return st
}

// fixStep es una acción del plan con su forma de ejecutarse: por subproceso
// (argv) o en proceso (run).
type fixStep struct {
	accion domain.FixAction
	argv   []string
	run    func(context.Context) error
}

// installFixCommand es la corrección de proyecto con --yes: la persona ya
// confirmó en `mem doctor --fix`, no hay que volver a preguntarle.
const installFixCommand = "mem install . --yes"

// buildFixPlan traduce el diagnóstico en acciones. Solo incluye lo que
// gomemory gestiona: nunca los canales del brazo codegraph (INV-1), ni los
// plugins ajenos de OpenCode, ni las copias locales (se retiran solas).
func buildFixPlan(st doctorState) []fixStep {
	var acciones []domain.FixAction
	ejecutores := map[string]fixStep{}
	add := func(s fixStep) {
		acciones = append(acciones, s.accion)
		if _, ok := ejecutores[s.accion.Comando]; !ok {
			ejecutores[s.accion.Comando] = s
		}
	}

	for _, c := range st.report.Channels {
		if c.Arm != domain.ArmGomemory {
			continue
		}
		switch c.State {
		case domain.StateOutdated, domain.StateDuplicated, domain.StateMissing:
		default:
			continue
		}
		motivo := fmt.Sprintf("%s · %s · %s (%s)", c.Agent, c.Scope, c.Kind, c.State)
		if c.Scope == domain.ScopeUser {
			r := domain.CorreccionPara(c.Agent, c.Scope)
			add(fixStep{
				accion: domain.FixAction{Comando: r.Comando, Ambito: domain.ScopeUser, Advertencia: r.Advertencia, Motivos: []string{motivo}},
				argv:   []string{"mem", "setup-mcp", "--scope", "global", "--agents", c.Agent},
			})
			continue
		}
		add(fixStep{
			accion: domain.FixAction{Comando: installFixCommand, Ambito: domain.ScopeProject, Motivos: []string{motivo}},
			argv:   []string{"mem", "install", ".", "--yes"},
		})
	}

	// Sin forma de plugin conocida no hubo inspección: nada que reparar.
	if st.openCode.PluginShape != "" && (openCodeProblem(st.openCode) || st.openCode.StaleTestArtifact) {
		motivo := "plugin de OpenCode que no carga en la versión instalada"
		if !openCodeProblem(st.openCode) {
			motivo = "artefacto de pruebas gomemory.test.mjs en la carpeta de plugins"
		}
		add(fixStep{
			accion: domain.FixAction{Comando: openCodeRemedy, Ambito: domain.ScopeUser, Motivos: []string{motivo}},
			argv:   []string{"mem", "setup-mcp", "--scope", "global", "--agents", "opencode"},
		})
	}

	if len(st.dupHooks) > 0 {
		home, root := st.home, st.root
		add(fixStep{
			accion: domain.FixAction{
				Comando: "retirar del .claude/settings.json del proyecto los hooks de gomemory que ya registra el global",
				Ambito:  domain.ScopeProject,
				Motivos: []string{"hooks duplicados (cada evento corre dos veces): " + joinComma(st.dupHooks)},
			},
			run: func(context.Context) error {
				_, err := dedupClaudeHooks(home, root)
				return err
			},
		})
	}

	if st.compression.NearLimit {
		deps := st.deps
		add(fixStep{
			accion: domain.FixAction{Comando: "mem pack purge", Ambito: domain.ScopeProject, Motivos: []string{"almacén de originales al 90 % del tope"}},
			run: func(ctx context.Context) error {
				if deps == nil || deps.OriginalStore == nil {
					return fmt.Errorf("almacén de originales no disponible")
				}
				_, _, err := deps.OriginalStore.Purge(ctx)
				return err
			},
		})
	}

	if len(st.codeGraphMissing) > 0 {
		add(fixStep{
			accion: domain.FixAction{
				Comando: codeGraphManual(st.codeGraphMissing),
				Ambito:  domain.ScopeUser,
				Motivos: []string{"servidor MCP de CodeGraph sin conectar en " + joinComma(st.codeGraphMissing)},
			},
			argv: append([]string{"codegraph"}, codeGraphInstallArgs(st.codeGraphMissing)...),
		})
	}

	plan := domain.PlanDeReparacion(acciones)
	steps := make([]fixStep, 0, len(plan))
	for _, a := range plan {
		s := ejecutores[a.Comando]
		s.accion = a
		steps = append(steps, s)
	}
	return steps
}

// fixMode decide cómo se tratan las acciones del plan.
type fixMode int

const (
	// fixSimulate muestra el plan sin ejecutar (--dry-run, o --json sin --yes).
	fixSimulate fixMode = iota
	// fixConfirm pide una confirmación para todas las acciones del proyecto y
	// una por cada acción que afecta a toda la máquina.
	fixConfirm
	// fixAll aplica todo sin preguntar (--yes).
	fixAll
)

// Resultados de cada acción en doctorFixActionJSON.Resultado.
const (
	fixApplied   = "aplicada"
	fixFailed    = "fallida"
	fixSkipped   = "omitida"
	fixSimulated = "simulada"
)

type doctorFixActionJSON struct {
	Comando     string   `json:"comando"`
	Ambito      string   `json:"ambito"`
	Advertencia string   `json:"advertencia,omitempty"`
	Motivos     []string `json:"motivos"`
	Resultado   string   `json:"resultado"`
	Error       string   `json:"error,omitempty"`
}

// doctorFixReportJSON es la clave "fixes" de `mem doctor --fix --json`.
type doctorFixReportJSON struct {
	Acciones         []doctorFixActionJSON `json:"acciones"`
	ProblemasAntes   int                   `json:"problemas_antes"`
	ProblemasDespues int                   `json:"problemas_despues"`
	// Pendientes son las acciones que el doctor seguiría proponiendo.
	Pendientes int `json:"pendientes"`
}

// runDoctorFix ejecuta el plan según mode y, si aplicó algo, vuelve a
// diagnosticar. Devuelve el estado final para el informe. show imprime el
// plan, las preguntas y el resumen (no en JSON).
func runDoctorFix(deps *Deps, root string, st doctorState, mode fixMode, show bool) (doctorState, doctorFixReportJSON) {
	steps := buildFixPlan(st)
	rep := doctorFixReportJSON{Acciones: []doctorFixActionJSON{}, ProblemasAntes: st.problems}
	if len(steps) == 0 {
		if show {
			humanf("Reparación: nada que reparar en lo que gestiona gomemory.\n\n")
		}
		rep.ProblemasDespues = st.problems
		return st, rep
	}

	if show {
		printFixPlan(steps, mode == fixSimulate)
	}

	decisions := decideFixSteps(steps, mode, show)
	applied := 0
	for i, s := range steps {
		a := doctorFixActionJSON{
			Comando: s.accion.Comando, Ambito: string(s.accion.Ambito),
			Advertencia: s.accion.Advertencia, Motivos: s.accion.Motivos,
		}
		switch {
		case mode == fixSimulate:
			a.Resultado = fixSimulated
		case !decisions[i]:
			a.Resultado = fixSkipped
		default:
			if err := runFixStep(root, s); err != nil {
				a.Resultado, a.Error = fixFailed, domain.StripTerminalControls(err.Error())
			} else {
				a.Resultado = fixApplied
				applied++
			}
		}
		if show && mode != fixSimulate {
			printFixResult(a)
		}
		rep.Acciones = append(rep.Acciones, a)
	}

	if applied > 0 {
		st = collectDoctor(deps, root)
		rep.Pendientes = len(buildFixPlan(st))
	} else {
		rep.Pendientes = len(steps)
	}
	rep.ProblemasDespues = st.problems
	if show && mode != fixSimulate {
		humanf("\nReparación: problemas %d → %d · acciones pendientes: %d\n", rep.ProblemasAntes, rep.ProblemasDespues, rep.Pendientes)
		if applied > 0 && rep.Pendientes > 0 {
			// Una reparación puede habilitar otra (p. ej. CodeGraph MCP solo se
			// propone cuando ya existe la configuración del agente).
			humanln("  quedan acciones nuevas o no aplicadas: vuelve a ejecutar mem doctor --fix")
		}
		humanln()
	}
	return st, rep
}

// decideFixSteps pregunta lo necesario: una vez para el proyecto y una por
// cada acción de usuario, mostrando su advertencia. Una sola lectura
// compartida, para que cada respuesta corresponda a su pregunta.
func decideFixSteps(steps []fixStep, mode fixMode, show bool) []bool {
	decisions := make([]bool, len(steps))
	switch mode {
	case fixSimulate:
		return decisions
	case fixAll:
		for i := range decisions {
			decisions[i] = true
		}
		return decisions
	}

	in := bufio.NewReader(fixConfirmIn)
	project := 0
	for _, s := range steps {
		if s.accion.Ambito != domain.ScopeUser {
			project++
		}
	}
	if project > 0 {
		ok := confirmLine(in, fmt.Sprintf("¿Aplicar las %d acción(es) de este proyecto?", project), show)
		for i, s := range steps {
			if s.accion.Ambito != domain.ScopeUser {
				decisions[i] = ok
			}
		}
	}
	for i, s := range steps {
		if s.accion.Ambito == domain.ScopeUser {
			decisions[i] = confirmLine(in, fmt.Sprintf("¿Aplicar `%s`? (%s)", s.accion.Comando, s.accion.Advertencia), show)
		}
	}
	return decisions
}

// confirmLine lee una respuesta de in; solo "si" o "sí" confirman, como en
// ConfirmAction. Sin entrada (EOF), no se confirma nada.
func confirmLine(in *bufio.Reader, prompt string, show bool) bool {
	if show {
		fmt.Printf("%s [escribe 'si' para confirmar]: ", prompt)
	}
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		if show {
			fmt.Println()
		}
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "si" || answer == "sí"
}

func runFixStep(root string, s fixStep) error {
	ctx, cancel := context.WithTimeout(context.Background(), fixCommandTimeout)
	defer cancel()
	if s.run != nil {
		return s.run(ctx)
	}
	return runFixCommand(ctx, root, s.argv)
}

// execFixCommand ejecuta una acción por subproceso en la raíz del proyecto.
// "mem" es este mismo binario; "codegraph" se valida antes, como en
// ensureCodeGraphMCP. La salida se descarta salvo para explicar un fallo.
func execFixCommand(ctx context.Context, root string, argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("acción sin comando")
	}
	prog := argv[0]
	switch prog {
	case "mem":
		self, err := os.Executable()
		if err != nil {
			return fmt.Errorf("localizar el binario de mem: %w", err)
		}
		prog = self
	case "codegraph":
		if !codeGraphInstallerOK(ctx) {
			return fmt.Errorf("el codegraph del PATH no ofrece install --target/--location")
		}
	}
	cmd := exec.CommandContext(ctx, prog, argv[1:]...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		if last := lastLine(string(out)); last != "" {
			return fmt.Errorf("%w: %s", err, last)
		}
		return err
	}
	return nil
}

func printFixPlan(steps []fixStep, simulated bool) {
	title := "Plan de reparación"
	if simulated {
		title += " (simulación: no se ejecuta nada)"
	}
	humanf("%s — %d acción(es):\n", title, len(steps))
	for i, s := range steps {
		ambito := "proyecto"
		if s.accion.Ambito == domain.ScopeUser {
			ambito = "máquina"
		}
		humanf("  %d. [%s] %s\n", i+1, ambito, s.accion.Comando)
		for _, m := range s.accion.Motivos {
			humanf("       corrige: %s\n", m)
		}
		if s.accion.Advertencia != "" {
			humanf("       ⚠ %s\n", s.accion.Advertencia)
		}
	}
	humanln()
}

func printFixResult(a doctorFixActionJSON) {
	switch a.Resultado {
	case fixApplied:
		humanf("  ✓ %s\n", a.Comando)
	case fixFailed:
		humanf("  ✗ %s — %s\n", a.Comando, a.Error)
	default:
		humanf("  ○ %s — %s\n", a.Comando, a.Resultado)
	}
}

func joinComma(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
