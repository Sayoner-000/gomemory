package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// levelArg resuelve --level: vacío = el nivel efectivo del proyecto.
func levelArg(deps *Deps, raw string) (ports.CompressionLevel, error) {
	if strings.TrimSpace(raw) == "" {
		return deps.CompressionLevel, nil
	}
	l := strings.ToLower(strings.TrimSpace(raw))
	if !domain.ValidCompressionLevel(l) {
		return 0, fmt.Errorf("--level=%q no válido: usa none, structural o max", raw)
	}
	return ports.CompressionLevelFromSetting(l), nil
}

// compressJSONOutput es la forma de `mem pack compress --json`
// (contracts/cli-and-mcp.md).
type compressJSONOutput struct {
	Content          string   `json:"content"`
	RawTokens        int      `json:"raw_tokens"`
	StructuralTokens int      `json:"structural_tokens"`
	Tokens           int      `json:"tokens"`
	Compressor       string   `json:"compressor"`
	ContentType      string   `json:"content_type"`
	Refs             []string `json:"refs"`
	FallbackReason   string   `json:"fallback_reason"`
}

// FormatTokensLine es la línea de resumen de `mem pack compress` y de la tool
// pack_compress: "tokens: 18788 → 4210 (json, -77.6%)".
func FormatTokensLine(r ports.CompressionResult) string {
	name := r.Compressor
	if name == "" {
		name = "structural"
	}
	pct := 0.0
	if r.RawTokens > 0 {
		pct = 100 * (1 - float64(r.Tokens)/float64(r.RawTokens))
	}
	line := fmt.Sprintf("tokens: %d → %d (%s, -%.1f%%)", r.RawTokens, r.Tokens, name, pct)
	if r.FallbackReason != "" {
		line += " · fallback " + r.FallbackReason
	}
	return line
}

// FormatCompare es la tabla de --compare / compare=true: los tres niveles
// sobre la misma entrada, sin el contenido.
func FormatCompare(compressor ports.Compressor, input string) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "nivel\ttokens\tahorro\tcompresor")
	levels := []struct {
		name  string
		level ports.CompressionLevel
	}{{"ninguna", ports.CompressionNone}, {"estructural", ports.CompressionStructural}, {"máxima", ports.CompressionMax}}
	for _, l := range levels {
		r, err := CompressTextAt(compressor, l.level, input)
		if err != nil {
			_, _ = fmt.Fprintf(w, "%s\terror\t—\t%v\n", l.name, err)
			continue
		}
		saving := "—"
		if l.level != ports.CompressionNone && r.RawTokens > 0 {
			saving = fmt.Sprintf("%.1f%%", 100*(1-float64(r.Tokens)/float64(r.RawTokens)))
		}
		name := r.Compressor
		if r.FallbackReason != "" {
			name += " (" + r.FallbackReason + ")"
		}
		_, _ = fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", l.name, r.Tokens, saving, name)
	}
	_ = w.Flush()
	return b.String()
}

func cmdPackCompress(deps *Deps, args []string) {
	fs := flag.NewFlagSet("pack compress", flag.ContinueOnError)
	levelFlag := fs.String("level", "", "Nivel: none|structural|max (default: el del proyecto)")
	asJSON := fs.Bool("json", false, "Emitir el resultado como JSON")
	compare := fs.Bool("compare", false, "Comparar los tres niveles sin imprimir el contenido")
	if err := fs.Parse(args); err != nil {
		fail("%v", err)
	}
	level, err := levelArg(deps, *levelFlag)
	if err != nil {
		fail("%v", err)
	}
	in, err := openPackInput(fs.Args())
	if err != nil {
		fail("abrir input: %v", err)
	}
	defer func() { _ = in.Close() }()
	raw, err := io.ReadAll(in)
	if err != nil {
		fail("leer input: %v", err)
	}

	if *compare {
		fmt.Print(FormatCompare(deps.Compressor, string(raw)))
		return
	}

	result, err := CompressTextAt(deps.Compressor, level, string(raw))
	if err != nil {
		fail("comprimir: %v", err)
	}
	if deps.UsageRecorder != nil {
		deps.UsageRecorder.Record(domain.OpCompressPack, result.RawTokens, result.Tokens)
	}

	if *asJSON {
		refs := result.Refs
		if refs == nil {
			refs = []string{}
		}
		out, _ := json.MarshalIndent(compressJSONOutput{
			Content: result.Content, RawTokens: result.RawTokens, StructuralTokens: result.StructuralTokens,
			Tokens: result.Tokens, Compressor: result.Compressor, ContentType: result.ContentType,
			Refs: refs, FallbackReason: result.FallbackReason,
		}, "", "  ")
		fmt.Println(string(out))
		return
	}

	// stdout lleva solo el contenido, byte a byte (se puede redirigir a un
	// archivo). La línea de tokens va a stderr y, si el contenido no termina en
	// salto de línea, precedida de uno: antes salía pegada a la última línea.
	_, _ = os.Stdout.WriteString(result.Content)
	sep := ""
	if !strings.HasSuffix(result.Content, "\n") {
		sep = "\n"
	}
	fmt.Fprintf(os.Stderr, "%s%s\n", sep, FormatTokensLine(result))
}

