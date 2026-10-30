package cli

import (
	"encoding/json"
	"errors"
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
		fail("subcomando requerido: build, show, compress, retrieve, savings, tune, purge, stats\nEjemplo: mem pack build --task \"...\" --max-tokens 4000")
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
	case "retrieve":
		cmdPackRetrieve(deps, subArgs)
	case "savings":
		cmdPackSavings(deps, subArgs)
	case "tune":
		cmdPackTune(deps, subArgs)
	case "purge":
		cmdPackPurge(deps, subArgs)
	default:
		fail("subcomando desconocido: %s (opciones: build, show, compress, retrieve, savings, tune, purge, stats)", sub)
	}
}

// ParsePackBuildFlags parsea los flags de `mem pack build` (contracts/cli.md).
// Separada de cmdPackBuild para poder probarla sin que un error dispare
// os.Exit (mismo patrón que ParsePurgeFlags).
func ParsePackBuildFlags(args []string, defaultProject string) (usecases.ContextRequest, bool, error) {
	fs := newFlagSet("pack build", flag.ContinueOnError)
	task := fs.String("task", "", "Descripción de la tarea (obligatorio)")
	maxTokens := fs.Int("max-tokens", 0, "Presupuesto total de tokens, > 0 (obligatorio)")
	project := fs.String("project", "", "Proyecto objetivo (default: proyecto actual)")
	minRelevance := fs.Float64("min-relevance", 0, "Relevancia mínima 0-1 (default: settings)")
	maxItems := fs.Int("max-items", 0, "Tope de candidatos antes de rankear (default: settings)")
	noCompress := fs.Bool("no-compress", false, "Desactivar compresión estructural (Compression=None)")
	noSpeckit := fs.Bool("no-speckit", false, "No incluir artefactos de Spec Kit")
	noCodeGraph := fs.Bool("no-code-graph", false, "Desactivar la señal de grafo de código externo")
	asJSON := fs.Bool("json", false, "Emitir el ContextPack como JSON")
	receipt := fs.Bool("receipt", false, "Incluir el recibo de procedencia y la contabilidad ampliada")

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
		Receipt:          *receipt,
	}
	return req, *asJSON, nil
}

func cmdPackBuild(deps *Deps, args []string) {
	req, asJSON, err := ParsePackBuildFlags(args, deps.Project)
	if err != nil {
		fail("%v", err)
	}
	req.Root = deps.Root
	// --no-compress manda; si no, el nivel efectivo del proyecto (feature 033).
	if req.Compression == ports.CompressionStructural {
		req.Compression = deps.CompressionLevel
	}
	req.CodeProviders = deps.CodeProviders
	req.Recorder = deps.UsageRecorder
	req.Relations = deps.RelationRepo

	out, err := buildPackOutput(deps, req, asJSON)
	if err != nil {
		fail("%v", err)
	}
	fmt.Print(out)
}

// buildPackOutput construye el paquete y devuelve la respuesta completa en el
// formato pedido (Markdown o JSON).
// El presupuesto se mide sobre esa misma serialización (HCD-001, Q3).
func buildPackOutput(deps *Deps, req usecases.ContextRequest, asJSON bool) (string, error) {
	format := formatContextPack
	switch {
	case asJSON && req.Receipt:
		format = formatContextPackJSON
	case asJSON:
		format = formatContextPackJSONWithoutReceipt
	}
	render := packRenderer(format)
	if !req.Receipt {
		// El presupuesto se mide sobre la misma salida, ya sin recibo.
		render = func(pack domain.ContextPack) string { return format(withoutReceipt(pack)) }
	}
	req.Renderer = render
	// Mismo almacén que pack_retrieve: las refs de omisión se recuperan con él.
	req.Originals = deps.OriginalStore
	pack, err := usecases.BuildContextPack(deps.MemoryRepo, deps.Compressor, deps.TokenCounter, deps.SpecKitReader, req)
	if err != nil {
		if errors.Is(err, domain.ErrCriticalContextOverflow) {
			return "", fmt.Errorf("el contenido crítico para esta tarea excede el presupuesto de %d tokens — subí --max-tokens o acotá la tarea: %w", req.MaxTokens, err)
		}
		return "", fmt.Errorf("construir contexto: %w", err)
	}
	return render(pack), nil
}

