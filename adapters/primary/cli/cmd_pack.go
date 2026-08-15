package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// CmdPack despacha `mem pack <subcomando>`, mismo patrón que CmdSession.
func CmdPack(deps *Deps, args []string) {
	if len(args) == 0 {
		fail("subcomando requerido: build, show, compress, stats\nEjemplo: mem pack build --task \"...\" --max-tokens 4000")
	}

	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "build":
		cmdPackBuild(deps, subArgs)
	case "show":
		cmdPackShow(deps, subArgs)
	case "compress":
		cmdPackCompress(deps, subArgs)
	case "stats":
		cmdPackStats(deps, subArgs)
	default:
		fail("subcomando desconocido: %s (opciones: build, show, compress, stats)", sub)
	}
}

// ParsePackBuildFlags parsea los flags de `mem pack build` (contracts/cli.md).
// Separada de cmdPackBuild para poder probarla sin que un error dispare
// os.Exit (mismo patrón que ParsePurgeFlags).
func ParsePackBuildFlags(args []string, defaultProject string) (usecases.ContextRequest, bool, error) {
	fs := flag.NewFlagSet("pack build", flag.ContinueOnError)
	task := fs.String("task", "", "Descripción de la tarea (obligatorio)")
	maxTokens := fs.Int("max-tokens", 0, "Presupuesto total de tokens, > 0 (obligatorio)")
	project := fs.String("project", "", "Proyecto objetivo (default: proyecto actual)")
	minRelevance := fs.Float64("min-relevance", 0, "Relevancia mínima 0-1 (default: settings)")
	maxItems := fs.Int("max-items", 0, "Tope de candidatos antes de rankear (default: settings)")
	noCompress := fs.Bool("no-compress", false, "Desactivar compresión estructural (Compression=None)")
	noSpeckit := fs.Bool("no-speckit", false, "No incluir artefactos de Spec Kit")
	noCodeGraph := fs.Bool("no-code-graph", false, "Desactivar la señal de grafo de código externo")
	asJSON := fs.Bool("json", false, "Emitir el ContextPack como JSON")

	if err := fs.Parse(args); err != nil {
		return usecases.ContextRequest{}, false, err
	}
	if *task == "" {
		return usecases.ContextRequest{}, false, fmt.Errorf("--task es obligatorio")
	}
	if *maxTokens <= 0 {
		return usecases.ContextRequest{}, false, fmt.Errorf("--max-tokens debe ser > 0")
	}

	proj := *project
	if proj == "" {
		proj = defaultProject
	}

	compressionLevel := ports.CompressionStructural
	if *noCompress {
		compressionLevel = ports.CompressionNone
	}

	req := usecases.ContextRequest{
		Task:             *task,
		Project:          proj,
		MaxTokens:        *maxTokens,
		MinRelevance:     float32(*minRelevance),
		MaxItems:         *maxItems,
		IncludeSpecKit:   !*noSpeckit,
		IncludeCodeGraph: !*noCodeGraph,
		Compression:      compressionLevel,
	}
	return req, *asJSON, nil
}

func cmdPackBuild(deps *Deps, args []string) {
	req, asJSON, err := ParsePackBuildFlags(args, deps.Project)
	if err != nil {
		fail("%v", err)
	}
	req.Root = deps.Root
	req.CodeProviders = deps.CodeProviders

	pack, err := usecases.BuildContextPack(deps.MemoryRepo, deps.Compressor, deps.TokenCounter, deps.SpecKitReader, req)
	if err != nil {
		if err == domain.ErrCriticalContextOverflow {
			fail("el contenido crítico para esta tarea excede el presupuesto de %d tokens — subí --max-tokens o acotá la tarea", req.MaxTokens)
		}
		fail("construir contexto: %v", err)
	}

	if asJSON {
		data, err := json.MarshalIndent(pack, "", "  ")
		if err != nil {
			fail("serializar ContextPack: %v", err)
		}
		os.Stdout.Write(data)
		os.Stdout.WriteString("\n")
		return
	}

	fmt.Print(formatContextPack(pack))
}

// openPackInput abre el archivo indicado por args[0], o stdin si es "-" o no
// se pasó ningún argumento — mismo criterio que `mem pack compress` usa para
// texto: sin estado del lado del servidor, todo viene de un archivo/stdin
// explícito (contracts/cli.md).
func openPackInput(args []string) (io.ReadCloser, error) {
	if len(args) == 0 || args[0] == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(args[0])
}

