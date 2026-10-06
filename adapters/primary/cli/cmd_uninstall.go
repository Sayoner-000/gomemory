package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dustin/go-humanize"

	"mem/adapters/primary/console"
	"mem/adapters/primary/setup"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// uninstallAgentFiles son los mismos archivos que CmdInstall puede haber
// modificado con el bloque de protocolo de memoria.
var uninstallAgentFiles = []string{"AGENTS.md", "CLAUDE.md", "CLAUDE.txt", ".cursorrules", ".windsurfrules"}

// uninstallMCPConfig es un archivo de configuración MCP por proyecto junto con
// la clave bajo la que ese agente registra sus servidores.
type uninstallMCPConfig struct {
	path string
	key  string
}

// uninstallMCPConfigs se DERIVA de la matriz de canales (feature 022), que es
// la declaración única de qué artefacto corresponde a cada agente, canal y
// ámbito.
//
// Antes era una lista escrita a mano, y ahí estaba el defecto: la instalación
// escribía la configuración de un agente con un esquema propio y esta lista
// solo conocía el de los demás, así que esa entrada sobrevivía a toda
// desinstalación. Derivarla elimina la posibilidad de que vuelvan a separarse.
//
// ~/.codex/config.toml queda fuera por ámbito, no por olvido: es de la persona
// y lo comparten todos sus proyectos. La matriz lo refleja y se informa en vez
// de tocarlo.
var uninstallMCPConfigs = buildUninstallMCPConfigs()

func buildUninstallMCPConfigs() []uninstallMCPConfig {
	vistos := map[string]bool{}
	var out []uninstallMCPConfig
	for _, c := range domain.CellsForActivity(domain.ActivityUninstall) {
		if c.Kind != domain.KindServerConfig || c.ConfigKey == "" {
			continue
		}
		ruta := filepath.Join(c.Path...)
		clave := ruta + "|" + c.ConfigKey
		if vistos[clave] {
			continue
		}
		vistos[clave] = true
		out = append(out, uninstallMCPConfig{path: ruta, key: c.ConfigKey})
	}
	// Los agentes sin canales propios más allá del registro del servidor no
	// tienen fila en la matriz todavía; se conservan aquí hasta que la tengan.
	out = append(out, []uninstallMCPConfig{
		{filepath.Join(".cursor", "mcp.json"), "mcpServers"},
		{filepath.Join(".windsurf", "mcp_config.json"), "mcpServers"},
		{filepath.Join(".cline", "mcp_settings.json"), "mcpServers"},
	}...)
	return out
}

// Códigos de salida de `mem uninstall` (contracts/cli.md).
const (
	uninstallExitOK       = 0
	uninstallExitRefused  = 1
	uninstallExitUsage    = 2
	uninstallExitWarnings = 3
)

