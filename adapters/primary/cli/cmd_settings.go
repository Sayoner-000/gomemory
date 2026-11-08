package cli

import (
	"flag"
	"fmt"
	"strings"

	"mem/adapters/primary/console"
	"mem/application/ports"
	"mem/domain"
)

func CmdSettings(deps *Deps, args []string) {
	fs := newFlagSet("settings", flag.ContinueOnError)
	autoApprove := fs.Bool("auto-approve", false, "Activar auto-approve en MCP")
	codeGraph := fs.Bool("code-graph", true, "Activar el grafo de código externo (codebase-memory-mcp)")
	codeGraphCmd := fs.String("code-graph-command", "", "Binario del proveedor de grafo externo (opcional, legado — ver --code-graph-providers)")
	codeGraphProviders := fs.String("code-graph-providers", "", "Lista de proveedores candidatos separados por coma, en orden de prioridad (usa el primero disponible)")
	codeImpactAnnotation := fs.Bool("code-impact-annotation", true, "Anotar impacto de código al guardar una memoria con archivo asociado")
	adrSync := fs.Bool("adr-sync", false, "Sincronizar memorias architecture/decision como ADR con el proveedor externo")
	speckitContext := fs.Bool("speckit-context", true, "Activar el brazo extensor hacia spec-kit (resumen de historial en /speckit-specify)")
	atomicPlan := fs.Bool("atomic-plan", true, "Activar la planificación atómica en modo plan (método + contexto vía get_plan_context)")
	compressionLevel := fs.String("compression-level", "", "Nivel de compresión del contexto: none|structural|max (feature 033)")
	toolOutput := fs.Bool("tool-output-compression", false, "Comprimir la salida de las herramientas del agente (opt-in; requiere `mem install` para registrar el hook)")
	concise := fs.Bool("concise-output", false, "Añadir la directiva de respuestas concisas al contexto (opt-in)")
	lang := fs.String("lang", "", "Idioma del proyecto (ej: es, en)")
	language := fs.String("language", "", "Alias de --lang")
	show := fs.Bool("show", false, "Mostrar configuración actual")
	if err := fs.Parse(args); err != nil {
		return
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("no hay proyecto con memoria: %v", err)
	}

	if *show {
		printSettings(deps, root)
		return
	}

	if fs.NFlag() == 0 {
		printSettings(deps, root)
		humanln("\nUsa --auto-approve=true|false o --code-graph=true|false para cambiar")
		return
	}

	// Solo se tocan los flags realmente pasados (fs.Visit), para no pisar el
	// resto de la configuración con sus valores por defecto.
	settings := deps.SettingsRepo.Read(root)
	autoApproveChanged := false
	var flagErr error
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "auto-approve":
			settings.AutoApprove = *autoApprove
			autoApproveChanged = true
		case "code-graph":
			settings.CodeGraphDisabled = !*codeGraph
		case "code-graph-command":
			settings.CodeGraphCommand = *codeGraphCmd
		case "code-graph-providers":
			settings.CodeGraphProviders = splitProviderList(*codeGraphProviders)
		case "code-impact-annotation":
			settings.CodeImpactAnnotationDisabled = !*codeImpactAnnotation
		case "adr-sync":
			settings.AdrSyncEnabled = *adrSync
		case "speckit-context":
			settings.SpeckitContextDisabled = !*speckitContext
		case "atomic-plan":
			settings.AtomicPlanDisabled = !*atomicPlan
		case "compression-level":
			flagErr = applyCompressionLevel(&settings, *compressionLevel)
		case "tool-output-compression":
			settings.ToolOutputCompression = *toolOutput
		case "concise-output":
			settings.ConciseOutputDirective = *concise
		case "lang", "language":
			target := *lang
			if target == "" {
				target = *language
			}
			target = strings.ToLower(strings.TrimSpace(target))
			cat := getCatalog()
			if !cat.Supports(target) {
				flagErr = fmt.Errorf("idioma no soportado %q: disponibles: %s", target, strings.Join(cat.SupportedCodes(), ", "))
			} else {
				settings.Language = target
			}
		}
	})
	if flagErr != nil {
		fail("%v", flagErr)
	}

	if err := deps.SettingsRepo.Write(root, settings); err != nil {
		fail("guardar settings: %v", err)
	}
	if autoApproveChanged {
		if err := deps.SettingsRepo.ApplyAutoApprove(root, settings); err != nil {
			fail("aplicar auto-approve: %v", err)
		}
	}
	cat := getCatalog()
	humanln(cat.Get(settings.Language, "settings_updated"))
	printSettings(deps, root)
}

