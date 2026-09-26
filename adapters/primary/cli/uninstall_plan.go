package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"mem/adapters/primary/setup"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// Planificación y ejecución de `mem uninstall` (feature 034, US2). El orden,
// la continuidad ante un fallo y los estados viven en domain.UninstallPlan;
// aquí solo se reúnen los hechos del disco y se aplica cada paso.

// uninstallOptions son las decisiones de la persona, por flags o por consola.
type uninstallOptions struct {
	scope     domain.UninstallScope
	target    string
	yes       bool
	dryRun    bool
	noScan    bool
	scanRoot  string
	memory    domain.MemoryChoice
	exportDir string
	// memorySet indica que la memoria la fijó un flag (--keep-memory/--export).
	memorySet bool
	scopeSet  bool
}

// errUsage es un uso inválido de flags: código de salida 2.
var errUsage = errors.New("uso inválido")

func parseUninstallArgs(args []string) (uninstallOptions, error) {
	o := uninstallOptions{scope: domain.UninstallProject, target: ".", memory: domain.MemoryDelete}
	keep := false
	next := func(i *int, flag string) (string, error) {
		if *i+1 >= len(args) {
			return "", fmt.Errorf("%w: %s necesita un valor", errUsage, flag)
		}
		*i++
		return args[*i], nil
	}
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--yes" || a == "-y":
			o.yes = true
		case a == "--all":
			o.scope, o.scopeSet = domain.UninstallSystem, true
		case a == "--dry-run":
			o.dryRun = true
		case a == "--no-scan":
			o.noScan = true
		case a == "--keep-memory":
			keep = true
		case a == "--export" || a == "--scan":
			v, err := next(&i, a)
			if err != nil {
				return o, err
			}
			if a == "--export" {
				o.exportDir = v
			} else {
				o.scanRoot = v
			}
		case strings.HasPrefix(a, "--export="):
			o.exportDir = strings.TrimPrefix(a, "--export=")
		case strings.HasPrefix(a, "--scan="):
			o.scanRoot = strings.TrimPrefix(a, "--scan=")
		case strings.HasPrefix(a, "-"):
			return o, fmt.Errorf("%w: flag desconocido %s", errUsage, a)
		default:
			positional = append(positional, a)
		}
	}
	if keep && o.exportDir != "" {
		return o, fmt.Errorf("%w: --keep-memory y --export son incompatibles", errUsage)
	}
	switch {
	case keep:
		o.memory, o.memorySet = domain.MemoryKeep, true
	case o.exportDir != "":
		o.memory, o.memorySet = domain.MemoryExport, true
	}
	if len(positional) > 0 {
		o.target = positional[0]
	}
	return o, nil
}

// uninstallStep une un elemento del inventario con su forma de retirarlo.
type uninstallStep struct {
	item  domain.UninstallItem
	apply func() error
}

type uninstallPlanner struct {
	o     uninstallOptions
	home  string
	data  string
	self  string
	steps []uninstallStep
	// projects son los proyectos incluidos; roots por clave para el índice.
	projects []string
	roots    map[string]string
	skipped  []string
}

func (pl *uninstallPlanner) add(item domain.UninstallItem, apply func() error) {
	item.Result = domain.ResultPending
	pl.steps = append(pl.steps, uninstallStep{item: item, apply: apply})
}

// plan construye el inventario de lo que EXISTE realmente (FR-008).
func (pl *uninstallPlanner) plan() error {
	pl.roots = map[string]string{}
	if pl.o.scope == domain.UninstallProject {
		pl.projects = []string{pl.o.target}
	} else if err := pl.discoverProjects(); err != nil {
		return err
	}
	for _, p := range pl.projects {
		pl.roots[persistence.ProjectKey(p)] = p
		pl.planProject(p)
	}
	if pl.o.scope == domain.UninstallSystem {
		pl.planGlobalAgentConfig()
		pl.planStore()
		pl.planBinaries()
	}
	return nil
}

func (pl *uninstallPlanner) discoverProjects() error {
	seen := map[string]bool{}
	addProject := func(p string) {
		key := persistence.ProjectKey(p)
		if !seen[key] {
			seen[key] = true
			pl.projects = append(pl.projects, p)
		}
	}
	regs, err := persistence.ListRegisteredProjects()
	if err != nil {
		return err
	}
	for _, r := range regs {
		pl.roots[r.Key] = r.Root
		if r.Exists {
			addProject(r.Root)
		}
	}
	if !pl.o.noScan {
		found, skipped, err := persistence.ScanProjects(pl.o.scanRoot, domain.UninstallScanMaxDepth)
		if err != nil {
			return err
		}
		pl.skipped = skipped
		for _, p := range found {
			addProject(p)
		}
	}
	sort.Strings(pl.projects)
	return nil
}