// CmdUninstall desinstala gomemory de un proyecto o de todo el sistema
// (feature 034, US2) y devuelve el código de salida. Acepta los flags en
// cualquier posición: flag.FlagSet deja de parsear en el primer argumento
// posicional, y eso no sirve para `mem uninstall [dir] [--yes]`.
func CmdUninstall(deps *Deps, args []string) int {
	o, err := parseUninstallArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗", err)
		return uninstallExitUsage
	}
	if o.target, err = filepath.Abs(o.target); err != nil {
		fmt.Fprintln(os.Stderr, "✗ ruta inválida:", err)
		return uninstallExitUsage
	}
	if st, err := os.Stat(o.target); err != nil || !st.IsDir() {
		fmt.Fprintf(os.Stderr, "✗ %s no es un directorio\n", o.target)
		return uninstallExitUsage
	}

	console.PrintBrand("uninstall")
	mode := console.DetectMode(console.DetectEnv(), o.yes)
	ui := console.New(mode)
	if ui != nil && !o.dryRun && !o.scopeSet {
		scope, err := ui.Select("¿Qué quieres desinstalar?", []console.Option{
			{Value: string(domain.UninstallProject), Label: "Solo este proyecto", Hint: o.target, Recommended: true},
			{Value: string(domain.UninstallSystem), Label: "Todo gomemory del sistema", Hint: "sin dejar rastros"},
		})
		if err != nil {
			fmt.Println("Desinstalación cancelada. No se eliminó nada.")
			return uninstallExitOK
		}
		o.scope = domain.UninstallScope(scope)
	}

	home, _ := os.UserHomeDir()
	data, err := persistence.DataHome()
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ no se pudo resolver el almacén global:", err)
		return uninstallExitRefused
	}
	if o.scanRoot == "" {
		o.scanRoot = home
	}
	self, _ := os.Executable()
	pl := &uninstallPlanner{o: o, home: home, data: data, self: self}

	if ui != nil && !o.dryRun && !o.memorySet {
		choice, err := ui.Select("¿Qué hacemos con la memoria?", []console.Option{
			{Value: string(domain.MemoryExport), Label: "Exportarla antes de borrar", Hint: "se puede reimportar con mem import", Recommended: true},
			{Value: string(domain.MemoryDelete), Label: "Borrarla sin exportar"},
			{Value: string(domain.MemoryKeep), Label: "Conservarla"},
		})
		if err != nil {
			fmt.Println("Desinstalación cancelada. No se eliminó nada.")
			return uninstallExitOK
		}
		pl.o.memory = domain.MemoryChoice(choice)
	}
	if pl.o.memory == domain.MemoryExport && pl.o.exportDir == "" {
		pl.o.exportDir = exportDirDefault(home)
	}
	if pl.o.exportDir != "" {
		if pl.o.exportDir, err = filepath.Abs(pl.o.exportDir); err != nil {
			fmt.Fprintln(os.Stderr, "✗ ruta de exportación inválida:", err)
			return uninstallExitUsage
		}
	}

	if err := pl.plan(); err != nil {
		fmt.Fprintln(os.Stderr, "✗ no se pudo preparar el inventario:", err)
		return uninstallExitRefused
	}
	printUninstallInventory(pl)

	if o.dryRun {
		fmt.Println("\n(simulación: no se modificó nada)")
		return uninstallExitOK
	}
	if len(pl.steps) == 0 {
		fmt.Println("\nNo hay nada de gomemory que desinstalar.")
		return uninstallExitOK
	}
	if ui == nil && !o.yes {
		fmt.Println("\n✗ se requiere --yes para desinstalar sin terminal interactiva. No se eliminó nada.")
		return uninstallExitRefused
	}
	if ui != nil {
		ok, err := confirmUninstall(ui, pl)
		if err != nil || !ok {
			fmt.Println("Desinstalación cancelada. No se eliminó nada.")
			return uninstallExitOK
		}
	}
	return runUninstall(pl, mode == console.ModeRich)
}

// confirmUninstall pide la confirmación. El texto enumera exactamente las
// categorías que se borrarán (FR-019); el alcance de sistema exige escribir
// la palabra (FR-016).
func confirmUninstall(ui console.UI, pl *uninstallPlanner) (bool, error) {
	msg := "Se retirará: " + strings.Join(uninstallCategories(pl), ", ") + "."
	if pl.o.scope == domain.UninstallSystem {
		return ui.TypedConfirm(msg+" Esta acción no se puede deshacer.", "gomemory")
	}
	return ui.Confirm(msg+" ¿Continuar?", false)
}

func uninstallCategories(pl *uninstallPlanner) []string {
	names := map[domain.ItemCategory]string{
		domain.CategoryProjectFiles: "la integración de los proyectos",
		domain.CategoryAgentConfig:  "la configuración de gomemory en tus agentes",
		domain.CategoryMemory:       "la memoria",
		domain.CategoryBinary:       "el binario",
	}
	seen := map[domain.ItemCategory]bool{}
	var out []string
	for _, s := range pl.steps {
		if c := s.item.Category; !seen[c] {
			seen[c] = true
			out = append(out, names[c])
		}
	}
	return out
}