func printSettings(deps *Deps, root string) {
	console.BeginPage()
	defer console.EndPage()
	s := deps.SettingsRepo.Read(root)
	lang := s.Language
	if lang == "" {
		lang = "es"
	}
	cat := getCatalog()
	humanf("%s\n\n%s: %v\n", cat.Get(lang, "settings_title"), cat.Get(lang, "settings_auto_approve"), s.AutoApprove)
	if s.AutoApprove {
		humanf("Tools: %v\n", s.AutoApproveTools)
	}
	humanf("%s: %v\n", cat.Get(lang, "settings_code_graph"), !s.CodeGraphDisabled)
	if len(s.CodeGraphProviders) > 0 {
		humanf("Proveedores candidatos (en orden): %s\n", strings.Join(s.CodeGraphProviders, ", "))
	} else if s.CodeGraphCommand != "" {
		humanf("Binario del proveedor: %s\n", s.CodeGraphCommand)
	}
	humanf("%s: %v\n", cat.Get(lang, "settings_impact_annotation"), !s.CodeImpactAnnotationDisabled)
	humanf("%s: %v\n", cat.Get(lang, "settings_adr_sync"), s.AdrSyncEnabled)
	humanf("%s: %v\n", cat.Get(lang, "settings_speckit"), !s.SpeckitContextDisabled)
	humanf("%s: %v\n", cat.Get(lang, "settings_atomic_plan"), !s.AtomicPlanDisabled)
	humanf("%s: %s\n", cat.Get(lang, "settings_language"), s.Language)
	humanf("%s", formatCompressionSettings(s))
}

// applyCompressionLevel valida y fija el nivel de compresión. Rechaza un valor
// fuera de none|structural|max en vez de guardarlo: un nivel inválido se leería
// después como "structural" sin que la persona se entere (FR-030).
func applyCompressionLevel(s *ports.SettingsData, raw string) error {
	level := strings.ToLower(strings.TrimSpace(raw))
	if !domain.ValidCompressionLevel(level) {
		return fmt.Errorf("--compression-level=%q no válido: usa none, structural o max", raw)
	}
	s.ContextCompressionLevel = level
	return nil
}

// formatCompressionSettings muestra el nivel efectivo (no el crudo) y de dónde
// sale, más los dos interruptores opt-in de la feature 033.
func formatCompressionSettings(s ports.SettingsData) string {
	efectivo := domain.ParseCompressionLevel(s.ContextCompressionLevel, s.ContextCompressionDisabled)
	origen := "ajuste"
	switch {
	case s.ContextCompressionLevel == "" && s.ContextCompressionDisabled:
		origen = "heredado: context_compression_disabled"
	case s.ContextCompressionLevel == "":
		origen = "por defecto"
	}
	return fmt.Sprintf("Compresión de contexto: %s (%s)\nComprimir salidas de herramientas: %v\nDirectiva de respuestas concisas: %v\n",
		efectivo, origen, s.ToolOutputCompression, s.ConciseOutputDirective)
}

// splitProviderList separa "--code-graph-providers=cmd1,cmd2" en una lista,
// recortando espacios y descartando elementos vacíos (p.ej. una coma de más).
// Un valor vacío ("") limpia la lista, volviendo al legado/autodetección.
func splitProviderList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
