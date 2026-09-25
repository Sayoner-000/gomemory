package cli

import (
	"flag"
	"fmt"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

func CmdSettings(deps *Deps, args []string) {
	fs := flag.NewFlagSet("settings", flag.ContinueOnError)
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
		fmt.Println("\nUsa --auto-approve=true|false o --code-graph=true|false para cambiar")
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
		}
	})
	if flagErr != nil {
		fail("%v", flagErr)
	}

	if err := deps.SettingsRepo.Write(root, settings); err != nil {
		fail("guardar settings: %v", err)
	}
	if autoApproveChanged {
		deps.SettingsRepo.ApplyAutoApprove(root, settings)
	}
	fmt.Println("✅ Settings actualizados")
	printSettings(deps, root)
}

func printSettings(deps *Deps, root string) {
	s := deps.SettingsRepo.Read(root)
	fmt.Printf("Auto-approve: %v\n", s.AutoApprove)
	if s.AutoApprove {
		fmt.Printf("Tools: %v\n", s.AutoApproveTools)
	}
	fmt.Printf("Grafo de código externo: %v\n", !s.CodeGraphDisabled)
	if len(s.CodeGraphProviders) > 0 {
		fmt.Printf("Proveedores candidatos (en orden): %s\n", strings.Join(s.CodeGraphProviders, ", "))
	} else if s.CodeGraphCommand != "" {
		fmt.Printf("Binario del proveedor: %s\n", s.CodeGraphCommand)
	}
	fmt.Printf("Anotación de impacto al guardar: %v\n", !s.CodeImpactAnnotationDisabled)
	fmt.Printf("Sincronización de ADR: %v\n", s.AdrSyncEnabled)
	fmt.Printf("Brazo extensor spec-kit: %v\n", !s.SpeckitContextDisabled)
	fmt.Printf("Planificación atómica en modo plan: %v\n", !s.AtomicPlanDisabled)
	fmt.Print(formatCompressionSettings(s))
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