func printUninstallInventory(pl *uninstallPlanner) {
	scope := "este proyecto"
	if pl.o.scope == domain.UninstallSystem {
		scope = "todo el sistema"
	}
	fmt.Printf("🗑️  Desinstalar gomemory de %s\n", scope)
	if pl.o.scope == domain.UninstallSystem && !pl.o.noScan {
		fmt.Printf("  ℹ️  Proyectos buscados en %s (hasta %d niveles). Para buscar en otra ruta: --scan <dir>\n",
			pl.o.scanRoot, domain.UninstallScanMaxDepth)
	}
	headers := []struct {
		c     domain.ItemCategory
		title string
	}{
		{domain.CategoryProjectFiles, "Proyectos"},
		{domain.CategoryAgentConfig, "Configuración de agentes"},
		{domain.CategoryMemory, "Memoria"},
		{domain.CategoryBinary, "Binario"},
	}
	for _, h := range headers {
		var lines []string
		for _, s := range pl.steps {
			if s.item.Category != h.c {
				continue
			}
			label := s.item.Label
			if label == "" {
				label = s.item.Path
			}
			if s.item.Bytes > 0 {
				label += " — " + humanize.Bytes(uint64(s.item.Bytes))
			}
			lines = append(lines, "    • "+label)
		}
		if len(lines) > 0 {
			fmt.Printf("  %s:\n%s\n", h.title, strings.Join(lines, "\n"))
		}
	}
	switch pl.o.memory {
	case domain.MemoryExport:
		fmt.Printf("  La memoria se exportará antes a %s\n", pl.o.exportDir)
	case domain.MemoryKeep:
		if dir, err := persistence.DataHome(); err == nil {
			fmt.Printf("  La memoria se conserva en %s\n", dir)
		}
	}
	for _, s := range pl.skipped {
		fmt.Printf("  ⚠️  No se pudo leer %s durante la búsqueda de proyectos\n", s)
	}
}

// runUninstall exporta (si procede) y ejecuta el plan en el orden de FR-014.
func runUninstall(pl *uninstallPlanner, styled bool) int {
	plan := domain.UninstallPlan{Scope: pl.o.scope, Memory: pl.o.memory, ExportDir: pl.o.exportDir}
	for _, s := range pl.steps {
		plan.Items = append(plan.Items, s.item)
	}
	apply := map[string]func() error{}
	for _, s := range pl.steps {
		apply[string(s.item.Category)+"|"+s.item.Path] = s.apply
	}
	plan.Sort()

	fmt.Println()
	if pl.o.memory == domain.MemoryExport {
		index, failed := usecases.ExportProjects(pl.o.exportDir, pl.exportTargets(), openStoreByKey)
		if len(failed) == 0 {
			fmt.Printf("  ✓ Memoria exportada a %s (%d proyecto(s))\n", pl.o.exportDir, len(index))
		}
		// FR-009: la memoria cuya exportación falló no se borra.
		for i := range plan.Items {
			it := &plan.Items[i]
			if it.Category != domain.CategoryMemory {
				continue
			}
			for key, err := range failed {
				if it.Key == "" || it.Key == key {
					it.Result = domain.ResultWarn
					it.Detail = "no se borró: la exportación falló (" + err.Error() + ")"
					it.Manual = "revisa el destino y vuelve a ejecutar mem uninstall"
				}
			}
		}
	}

	rep := console.NewReporter(os.Stdout, styled)
	plan.Execute(func(it *domain.UninstallItem) error {
		err := apply[string(it.Category)+"|"+it.Path]()
		if err != nil {
			it.Manual = manualRemoval(it)
		}
		return err
	})
	fmt.Println("\nResumen:")
	for _, it := range plan.Items {
		label := it.Label
		if label == "" {
			label = it.Path
		}
		st := console.StepOK
		if it.Result == domain.ResultWarn {
			st = console.StepWarn
		}
		rep.Done(console.StepResult{Name: label, Status: st, Detail: it.Detail, Manual: it.Manual})
	}
	if plan.Warnings() > 0 {
		return uninstallExitWarnings
	}
	return uninstallExitOK
}

func manualRemoval(it *domain.UninstallItem) string {
	switch it.Kind {
	case domain.KindEntry:
		return "quita a mano las entradas de gomemory de " + it.Path
	case domain.KindDir:
		return "borra a mano " + it.Path
	default:
		return "borra a mano " + it.Path
	}
}