func (pl *uninstallPlanner) planProject(p string) {
	integ := domain.NewUninstallItem(domain.CategoryProjectFiles, p, domain.KindEntry)
	integ.Label = "Integración de gomemory en " + p
	keepsGlobal := pl.o.scope == domain.UninstallProject
	pl.add(integ, func() error { return removeProjectIntegration(p, keepsGlobal) })

	memDir := filepath.Join(p, persistence.MemDir)
	if _, err := os.Stat(memDir); err == nil {
		pl.add(domain.NewUninstallItem(domain.CategoryProjectFiles, memDir, domain.KindDir),
			func() error { return os.RemoveAll(memDir) })
	}
	if c, ok := identifyLocalCopy(p); ok {
		it := domain.NewUninstallItem(domain.CategoryProjectFiles, c.Path, domain.KindFile)
		it.Label = c.Path + " (copia v" + c.Version + ")"
		pl.add(it, func() error { return removeBinaryFile(c.Path, pl.self) })
	}
	// En el alcance de proyecto, su base del almacén (FR-010). En el de
	// sistema la cubre el almacén entero.
	if pl.o.scope == domain.UninstallProject && pl.o.memory != domain.MemoryKeep {
		key := persistence.ProjectKey(p)
		dir, err := persistence.GlobalProjectDir(key)
		if err == nil {
			if _, err := os.Stat(dir); err == nil {
				it := domain.NewUninstallItem(domain.CategoryMemory, dir, domain.KindDir)
				it.Key, it.Bytes = key, dirSize(dir)
				it.Label = "Memoria del proyecto (" + dir + ")"
				pl.add(it, func() error { return os.RemoveAll(dir) })
			}
		}
	}
}

// removeProjectIntegration retira lo que install escribe en un proyecto. Son
// las funciones de siempre de uninstall; cada una informa de su parte.
// keepsGlobal indica que la configuración de usuario se conserva (alcance de
// proyecto); en el de sistema la retira después globalRemover.
func removeProjectIntegration(p string, keepsGlobal bool) error {
	removeIntegrationBlocks(p)
	removeMCPEntries(p)
	if keepsGlobal {
		fmt.Println("  ℹ️  ~/.codex/config.toml conserva [mcp_servers.gomemory] y los hooks del ciclo de gomemory — son configuración global compartida por todos los proyectos.")
	}
	removeClaudePlugin(p)
	removeClaudePermissions(p)
	removeOpenCodeArtifacts(p)
	removeNativeWrappers(p)
	return nil
}

// globalRemover devuelve cómo retirar una celda de usuario de la matriz, o nil
// si la celda no tiene retirada automática.
func globalRemover(home string, c domain.MatrixCell) (label string, present func() bool, apply func() error) {
	rel := filepath.Join(c.Path...)
	path := filepath.Join(home, rel)
	contains := func(needles ...string) func() bool {
		return func() bool {
			data, err := os.ReadFile(path)
			if err != nil {
				return false
			}
			for _, n := range needles {
				if bytes.Contains(data, []byte(n)) {
					return true
				}
			}
			return false
		}
	}
	switch {
	case c.Kind == domain.KindServerConfig && strings.HasSuffix(rel, ".json"):
		return "Servidor MCP de gomemory en " + path, contains(`"gomemory"`), func() error {
			_, err := removeJSONServerEntry(path, c.ConfigKey)
			if err == nil && strings.HasSuffix(rel, filepath.Join("opencode", "opencode.json")) {
				_, err = setup.RemoveOpenCodePermissions(filepath.Dir(path))
			}
			return err
		}
	case strings.HasSuffix(rel, filepath.Join(".codex", "config.toml")):
		return "Servidor MCP y hooks de gomemory en " + path, contains("gomemory", "mem hook"), func() error {
			_, err := removeCodexConfigFile(path)
			if _, bErr := removeCodexBackups(home); err == nil {
				err = bErr
			}
			return err
		}
	case strings.HasSuffix(rel, filepath.Join(".codex", "hooks.json")):
		return "Hooks heredados de gomemory en " + path, contains("mem hook"), func() error {
			_, err := removeLegacyCodexHooksJSON(path)
			return err
		}
	case strings.HasSuffix(rel, filepath.Join(".claude", "settings.json")):
		return "Hooks y permisos de gomemory en " + path, contains("mem hook", "gomemory"), func() error {
			removeClaudePlugin(home)
			_, err := setup.RemoveClaudePermissions(home)
			return err
		}
	case c.Kind == domain.KindInstructions:
		return "Protocolo de gomemory en " + path, contains("gomemory-protocol"), func() error {
			_, err := removeProtocolBlockFile(path)
			return err
		}
	case filepath.Base(rel) == "gomemory.ts":
		return "Plugin de gomemory " + path, func() bool { _, err := os.Stat(path); return err == nil },
			func() error { return os.Remove(path) }
	case c.Kind == domain.KindNativeWrapper:
		// Los envoltorios se retiran en un solo paso (planGlobalAgentConfig).
		return "", nil, nil
	}
	return "", nil, nil
}