// ParseContextPackInput deserializa un domain.ContextPack en JSON (el mismo
// formato que emite `mem pack build --json`) — usado por `pack show` y
// `pack stats`, que son puramente stateless: no recuerdan ningún paquete
// entre invocaciones (contracts/cli.md).
func ParseContextPackInput(r io.Reader) (domain.ContextPack, error) {
	var pack domain.ContextPack
	dec := json.NewDecoder(r)
	if err := dec.Decode(&pack); err != nil {
		return domain.ContextPack{}, fmt.Errorf("el input no es un ContextPack JSON válido: %w", err)
	}
	return pack, nil
}

func cmdPackShow(_ *Deps, args []string) {
	in, err := openPackInput(args)
	if err != nil {
		fail("abrir input: %v", err)
	}
	defer in.Close()

	pack, err := ParseContextPackInput(in)
	if err != nil {
		fail("%v", err)
	}
	fmt.Print(formatContextPack(pack))
}

func cmdPackStats(_ *Deps, args []string) {
	in, err := openPackInput(args)
	if err != nil {
		fail("abrir input: %v", err)
	}
	defer in.Close()

	pack, err := ParseContextPackInput(in)
	if err != nil {
		fail("%v", err)
	}
	fmt.Print(FormatContextStats(pack.Stats))
}

// CompressText corre solo el paso de compresión estructural sobre input, sin
// retrieval ni budget (contracts/cli.md `mem pack compress`).
func CompressText(compressor ports.Compressor, input string) (ports.CompressionResult, error) {
	return compressor.Compress(input, ports.CompressionOptions{
		Level:          ports.CompressionStructural,
		PreserveCode:   true,
		PreserveURLs:   true,
		PreservePaths:  true,
		PreserveErrors: true,
	})
}

func cmdPackCompress(deps *Deps, args []string) {
	in, err := openPackInput(args)
	if err != nil {
		fail("abrir input: %v", err)
	}
	defer in.Close()

	raw, err := io.ReadAll(in)
	if err != nil {
		fail("leer input: %v", err)
	}

	result, err := CompressText(deps.Compressor, string(raw))
	if err != nil {
		fail("comprimir: %v", err)
	}

	os.Stdout.WriteString(result.Content)
	fmt.Fprintf(os.Stderr, "tokens: %d → %d\n", result.RawTokens, result.Tokens)
}

// formatContextPack renderiza un domain.ContextPack en Markdown (items +
// bloque de estadísticas, contracts/cli.md), reusado por el CLI y por la
// tool MCP pack_build/pack_show para no duplicar el formato en dos lugares.
func formatContextPack(pack domain.ContextPack) string {
	var b strings.Builder
	for _, item := range pack.Items {
		heading := item.Source
		if heading == "" {
			heading = item.ID
		}
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", heading, item.Content)
	}
	b.WriteString(FormatContextStats(pack.Stats))
	return b.String()
}

// FormatContextStats renderiza el bloque de estadísticas de un
// domain.ContextStats (contracts/cli.md `mem pack stats`).
func FormatContextStats(stats domain.ContextStats) string {
	var b strings.Builder
	b.WriteString("GoMemory Context Optimization\n\n")
	fmt.Fprintf(&b, "Raw tokens:          %d\n", stats.RawTokens)
	fmt.Fprintf(&b, "Final tokens:        %d\n", stats.FinalTokens)
	fmt.Fprintf(&b, "Reducción:            %.2f%%\n", stats.CompressionRatio*100)
	fmt.Fprintf(&b, "Ahorrados:            %d\n\n", stats.SavedTokens)
	fmt.Fprintf(&b, "Items críticos:       %d\n", stats.ItemsCritical)
	fmt.Fprintf(&b, "Items relevantes:     %d\n", stats.ItemsRelevant)
	fmt.Fprintf(&b, "Items opcionales:     %d\n", stats.ItemsOptional)
	fmt.Fprintf(&b, "Duplicados removidos: %d\n", stats.ItemsDuplicate)
	fmt.Fprintf(&b, "Descartados:          %d\n", stats.ItemsDiscarded)
	return b.String()
}