func removeIntegrationBlocks(target string) {
	for _, fname := range uninstallAgentFiles {
		fpath := filepath.Join(target, fname)
		data, err := os.ReadFile(fpath)
		if err != nil {
			fmt.Printf("  ℹ️  %s: no encontrado\n", fname)
			continue
		}
		content := string(data)

		loc := versionMarkerPattern.FindStringIndex(content)
		if loc == nil {
			fmt.Printf("  ℹ️  %s: sin bloque de protocolo de memoria reconocible, se conserva tal cual\n", fname)
			continue
		}
		idx := loc[0]

		trimmed := strings.TrimRight(content[:idx], "\n")
		if trimmed == "" || isAutoGeneratedTitle(trimmed) {
			if err := os.Remove(fpath); err != nil {
				fmt.Printf("  ⚠️  %s: error al eliminar archivo vacío tras quitar el bloque: %v\n", fname, err)
				continue
			}
			fmt.Printf("  ✅ %s: eliminado (era generado enteramente por mem install)\n", fname)
			continue
		}

		if err := os.WriteFile(fpath, []byte(trimmed+"\n"), 0644); err != nil {
			fmt.Printf("  ⚠️  %s: error al quitar el bloque: %v\n", fname, err)
			continue
		}
		fmt.Printf("  ✅ %s: bloque de protocolo de memoria removido\n", fname)
	}
}

// isAutoGeneratedTitle detecta si lo que queda tras quitar el bloque de
// protocolo es exactamente el título que las versiones anteriores de
// `mem install` generaban para archivos creados desde cero (sin contenido
// previo del usuario) — en ese caso el archivo entero era generado y debe
// eliminarse, no solo el bloque.
//
// La feature 021 retiró esa generación, pero los títulos siguen listados aquí:
// `mem uninstall` debe poder limpiar proyectos instalados con versiones
// anteriores, que son justamente los que tienen esos archivos.
func isAutoGeneratedTitle(trimmed string) bool {
	switch trimmed {
	case "# Instrucciones para agentes AI", "# Instrucciones para Claude Code":
		return true
	default:
		return false
	}
}

func removeMCPEntries(target string) {
	for _, conf := range uninstallMCPConfigs {
		rel := conf.path
		fpath := filepath.Join(target, rel)
		data, err := os.ReadFile(fpath)
		if err != nil {
			fmt.Printf("  ℹ️  %s: no encontrado\n", rel)
			continue
		}

		var cfg map[string]interface{}
		if err := json.Unmarshal(data, &cfg); err != nil {
			fmt.Printf("  ⚠️  %s: JSON inválido, se conserva sin tocar\n", rel)
			continue
		}

		ms, ok := cfg[conf.key].(map[string]interface{})
		if !ok {
			fmt.Printf("  ℹ️  %s: sin entrada gomemory\n", rel)
			continue
		}
		if _, has := ms["gomemory"]; !has {
			fmt.Printf("  ℹ️  %s: sin entrada gomemory\n", rel)
			continue
		}

		delete(ms, "gomemory")
		cfg[conf.key] = ms

		out, _ := json.MarshalIndent(cfg, "", "  ")
		if err := os.WriteFile(fpath, out, 0644); err != nil {
			fmt.Printf("  ⚠️  %s: error al escribir: %v\n", rel, err)
			continue
		}
		fmt.Printf("  ✅ %s: entrada gomemory removida\n", rel)
	}
}

func removeClaudePlugin(target string) {
	pluginDir := filepath.Join(target, ".claude", "plugins", "gomemory")
	if _, err := os.Stat(pluginDir); err == nil {
		if err := os.RemoveAll(pluginDir); err != nil {
			fmt.Printf("  ⚠️  %s: error al eliminar: %v\n", pluginDir, err)
		} else {
			fmt.Printf("  ✅ %s: eliminado\n", pluginDir)
		}
	} else {
		fmt.Println("  ℹ️  .claude/plugins/gomemory: no encontrado")
	}

	settingsPath := filepath.Join(target, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		fmt.Println("  ℹ️  .claude/settings.json: no encontrado")
		return
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		fmt.Println("  ⚠️  .claude/settings.json: JSON inválido, se conserva sin tocar")
		return
	}

	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		fmt.Println("  ℹ️  .claude/settings.json: sin hooks de gomemory")
		return
	}

	changed := false
	for _, key := range []string{"SessionStart", "PreCompact", "UserPromptSubmit", "SessionEnd", "Stop", "SubagentStart", "SubagentStop", "PostToolUse"} {
		entries, ok := hooks[key].([]interface{})
		if !ok {
			continue
		}
		kept := make([]interface{}, 0, len(entries))
		for _, e := range entries {
			// Reconoce tanto el formato objeto nuevo (command "mem hook ...")
			// como el legado ([]string a scripts/*.sh del plugin).
			if setup.IsGomemoryHookEntry(e) {
				changed = true
				continue
			}
			kept = append(kept, e)
		}
		if len(kept) == 0 {
			delete(hooks, key)
		} else {
			hooks[key] = kept
		}
	}

	if !changed {
		fmt.Println("  ℹ️  .claude/settings.json: sin hooks de gomemory")
		return
	}

	settings["hooks"] = hooks
	out, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, out, 0644); err != nil {
		fmt.Printf("  ⚠️  .claude/settings.json: error al escribir: %v\n", err)
		return
	}
	fmt.Println("  ✅ .claude/settings.json: hooks de gomemory removidos")
}

