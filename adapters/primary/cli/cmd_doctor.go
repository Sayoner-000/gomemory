package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

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
			out.Channels = append(out.Channels, doctorChannelJSON{
				Arm: string(c.Arm), Agent: c.Agent, Scope: string(c.Scope),
				Kind: string(c.Kind), State: string(c.State), Detail: c.Detail,
			})
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
	} else {
		printDoctorHuman(report)
	}

	if *strict && report.Problems() > 0 {
		os.Exit(1)
	}
}

func printDoctorHuman(report domain.CoverageReport) {
	fmt.Printf("mem doctor — %d canal(es), %d problema(s)\n\n", len(report.Channels), report.Problems())
	for _, c := range report.Channels {
		symbol := "✅"
		switch c.State {
		case domain.StateOutdated, domain.StateDuplicated, domain.StateMissing:
			symbol = "❌"
		case domain.StateNotApplicable:
			symbol = "➖"
		}
		fmt.Printf("  %s %-10s %-10s %-8s %-14s %s\n", symbol, c.Arm, c.Agent, c.Scope, c.Kind, c.Detail)
	}
	if len(report.Degradations) > 0 {
		fmt.Println("\nDegradaciones declaradas:")
		for _, d := range report.Degradations {
			fmt.Println("  - " + d)
		}
	}
}