// cmdPackRetrieve imprime el original exacto de una ref ⟦mem⟧. Código de
// salida 2 si no existe o caducó (contracts/cli-and-mcp.md).
func cmdPackRetrieve(deps *Deps, args []string) {
	if len(args) != 1 {
		fail("uso: mem pack retrieve <ref>")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*domain.StoreTimeout)
	defer cancel()
	content, meta, err := usecases.RetrieveOriginal(ctx, deps.OriginalStore, args[0])
	if errors.Is(err, usecases.ErrOriginalNotFound) {
		fmt.Fprintf(os.Stderr, "ref %s no encontrada o caducada; vuelve a pedir el contenido a su fuente\n", strings.TrimSpace(args[0]))
		os.Exit(2)
	}
	if err != nil {
		fail("%v", err)
	}
	_, _ = os.Stdout.WriteString(content)
	// Cada recuperación alimenta el ajuste adaptativo (FR-027). Best-effort.
	_, _ = usecases.RecordRetrievalAndTune(ctx, deps.CompressionStats, deps.CompressionTuning, deps.Project, meta, deps.CompressionAdaptiveThreshold)
}

func savingsReport(deps *Deps, ctx context.Context) (usecases.SavingsReport, error) {
	var maxBytes int64
	ttl := 0
	if deps.SettingsRepo != nil {
		st := deps.SettingsRepo.Read(deps.Root)
		maxBytes = int64(st.CompressionOriginalsMaxMB) << 20
		ttl = st.CompressionOriginalsTTLDays
	}
	return usecases.BuildSavingsReport(ctx, deps.CompressionStats, deps.CompressionTuning, deps.OriginalStore, deps.Project, maxBytes, ttl)
}

// cmdPackSavings implementa `mem pack savings [--json]` (FR-026).
func cmdPackSavings(deps *Deps, args []string) {
	fs := flag.NewFlagSet("pack savings", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "Emitir el informe como JSON")
	if err := fs.Parse(args); err != nil {
		fail("%v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*domain.StoreTimeout)
	defer cancel()
	rep, err := savingsReport(deps, ctx)
	if err != nil {
		fail("%v", err)
	}
	if *asJSON {
		out, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(out))
		return
	}
	fmt.Print(rep.Format())
}

// cmdPackTune implementa `mem pack tune --reset [--type T]` (FR-027).
func cmdPackTune(deps *Deps, args []string) {
	fs := flag.NewFlagSet("pack tune", flag.ContinueOnError)
	reset := fs.Bool("reset", false, "Restablecer la agresividad máxima")
	typ := fs.String("type", "", "Tipo de contenido (json, code, log, diff, table, prose); vacío = todos")
	if err := fs.Parse(args); err != nil {
		fail("%v", err)
	}
	if !*reset {
		fail("uso: mem pack tune --reset [--type <tipo>]")
	}
	if deps.CompressionTuning == nil {
		fail("ajuste adaptativo no disponible")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*domain.StoreTimeout)
	defer cancel()
	if err := deps.CompressionTuning.Reset(ctx, deps.Project, strings.TrimSpace(*typ)); err != nil {
		fail("%v", err)
	}
	if *typ == "" {
		fmt.Println("✅ Agresividad máxima restablecida para todos los tipos")
	} else {
		fmt.Printf("✅ Agresividad máxima restablecida para %s\n", *typ)
	}
}

// cmdPackPurge implementa `mem pack purge`: caducados + tope de espacio.
func cmdPackPurge(deps *Deps, _ []string) {
	if deps.OriginalStore == nil {
		fail("almacén de originales no disponible")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*domain.StoreTimeout)
	defer cancel()
	n, freed, err := deps.OriginalStore.Purge(ctx)
	if err != nil {
		fail("%v", err)
	}
	fmt.Printf("purgados %d (%.1f MB)\n", n, float64(freed)/(1<<20))
}