func (pl *uninstallPlanner) planGlobalAgentConfig() {
	seen := map[string]bool{}
	for _, c := range domain.CellsForActivity(domain.ActivityUninstallGlobal) {
		label, present, apply := globalRemover(pl.home, c)
		path := filepath.Join(append([]string{pl.home}, c.Path...)...)
		if apply == nil || seen[path] || !present() {
			continue
		}
		seen[path] = true
		it := domain.NewUninstallItem(domain.CategoryAgentConfig, path, domain.KindEntry)
		it.Label = label
		pl.add(it, apply)
	}
	var wrappers []string
	for _, rel := range setup.GlobalGeneratedArtifacts() {
		p := filepath.Join(append([]string{pl.home}, rel...)...)
		if _, err := os.Lstat(p); err == nil {
			wrappers = append(wrappers, p)
		}
	}
	if len(wrappers) > 0 {
		it := domain.NewUninstallItem(domain.CategoryAgentConfig, filepath.Dir(wrappers[0]), domain.KindDir)
		it.Label = fmt.Sprintf("Habilidades y comandos de gomemory (%d)", len(wrappers))
		pl.add(it, func() error {
			_, errs := removeGlobalGeneratedArtifacts(pl.home)
			return errors.Join(errs...)
		})
	}
}

func (pl *uninstallPlanner) planStore() {
	if pl.o.memory == domain.MemoryKeep {
		return
	}
	if _, err := os.Stat(pl.data); err != nil {
		return
	}
	it := domain.NewUninstallItem(domain.CategoryMemory, pl.data, domain.KindDir)
	it.Label = "Almacén global de gomemory (" + pl.data + ")"
	it.Bytes = dirSize(pl.data)
	pl.add(it, func() error { return os.RemoveAll(pl.data) })
}

func (pl *uninstallPlanner) planBinaries() {
	seen := map[string]bool{}
	for _, p := range []string{globalBinaryFrom(pl.o.target), pl.self} {
		if p == "" || seen[p] {
			continue
		}
		if _, ok := identifyVersion(p); !ok {
			continue
		}
		seen[p] = true
		path := p
		pl.add(domain.NewUninstallItem(domain.CategoryBinary, path, domain.KindFile),
			func() error { return removeBinaryFile(path, pl.self) })
	}
}

func globalBinaryFrom(root string) string {
	g, _ := resolveGlobalBinary(root)
	return g
}

// exportTargets son los proyectos cuya memoria se exporta: el del alcance de
// proyecto, o todas las bases del almacén en el de sistema (también las que
// no tienen ruta registrada).
func (pl *uninstallPlanner) exportTargets() []usecases.ExportTarget {
	if pl.o.scope == domain.UninstallProject {
		key := persistence.ProjectKey(pl.o.target)
		return []usecases.ExportTarget{{Key: key, Root: pl.o.target}}
	}
	entries, _ := os.ReadDir(filepath.Join(pl.data, "projects"))
	var out []usecases.ExportTarget
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, usecases.ExportTarget{Key: e.Name(), Root: pl.roots[e.Name()]})
		}
	}
	return out
}

func openStoreByKey(key string) (ports.MemoryRepository, ports.RelationRepository, func(), bool, error) {
	db, err := persistence.OpenByKey(key)
	if err != nil || db == nil {
		return nil, nil, nil, false, err
	}
	return persistence.NewMemoryRepository(db), persistence.NewRelationRepository(db), func() { _ = db.Close() }, true, nil
}

// removeBinaryFile borra un binario de gomemory. El ejecutable en curso en
// Windows está bloqueado: se borra en diferido al terminar este proceso.
func removeBinaryFile(path, self string) error {
	if runtime.GOOS == "windows" && sameFilePath(path, self) {
		// En Windows un proceso hijo sobrevive al padre: `cmd` espera a que
		// este termine y entonces borra el ejecutable liberado.
		cmd := exec.Command("cmd", "/c", "ping -n 3 127.0.0.1 >nul & del /f /q \""+path+"\"")
		return cmd.Start()
	}
	return os.Remove(path)
}

func sameFilePath(a, b string) bool {
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}

func dirSize(root string) int64 {
	var n int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

func exportDirDefault(home string) string {
	return filepath.Join(home, "gomemory-export-"+time.Now().Format("20060102-150405"))
}