func removeClaudePermissions(target string) {
	changed, err := setup.RemoveClaudePermissions(target)
	if err != nil {
		fmt.Printf("  ⚠️  .claude/settings.json: error al limpiar permisos: %v\n", err)
		return
	}
	if !changed {
		fmt.Println("  ℹ️  .claude/settings.json: sin permisos de gomemory")
		return
	}
	fmt.Println("  ✅ .claude/settings.json: permisos de gomemory removidos")
}

// removeOpenCodeArtifacts retira lo que InstallOpenCode escribe y que la entrada
// MCP no cubre: los permisos pre-aprobados del opencode.json y el plugin de
// usuario.
//
// Existe por simetría con removeClaudePlugin y removeClaudePermissions: la
// desinstalación retiraba por completo los artefactos de un agente y dejaba
// intactos los del otro. El registro de capacidades no detecta esta clase de
// asimetría, porque describe la activación y no la retirada.
func removeOpenCodeArtifacts(target string) {
	changed, err := setup.RemoveOpenCodePermissions(target)
	switch {
	case err != nil:
		fmt.Printf("  ⚠️  opencode.json: error al limpiar permisos: %v\n", err)
	case changed:
		fmt.Println("  ✅ opencode.json: permisos de gomemory removidos")
	default:
		fmt.Println("  ℹ️  opencode.json: sin permisos de gomemory")
	}

	// El plugin de OpenCode NO se elimina: vive en el HOME y lo comparten todos
	// los proyectos. Se informa su ruta, misma política que ~/.codex/config.toml.
	if ruta := setup.OpenCodePluginPath(); ruta != "" {
		fmt.Printf("  ℹ️  %s no se elimina — es de ámbito de usuario y lo comparten todos tus proyectos; bórralo a mano si ya no usas gomemory en ninguno.\n", ruta)
	}
}

// removeNativeWrappers retira los envoltorios nativos que la instalación
// genera para cada agente.
//
// Retira archivos, nunca directorios: `.claude/skills` y `.opencode/commands`
// alojan también habilidades de otras herramientas y comandos de la persona.
// El directorio contenedor se elimina solo si queda vacío, que es la señal de
// que era enteramente nuestro.
func removeNativeWrappers(target string) {
	retirados := 0
	for _, seg := range setup.GeneratedWrapperPaths() {
		ruta := filepath.Join(append([]string{target}, seg...)...)
		if _, err := os.Stat(ruta); err != nil {
			continue
		}
		if err := os.Remove(ruta); err != nil {
			fmt.Printf("  ⚠️  %s: error al eliminar: %v\n", filepath.Join(seg...), err)
			continue
		}
		retirados++
		// Si el envoltorio vivía en su propio subdirectorio y ese subdirectorio
		// queda vacío, era nuestro por completo.
		if dir := filepath.Dir(ruta); dir != target {
			if entradas, err := os.ReadDir(dir); err == nil && len(entradas) == 0 {
				_ = os.Remove(dir)
			}
		}
	}
	if retirados > 0 {
		fmt.Printf("  ✅ %d envoltorio(s) nativo(s) retirado(s)\n", retirados)
	} else {
		fmt.Println("  ℹ️  envoltorios nativos: no encontrados")
	}
}
