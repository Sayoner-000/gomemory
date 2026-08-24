package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"mem/adapters/primary/setup"
	"mem/application/usecases"
	"mem/domain"
	"mem/version"
)

// doctorChannelJSON es la forma JSON de un domain.ActivationChannel,
// documentada en contracts/doctor-report.md.
type doctorChannelJSON struct {
	Arm    string `json:"arm"`
	Agent  string `json:"agent"`
	Scope  string `json:"scope"`
	Kind   string `json:"kind"`
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`

	// Efecto y Remedio hacen accionable la salida legible por máquina (FR-007):
	// quien la consume no debería tener que reimplementar la traducción de
	// mecanismo a consecuencia. Solo se pueblan en los canales rotos: en uno
	// sano no hay nada que corregir, y en una degradación declarada tampoco.
	Efecto          string `json:"efecto,omitempty"`
	Remedio         string `json:"remedio,omitempty"`
	RemedioAdvierte string `json:"remedio_advierte,omitempty"`
}

type doctorReportJSON struct {
	Version      string              `json:"version"`
	Problems     int                 `json:"problems"`
	Channels     []doctorChannelJSON `json:"channels"`
	Degradations []string            `json:"degradations"`
}

// CmdDoctor implementa `mem doctor [--json] [--strict]`: el reporte de
// cobertura de los dos brazos (feature 019, contracts/doctor-report.md).
// Sin --strict, termina SIEMPRE con código 0 — un diagnóstico no debe romper
// el flujo de quien lo consulta.
func CmdDoctor(deps *Deps, args []string) {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "Salida JSON estable, para scripts")
	strict := fs.Bool("strict", false, "Terminar con código != 0 si hay problemas")
	if err := fs.Parse(args); err != nil {
		return
	}

	root := deps.Root
	if root == "" {
		root, _ = deps.ProjectRepo.FindRoot()
	}

	report := usecases.BuildActivationReport(setup.NewActivationInspector(), root)

	if *asJSON {
		out := doctorReportJSON{
			Version:      version.Version,
			Problems:     report.Problems(),
			Degradations: report.Degradations,
		}
		if out.Degradations == nil {
			out.Degradations = []string{}
		}
		for _, c := range report.Channels {
			fila := doctorChannelJSON{
				Arm: string(c.Arm), Agent: c.Agent, Scope: string(c.Scope),
				Kind: string(c.Kind), State: string(c.State), Detail: c.Detail,
			}
			switch c.State {
			case domain.StateOutdated, domain.StateDuplicated, domain.StateMissing:
				if c.Arm == domain.ArmGomemory {
					r := domain.CorreccionPara(c.Agent, c.Scope)
					fila.Efecto = domain.EfectoDelCanal(c.Kind)
					fila.Remedio = r.Comando
					fila.RemedioAdvierte = r.Advertencia
				}
			}
			out.Channels = append(out.Channels, fila)
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
	} else {
		printDoctorHuman(report, deps)
	}

	if *strict && report.Problems() > 0 {
		os.Exit(1)
	}
}

func printDoctorHuman(report domain.CoverageReport, deps *Deps) {
	fmt.Printf("mem doctor — %d canal(es), %d problema(s)\n\n", len(report.Channels), report.Problems())
	for _, c := range report.Channels {
		fmt.Printf("  %s %-10s %-10s %-8s %-14s %s\n", doctorSymbol(c.State), c.Arm, c.Agent, c.Scope, c.Kind, c.Detail)
	}
	if len(report.Degradations) > 0 {
		fmt.Println("\nDegradaciones declaradas (no requieren acción):")
		for _, d := range report.Degradations {
			fmt.Println("  - " + d)
		}
	}
	printDoctorRemedies(report)
	printDoctorLiveness(deps)
}

func doctorSymbol(state domain.ChannelState) string {
	switch state {
	case domain.StateOutdated, domain.StateDuplicated, domain.StateMissing:
		return "❌"
	case domain.StateNotApplicable:
		return "➖"
	default:
		return "✅"
	}
}

// printDoctorRemedies explica qué se pierde en cada canal roto y cómo
// restablecerlo.
//
// Antes, el informe nombraba el archivo ausente y nada más: quien lo leía tenía
// que conocer el sistema por dentro para saber si le importaba y para encontrar
// el comando. Las correcciones se agrupan porque varios canales caídos suelen
// restablecerse con el mismo comando (FR-004); repetirlo una vez por canal
// sugeriría que hay que ejecutarlo varias veces.
func printDoctorRemedies(report domain.CoverageReport) {
	type grupo struct {
		correccion domain.Correccion
		canales    []string
		efectos    []string
	}
	var orden []string
	grupos := map[string]*grupo{}

	for _, c := range report.Channels {
		switch c.State {
		case domain.StateOutdated, domain.StateDuplicated, domain.StateMissing:
		default:
			continue
		}
		if c.Arm != domain.ArmGomemory {
			continue // el brazo extensor es de solo lectura: se observa, no se corrige
		}
		r := domain.CorreccionPara(c.Agent, c.Scope)
		if _, visto := grupos[r.Comando]; !visto {
			grupos[r.Comando] = &grupo{correccion: r}
			orden = append(orden, r.Comando)
		}
		g := grupos[r.Comando]
		g.canales = append(g.canales, fmt.Sprintf("%s · %s · %s", c.Agent, c.Scope, c.Kind))
		g.efectos = append(g.efectos, domain.EfectoDelCanal(c.Kind))
	}

	if len(orden) == 0 {
		if report.Problems() == 0 {
			fmt.Println("\n✅ Sin problemas: todos los canales activos funcionan.")
		}
		return
	}

	fmt.Println("\nQué hacer:")
	for _, cmd := range orden {
		g := grupos[cmd]
		fmt.Printf("\n  Afecta a %d canal(es): %s\n", len(g.canales), strings.Join(g.canales, ", "))
		for _, e := range dedupeStrings(g.efectos) {
			fmt.Printf("    • %s\n", e)
		}
		if g.correccion.Advertencia != "" {
			fmt.Printf("    ⚠️  %s\n", g.correccion.Advertencia)
		}
		fmt.Printf("    → %s\n", g.correccion.Comando)
	}
}

// dedupeStrings conserva el orden y descarta repeticiones: varios canales del
// mismo agente suelen perder lo mismo, y listarlo dos veces no añade nada.
func dedupeStrings(in []string) []string {
	vistos := map[string]bool{}
	var out []string
	for _, s := range in {
		if vistos[s] {
			continue
		}
		vistos[s] = true
		out = append(out, s)
	}
	return out
}

// canalesDeInyeccion son los canales cuya salud no se puede deducir de que el
// artefacto exista: dependen de que el agente los invoque de verdad.
//
// claude deja rastro desde hookPlanEntered (su única puerta de plan_entry) y
// opencode desde el complemento; sin esos rastros, un canal roto era
// indistinguible de uno sano.
var canalesDeInyeccion = []struct{ agent, scope, kind string }{
	{"opencode", "user", "plan_entry"},
	{"claude", "user", "plan_entry"},
}

// printDoctorLiveness reporta los canales que no responden.
//
// Solo se imprime cuando hay algo que decir: un canal sano, o inactivo porque
// nadie trabajó, no merece una línea. El informe gana credibilidad al no
// alarmar sin motivo (FR-011).
func printDoctorLiveness(deps *Deps) {
	if deps == nil || deps.ChannelActivity == nil {
		return
	}
	umbral := domain.DefaultLivenessThreshold
	ahora := time.Now()

	var avisos []string
	for _, c := range canalesDeInyeccion {
		fired, lastErr, ok := deps.ChannelActivity.Last(c.agent, c.scope, c.kind)
		if !ok && lastErr == "" && fired.IsZero() {
			// Sin ningún registro: puede que este agente no se use aquí.
			// Se consulta igual el número de sesiones para no callar un canal
			// que nunca respondió habiendo trabajo.
			if deps.ChannelActivity.SessionsSince(ahora.Add(-umbral)) == 0 {
				continue
			}
		}
		desde := fired
		if desde.IsZero() {
			desde = ahora.Add(-umbral)
		}
		sesiones := deps.ChannelActivity.SessionsSince(desde)
		veredicto, detalle := domain.EvaluateLiveness(fired, sesiones, umbral, ahora)
		if veredicto != domain.LivenessStale {
			continue
		}
		aviso := fmt.Sprintf("  ⚠️  %s · %s · %s: %s", c.agent, c.scope, c.kind, detalle)
		if lastErr != "" {
			aviso += fmt.Sprintf("\n      último fallo registrado: %s", lastErr)
		}
		aviso += fmt.Sprintf("\n      %s", domain.EfectoDelCanal(domain.ChannelKind(c.kind)))
		r := domain.CorreccionPara(c.agent, domain.AgentScope(c.scope))
		aviso += fmt.Sprintf("\n      → %s", r.Comando)
		avisos = append(avisos, aviso)
	}

	if len(avisos) == 0 {
		return
	}
	fmt.Println("\nCanales que no responden:")
	for _, a := range avisos {
		fmt.Println(a)
	}
}
