package cli

import (
	"flag"
	"fmt"
	"strings"
)

func CmdSettings(deps *Deps, args []string) {
	fs := flag.NewFlagSet("settings", flag.ContinueOnError)
	autoApprove := fs.Bool("auto-approve", false, "Activar auto-approve en MCP")
	codeGraph := fs.Bool("code-graph", true, "Activar el grafo de código externo (codebase-memory-mcp)")
	codeGraphCmd := fs.String("code-graph-command", "", "Binario del proveedor de grafo externo (opcional, legado — ver --code-graph-providers)")
	codeGraphProviders := fs.String("code-graph-providers", "", "Lista de proveedores candidatos separados por coma, en orden de prioridad (usa el primero disponible)")
	codeImpactAnnotation := fs.Bool("code-impact-annotation", true, "Anotar impacto de código al guardar una memoria con archivo asociado")
	adrSync := fs.Bool("adr-sync", false, "Sincronizar memorias architecture/decision como ADR con el proveedor externo")
	show := fs.Bool("show", false, "Mostrar configuración actual")
	fs.Parse(args)

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
		}
	})

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
