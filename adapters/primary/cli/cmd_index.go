package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"mem/adapters/primary/console"
	"mem/application/ports"
	"mem/application/usecases"
)

// CmdIndex indexa manualmente el código Go del proyecto (`mem index [--force]`)
// y, salvo --skip-graph, refresca también el grafo de código externo
// (opcional, multi-lenguaje) para cada proveedor configurado que lo soporte.
// El indexado incremental normal ocurre solo vía el hook turn-end tras cada
// turno del agente; este comando es para correrlo a demanda (primera carga
// de un proyecto grande, o forzar un reindexado completo con --force).
func CmdIndex(deps *Deps, args []string) {
	fs := newFlagSet("index", flag.ContinueOnError)
	force := fs.Bool("force", false, "Reindexar todos los archivos aunque no hayan cambiado")
	skipGraph := fs.Bool("skip-graph", false, "Omitir el refresco del grafo de código externo")
	if err := fs.Parse(args); err != nil {
		return
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}
	project := deps.ProjectRepo.Key(root)

	flow := console.NewFlow(os.Stdout, console.DetectEnv(), "Indexar proyecto")
	defer flow.Stop()
	steps := []string{goStep}
	if !*skipGraph {
		steps = append(steps, externalGraphSteps(deps)...)
	}
	flow.Plan(steps...)
	flow.Next("mem context · mem tui")

	flow.Progress(goStep)
	ix := usecases.NewIndexer(deps.CodeGraphRepo, root, project)
	report, err := ix.IndexProject(*force)
	if err != nil {
		flow.Done(console.StepResult{Name: goStep, Status: console.StepFail, Detail: err.Error()})
		flow.End("fail")
		os.Exit(1)
	}
	flow.Done(console.StepResult{Name: goStep, Detail: goDetail(report), Meta: goMeta(report)})

	if !*skipGraph {
		indexExternalGraph(deps, flow)
	}
	flow.End("")
}

const goStep = "Código Go"

// goDetail resume solo los conteos con contenido: "0 parseados" no informa.
func goDetail(r usecases.IndexReport) string {
	parts := []string{}
	for _, c := range []struct {
		n         int
		one, many string
	}{{r.Parsed, "parseado", "parseados"}, {r.Skipped, "sin cambios", "sin cambios"}, {r.Deleted, "eliminado", "eliminados"}} {
		if c.n == 1 {
			parts = append(parts, "1 "+c.one)
		} else if c.n > 1 {
			parts = append(parts, console.FormatInt(c.n)+" "+c.many)
		}
	}
	return strings.Join(parts, " · ")
}

// goMeta solo informa nodos y aristas cuando hubo archivos parseados: el
// reporte cuenta lo reindexado, no el total del grafo.
func goMeta(r usecases.IndexReport) string {
	if r.Parsed == 0 {
		return r.Duration.Round(time.Millisecond).String()
	}
	return graphMeta(r.Nodes, r.Edges, r.Duration)
}

func graphMeta(nodes, edges int, d time.Duration) string {
	return fmt.Sprintf("%s nodos · %s aristas · %s", console.FormatInt(nodes), console.FormatInt(edges), d.Round(time.Millisecond))
}

// externalGraphSteps anticipa los pasos del grafo externo para que el plan
// muestre desde el inicio cuántos quedan.
func externalGraphSteps(deps *Deps) []string {
	steps := make([]string, 0, len(deps.CodeProviders)+1)
	for _, provider := range deps.CodeProviders {
		steps = append(steps, provider.Name())
	}
	if missingCodeGraphHint(deps) != "" {
		steps = append(steps, "codegraph")
	}
	return steps
}

// indexExternalGraph refresca el grafo de código externo (opcional,
// multi-lenguaje) tras el indexado nativo Go. El indexado nativo ya se
// completó con éxito y es el resultado principal del comando: cualquier
// problema con un proveedor externo se reporta como paso omitido (no
// instalado) o aviso (fallo real), pero nunca hace fallar el comando ni
// impide indexar los siguientes — el exit code sigue en 0.
func indexExternalGraph(deps *Deps, flow *console.Flow) {
	for _, provider := range deps.CodeProviders {
		indexer, ok := provider.(ports.CodeGraphIndexer)
		if !ok {
			flow.Done(console.StepResult{Name: provider.Name(), Status: console.StepSkip, Detail: "no soporta reindexado"})
			continue
		}
		flow.Progress(indexer.Name())
		started := time.Now()
		nodes, edges, err := indexer.IndexRepository(context.Background(), "full")
		if err != nil {
			if errors.Is(err, ports.ErrIndexerNotInstalled) {
				flow.Done(console.StepResult{Name: indexer.Name(), Status: console.StepSkip, Detail: "no instalado en PATH"})
				continue
			}
			flow.Done(console.StepResult{Name: indexer.Name(), Status: console.StepWarn, Detail: err.Error()})
			continue
		}
		flow.Done(console.StepResult{Name: indexer.Name(), Meta: graphMeta(nodes, edges, time.Since(started))})
	}
	if missingCodeGraphHint(deps) != "" {
		flow.Done(console.StepResult{Name: "codegraph", Status: console.StepSkip, Detail: "no instalado", Manual: codeGraphInstallHint})
	}
}

// codeGraphInstallHint es el instalador oficial de CodeGraph; `codegraph
// install` registra después su integración con los agentes.
const codeGraphInstallHint = "curl -fsSL https://raw.githubusercontent.com/colbymchenry/codegraph/main/install.sh | sh"

// missingCodeGraphHint avisa cuando CodeGraph no entró en la autodetección
// por no estar en PATH. Con una lista explícita de proveedores calla: quien
// la fijó ya eligió qué grafos usar.
func missingCodeGraphHint(deps *Deps) string {
	for _, provider := range deps.CodeProviders {
		if provider.Name() == "codegraph" {
			return ""
		}
	}
	if deps.SettingsRepo != nil && len(deps.SettingsRepo.Read(deps.Root).CodeGraphProviders) > 0 {
		return ""
	}
	return "codegraph · no instalado → " + codeGraphInstallHint
}