// withoutReceipt quita el recibo de procedencia y la contabilidad ampliada:
// por defecto no viajan porque encarecían cada pack (HCD-003).
func withoutReceipt(pack domain.ContextPack) domain.ContextPack {
	pack.Receipts = nil
	s := &pack.Stats
	s.SelectedRawTokens, s.SelectionSavedTokens, s.CompressionSavedTokens = 0, 0, 0
	s.DeltaSavedTokens, s.MetadataTokens, s.RecoveryTokens = 0, 0, 0
	s.Scope, s.CounterKind = "", ""
	return pack
}

// packStatsView son las estadísticas de siempre, sin la contabilidad ampliada:
// el JSON por defecto no la incluye (HCD-003). Conserva el orden de campos.
type packStatsView struct {
	RawTokens        int
	FinalTokens      int
	SavedTokens      int
	CompressionRatio float64
	ItemsRetrieved   int
	ItemsDuplicate   int
	ItemsCritical    int
	ItemsRelevant    int
	ItemsOptional    int
	ItemsDiscarded   int
}

// formatContextPackJSONWithoutReceipt es `mem pack build --json` sin --receipt.
func formatContextPackJSONWithoutReceipt(pack domain.ContextPack) string {
	s := pack.Stats
	view := struct {
		domain.ContextPack
		Stats packStatsView
	}{pack, packStatsView{s.RawTokens, s.FinalTokens, s.SavedTokens, s.CompressionRatio,
		s.ItemsRetrieved, s.ItemsDuplicate, s.ItemsCritical, s.ItemsRelevant, s.ItemsOptional, s.ItemsDiscarded}}
	data, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return ""
	}
	return string(data) + "\n"
}

// packRenderer adapta un formateador puro a ports.ContextPackRenderer.
type packRenderer func(domain.ContextPack) string

func (r packRenderer) Render(pack domain.ContextPack) string { return r(pack) }

// formatContextPackJSON es la salida de `mem pack build --json`. Un
// ContextPack siempre es serializable; ante un fallo imposible no inventa texto.
func formatContextPackJSON(pack domain.ContextPack) string {
	data, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		return ""
	}
	return string(data) + "\n"
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
	defer func() { _ = in.Close() }()

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
	defer func() { _ = in.Close() }()

	pack, err := ParseContextPackInput(in)
	if err != nil {
		fail("%v", err)
	}
	fmt.Print(FormatContextStats(pack.Stats))
}

// CompressText corre solo el paso de compresión estructural sobre input, sin
// retrieval ni budget (contracts/cli.md `mem pack compress`).
func CompressText(compressor ports.Compressor, input string) (ports.CompressionResult, error) {
	return CompressTextAt(compressor, ports.CompressionStructural, input)
}

// CompressTextAt comprime input con el nivel indicado (feature 033).
func CompressTextAt(compressor ports.Compressor, level ports.CompressionLevel, input string) (ports.CompressionResult, error) {
	return compressor.Compress(input, ports.CompressionOptions{
		Level:          level,
		PreserveCode:   true,
		PreservePaths:  true,
		PreserveURLs:   true,
		PreserveErrors: true,
	})
}

// formatContextPack renderiza un domain.ContextPack en Markdown (items +
// bloque de estadísticas, contracts/cli.md), reusado por el CLI y por la
// tool MCP pack_build/pack_show para no duplicar el formato en dos lugares.
func formatContextPack(pack domain.ContextPack) string {
	var b strings.Builder
	if pack.MatchStatus == "no_match" {
		b.WriteString("Sin coincidencias para la consulta; no se añade evidencia irrelevante.\n\n")
	}
	for _, item := range pack.Items {
		heading := item.Source
		if heading == "" {
			heading = item.ID
		}
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", heading, item.Content)
	}
	if len(pack.Receipts) > 0 {
		// El recibo es parte de la respuesta visible y de su presupuesto.
		data, err := json.Marshal(pack.Receipts)
		if err == nil {
			fmt.Fprintf(&b, "Procedencia (contenido protegido; SHA-256): %s\n\n", domain.ProtectText(string(data)))
		}
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
	if stats.Scope != "" {
		fmt.Fprintf(&b, "Selección: %d; compresión: %d; delta: %d; metadatos: %d; recuperaciones: %d (contador local, no facturación)\nÁmbito: %s\n", stats.SelectionSavedTokens, stats.CompressionSavedTokens, stats.DeltaSavedTokens, stats.MetadataTokens, stats.RecoveryTokens, stats.Scope)
	}
	return b.String()
}
