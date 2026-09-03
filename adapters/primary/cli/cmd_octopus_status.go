package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"mem/application/usecases"
	"mem/domain"
)

// --- `mem octopus status | usage | history` (feature 027) ---

// RenderOctopusStatus muestra los topes efectivos y los agregados de telemetría.
func RenderOctopusStatus(deps *Deps, s domain.RoutingStats) string {
	var b strings.Builder
	politica, reparto := politicaDesdeAjustes(deps)

	b.WriteString("Octopus AAR — estado\n\n")
	b.WriteString("Topes efectivos\n")
	fmt.Fprintf(&b, "  Agentes por plan: %d\n", politica.MaxSubagentsEfectivo())
	fmt.Fprintf(&b, "  Concurrencia: %d\n", politica.MaxParallelEfectivo())
	fmt.Fprintf(&b, "  Profundidad de delegación: %d\n", politica.MaxDepthEfectiva())
	fmt.Fprintf(&b, "  Reintentos por delegación: %d\n", politica.MaxRetriesEfectivo())
	fmt.Fprintf(&b, "  Reparto del presupuesto: %d %% principal · %d %% delegación · %d %% validación\n\n",
		reparto.MainAgentPct, reparto.DelegationPct, reparto.ValidationPct)

	if s.Decisiones == 0 {
		b.WriteString("Sin decisiones registradas todavía.\n")
		b.WriteString("El enrutador funciona igual sin historial: la política determinista no lo necesita.\n")
		return b.String()
	}

	b.WriteString(renderConteosPorRuta(s))
	b.WriteString(renderConsumo(s))
	return b.String()
}

func renderConteosPorRuta(s domain.RoutingStats) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Decisiones registradas: %d\n", s.Decisiones)
	// Orden fijo: la salida debe ser comparable entre corridas.
	for _, ruta := range domain.AllRoutes() {
		if n := s.PorRuta[ruta]; n > 0 {
			fmt.Fprintf(&b, "  %-9s %d\n", ruta, n)
		}
	}
	if s.AnchoParaleloMax > 0 {
		fmt.Fprintf(&b, "  Ancho de paralelismo observado: %d\n", s.AnchoParaleloMax)
	}
	b.WriteString("\n")
	return b.String()
}

func renderConsumo(s domain.RoutingStats) string {
	var b strings.Builder

	if s.ConReporte == 0 {
		// Sin ninguna medición real, la cifra estimada se muestra COMO
		// estimación y no se deriva de ella ningún ahorro (FR-033).
		fmt.Fprintf(&b, "Consumo estimado: %d tokens (estimado, sin reportes del runtime)\n", s.TokensEstimados)
		b.WriteString("Todavía no hay consumo real con el que contrastar: el ahorro no puede declararse.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "Consumo estimado: %d tokens (estimado)\n", s.TokensEstimados)
	fmt.Fprintf(&b, "Consumo real: %d tokens (medido sobre %d de %d decisiones)\n",
		s.TokensReales, s.ConReporte, s.Decisiones)

	if ahorro, ok := s.AhorroEstimado(); ok {
		signo := "ahorro"
		if ahorro < 0 {
			signo, ahorro = "exceso sobre lo estimado", -ahorro
		}
		fmt.Fprintf(&b, "Diferencia: %d tokens de %s (estimación contra medición parcial)\n", ahorro, signo)
	}
	fmt.Fprintf(&b, "Resultados: %d completados · %d fallidos · %d con contexto insuficiente\n",
		s.Exitos, s.Fallos, s.ContextoInsuf)
	if ratio, ok := s.RatioDeExito(); ok {
		fmt.Fprintf(&b, "Tasa de éxito: %.0f %%\n", ratio*100)
	}
	return b.String()
}

func cmdOctopusStatus(deps *Deps, args []string) {
	fs := flag.NewFlagSet("octopus status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "Emitir el estado como JSON")
	if err := fs.Parse(args); err != nil {
		fail("%v", err)
	}

	s := usecases.NewReportUseCase(deps.OctopusRepo).Stats(deps.Project)
	if *asJSON {
		emitirJSON(s)
		return
	}
	fmt.Print(RenderOctopusStatus(deps, s))
}

func cmdOctopusUsage(deps *Deps, args []string) {
	fs := flag.NewFlagSet("octopus usage", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "Emitir el consumo como JSON")
	if err := fs.Parse(args); err != nil {
		fail("%v", err)
	}

	s := usecases.NewReportUseCase(deps.OctopusRepo).Stats(deps.Project)
	if *asJSON {
		emitirJSON(s)
		return
	}
	if s.Decisiones == 0 {
		fmt.Println("Sin decisiones registradas todavía.")
		return
	}
	fmt.Print(renderConteosPorRuta(s))
	fmt.Print(renderConsumo(s))
}

func cmdOctopusHistory(deps *Deps, args []string) {
	fs := flag.NewFlagSet("octopus history", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	n := fs.Int("n", 20, "Cuántas decisiones mostrar")
	clase := fs.String("class", "", "Filtrar por clase de tarea")
	asJSON := fs.Bool("json", false, "Emitir el historial como JSON")
	if err := fs.Parse(args); err != nil {
		fail("%v", err)
	}

	hist := usecases.NewReportUseCase(deps.OctopusRepo).
		History(deps.Project, domain.TaskClass(*clase), *n)

	if *asJSON {
		emitirJSON(hist)
		return
	}
	fmt.Print(RenderOctopusHistory(hist))
}

// RenderOctopusHistory formatea el historial de decisiones y sus resultados.
func RenderOctopusHistory(hist []domain.ExecutionRecord) string {
	if len(hist) == 0 {
		return "Sin decisiones registradas todavía.\n"
	}

	var b strings.Builder
	for _, r := range hist {
		fmt.Fprintf(&b, "%s  %-9s %s\n", r.DecidedAt, r.Route, r.TaskID)
		fmt.Fprintf(&b, "    %s\n", r.Reason.Text())
		if !r.Reported {
			// Distinguir "sin reporte" de "reportó cero" evita leer un cero
			// como una medición que nadie hizo.
			fmt.Fprintf(&b, "    estimado %d tokens · sin reporte del runtime\n", r.EstimatedCost)
			continue
		}
		fmt.Fprintf(&b, "    estimado %d · real %d (contexto %d + salida %d) · %s",
			r.EstimatedCost, r.ContextTokens+r.OutputTokens, r.ContextTokens, r.OutputTokens, r.Status)
		if r.DurationMS > 0 {
			fmt.Fprintf(&b, " · %d ms", r.DurationMS)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// clasesDeLasUnidades indexa la clase de cada unidad para registrar el plan sin
// volver a recorrerlo desde el dominio.
func clasesDeLasUnidades(unidades []domain.WorkUnit) map[string]domain.TaskClass {
	out := make(map[string]domain.TaskClass, len(unidades))
	for _, u := range unidades {
		out[u.ID] = u.Class
	}
	return out
}
