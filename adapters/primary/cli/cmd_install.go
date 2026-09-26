package cli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"mem/adapters/primary/console"
	"mem/adapters/primary/setup"
	"mem/adapters/secondary/persistence"
	"mem/domain"
)

func CmdInstall(deps *Deps, args []string) {
	opts, err := parseInstallArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗", err)
		os.Exit(2)
	}

	target, err := filepath.Abs(opts.target)
	if err != nil {
		fail("ruta inválida: %v", err)
	}

	stat, err := os.Stat(target)
	if err != nil {
		fail("no existe: %v", err)
	}
	if !stat.IsDir() {
		fail("%s no es un directorio", target)
	}

	// Selección (feature 034, US3): se decide antes de escribir nada. Sin
	// terminal interactiva (hooks, CI, `curl | sh`, el subproceso de update)
	// o con --yes nunca se pregunta (FR-022).
	stored := deps.SettingsRepo.Read(target)
	saved := installSelection{agents: stored.Agents, scope: stored.AgentScope}
	home, _ := os.UserHomeDir()
	choices := detectAgents(home, exec.LookPath)
	sel := resolveInstallSelection(opts, saved, choices)
	_, hasGlobal := resolveGlobalBinary(target)
	if ui := console.New(console.DetectMode(console.DetectEnv(), opts.yes)); ui != nil && len(opts.agents) == 0 {
		asked, err := askInstallSelection(ui, choices, hasGlobal, saved)
		if err != nil {
			fmt.Println("Instalación cancelada. No se escribió nada.")
			return
		}
		if opts.scope != "" {
			asked.scope = opts.scope
		}
		sel = asked
	}
	if sel.scope == "global" {
		for _, agent := range sel.agents {
			if !globalScopeAgents[agent] {
				fmt.Fprintf(os.Stderr, "✗ %s no admite --scope global; usa --scope project\n", agent)
				os.Exit(2)
			}
		}
	}

	fmt.Printf("📦 Instalando gomemory en %s\n\n", target)
	var results []console.StepResult
	addStep := func(name string, err error, manual string) {
		step := console.StepResult{Name: name, Status: console.StepOK}
		if err != nil {
			step.Status, step.Detail, step.Manual = console.StepWarn, err.Error(), manual
		}
		results = append(results, step)
	}

	// Instalación nueva = proyecto sin settings.json todavía. Se mira ANTES de
	// cualquier paso que pueda crearlo (feature 033, research R13).
	nuevaInstalacion := !settingsFileExists(target)

	// 1. Binario (feature 034). destBin es el que ejecutan los subprocesos de
	// los pasos siguientes: el global si existe, la copia del proyecto si no.
	self, err := os.Executable()
	if err != nil {
		fail("obtener ruta del binario: %v", err)
	}
	if !hasGlobal && sel.binary == "global" && console.DetectMode(console.DetectEnv(), opts.yes) != console.ModeNonInteractive {
		// La persona eligió la instalación global en la consola (FR-020).
		if err := installGlobalBinary(self); err != nil {
			fmt.Printf("  ⚠️  No se pudo instalar el binario global: %v\n", err)
			addStep("Binario global", err, "scripts/install.sh")
		}
	}
	destBin, binStep := installBinary(target, self)
	results = append(results, binStep)

	// 2. Init memory (or verify existing)
	dbName := "mem.db"
	dbPath := filepath.Join(target, deps.ProjectRepo.MemDir(), dbName)
	var memoryErr error
	if _, err := os.Stat(dbPath); err == nil {
		err := deps.ProjectRepo.Init(target)
		if err == nil {
			fmt.Printf("  ✅ Memoria existente verificada\n")
		} else {
			fmt.Printf("  ⚠️  Base de datos dañada, reinicializando: %v\n", err)
			if err := runIn(target, destBin, "init", "--force"); err != nil {
				fmt.Printf("  ⚠️  Error al reinicializar: %v\n", err)
				memoryErr = err
			} else {
				fmt.Printf("  ✅ Memoria reinicializada\n")
			}
		}
	} else {
		if err := runIn(target, destBin, "init"); err != nil {
			fmt.Printf("  ⚠️  Error al inicializar: %v\n", err)
			memoryErr = err
		} else {
			fmt.Printf("  ✅ Memoria inicializada\n")
		}
	}
	addStep("Memoria", memoryErr, "ejecuta mem init en el proyecto")

	// 2a. Registrar la ruta del proyecto en el almacén global: sin ella, la
	// desinstalación de sistema no podría encontrar este proyecto para
	// retirar su integración (feature 034, FR-013).
	if err := persistence.RegisterProject(target); err != nil {
		fmt.Printf("  ⚠️  No se pudo registrar el proyecto en el almacén global: %v\n", err)
		addStep("Registro del proyecto", err, "ejecuta mem install --yes de nuevo")
	} else {
		addStep("Registro del proyecto", nil, "")
	}

	// 2b. Sembrar las memorias por defecto (feature 021): las reglas de trabajo
	// y la constitución viven en la memoria, no en archivos del repositorio.
	// Nunca pisa una semilla existente — en cuanto existe, es de la persona.
	//
	// Va por SUBPROCESO con cwd en el destino, igual que el paso 2 hace con
	// `init`. No es un rodeo: `install` está en rootIndependentCommands y se
	// despacha sin contenedor, así que su Deps no trae MemoryRepo; y el destino
	// puede ser un proyecto distinto del directorio actual, con su propio store.
	if err := runIn(target, destBin, "seed"); err != nil {
		fmt.Printf("  ⚠️  No se pudieron sembrar las memorias por defecto: %v\n", err)
		addStep("Memorias por defecto", err, "ejecuta mem seed en el proyecto")
	} else {
		addStep("Memorias por defecto", nil, "")
	}

	// 3. Update .gitignore
	gitignore := filepath.Join(target, ".gitignore")
	content := ""
	if _, err := os.Stat(gitignore); err == nil {
		data, _ := os.ReadFile(gitignore)
		content = string(data)
	}

	needed := []string{".memory/", "mem\n"}
	for _, line := range needed {
		if !strings.Contains(content, line) {
			if content != "" && !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			content += line
		}
	}

	if err := os.WriteFile(gitignore, []byte(content), 0644); err != nil {
		fmt.Printf("  ⚠️  Error al actualizar .gitignore: %v\n", err)
		addStep(".gitignore", err, "añade .memory/ y /mem a .gitignore")
	} else {
		fmt.Printf("  ✅ .gitignore actualizado\n")
		addStep(".gitignore", nil, "")
	}

	// 3b. Retirar los artefactos de instalaciones anteriores (feature 021, US3).
	// Va DESPUÉS de la siembra a propósito: si sembrar fallara, los archivos
	// legados siguen en su sitio y la persona no se queda sin las reglas ni en
	// la memoria ni en el repositorio.
	cleanupLegacyArtifacts(target, deps.ProjectRepo.MemDir())

	// Los pasos 4 (AGENTS.md/CLAUDE.md) y 4b (copia de la constitución) se
	// retiraron en la feature 021. El bloque de protocolo que se escribía en
	// esos archivos era una SEGUNDA copia del texto que el agente ya recibe en
	// la respuesta initialize del MCP (ver cmd_mcp.go, ServerOptions.Instructions),
	// y la constitución copiada eran 635 líneas congeladas que divergían de la
	// fuente en cuanto alguien editaba una de las dos. Ambas viven ahora como
	// memorias semilla (paso 2b) y se administran con `mem docs`.
	//
	// composeAgentFile/protocolStart/protocolEnd NO se eliminaron: las usa
	// `setup-mcp --scope global` para el ámbito de USUARIO, que sigue vigente.

	// 4c. Distribuir el brazo extensor gomemory-context (spec 011/012):
	// no-op silencioso si el proyecto destino no tiene spec-kit (.specify/
	// ausente) — nunca bloquea el resto de la instalación.
	if err := setup.InstallSpeckitExtension(target, TemplatesFS); err != nil {
		fmt.Printf("  ⚠️  Error al distribuir el brazo extensor spec-kit: %v\n", err)
		addStep("Extensión Spec Kit", err, "revisa .specify y repite mem install --yes")
	} else {
		addStep("Extensión Spec Kit", nil, "")
	}

	// 4d. Envoltorios nativos del método de planificación atómica (spec 013).
	// Capa OPCIONAL: el disparador ya viaja en el bloque de protocolo que todos
	// los agentes leen, así que un fallo aquí nunca bloquea la instalación.
	wrapperAgents := selectedWrapperAgents(sel)
	if len(wrapperAgents) == 0 {
		// Ni Claude Code ni OpenCode elegidos: no hay envoltorios que escribir.
	} else if err := setup.InstallAtomicPlanWrappers(target, PlanMethod(), wrapperAgents...); err != nil {
		fmt.Printf("  ⚠️  Error al distribuir el método de planificación atómica: %v\n", err)
		addStep("Planificación atómica", err, "repite mem install --yes")
	} else if PlanMethod() != "" {
		fmt.Printf("  ✅ Método de planificación atómica distribuido (claude-code, opencode)\n")
		addStep("Planificación atómica", nil, "")
	}

	// 4e. Envoltorio nativo de la constitución (feature 021, FR-027). Capa
	// OPCIONAL, mismo criterio que 4d: siempre queda `mem constitution`, así
	// que un fallo aquí nunca bloquea la instalación. El envoltorio NO lleva
	// copia del texto — resuelve desde la memoria en cada invocación.
	if len(wrapperAgents) == 0 {
		// Sin agentes con envoltorio nativo.
	} else if err := setup.InstallConstitutionWrappers(target, wrapperAgents...); err != nil {
		fmt.Printf("  ⚠️  Error al distribuir el envoltorio de la constitución: %v\n", err)
		addStep("Constitución", err, "repite mem install --yes")
	} else {
		fmt.Printf("  ✅ Envoltorio /constitution distribuido (claude-code, opencode)\n")
		addStep("Constitución", nil, "")
	}

	// 5. MCP server config + plugins/hooks for all agents.
	// Para OpenCode y Claude Code instalamos el plugin completo (que incluye los
	// hooks automáticos), no solo el MCP: `install` debe dejar todo listo en un
	// solo paso. El resto de agentes solo soportan config MCP.
	fmt.Printf("  🔌 Configurando agentes (MCP + hooks)...\n")
	br := binRefFor(target)
	ref := setup.AgentRef{
		HookCommand: br.HookCommand,
		MCPCommand:  br.MCPCommand,
		MCPArgs:     br.MCPArgs,
	}
	// Solo los agentes elegidos (FR-021). Con alcance global, el registro de
	// usuario lo hace el mismo flujo que `mem setup-mcp --scope global`.
	if sel.scope == "global" {
		var globales []string
		for _, a := range sel.agents {
			if globalScopeAgents[a] {
				globales = append(globales, a)
			}
		}
		if len(globales) > 0 {
			registered := runGlobalScopeSetup(globales)
			for _, a := range globales {
				if registered[a] {
					addStep("Integración "+agentLabel(a)+" (global)", nil, "")
				} else {
					addStep("Integración "+agentLabel(a)+" (global)", fmt.Errorf("no se completó el registro"), "repite mem setup-mcp --scope global --agents "+a)
				}
			}
		}
	}
	if sel.scope == "project" && sel.has("opencode") {
		if err := setup.InstallOpenCode(target, ref); err != nil {
			fmt.Printf("  ⚠️  opencode: %v\n", err)
			addStep("Integración OpenCode", err, "repite mem install --yes --agents opencode")
		} else {
			addStep("Integración OpenCode", nil, "")
		}
	}
	if sel.scope == "project" && sel.has("claude") {
		if err := setup.InstallClaudeCode(target, ref); err != nil {
			fmt.Printf("  ⚠️  claude-code: %v\n", err)
			addStep("Integración Claude Code", err, "repite mem install --yes --agents claude")
		} else {
			addStep("Integración Claude Code", nil, "")
		}
	}
	if sel.scope == "project" && sel.has("cursor") {
		if setupCursor(target) {
			addStep("Integración Cursor", nil, "")
		} else {
			addStep("Integración Cursor", fmt.Errorf("no se pudo configurar Cursor"), "repite mem install --yes --agents cursor")
		}
	}
	if sel.scope == "project" && sel.has("codex") {
		if setupCodex(target) {
			addStep("Integración Codex", nil, "")
		} else {
			addStep("Integración Codex", fmt.Errorf("no se pudo configurar Codex"), "repite mem install --yes --agents codex")
		}
	}
	if sel.scope == "project" && sel.has("windsurf") {
		if setupWindsurf(target) {
			addStep("Integración Windsurf", nil, "")
		} else {
			addStep("Integración Windsurf", fmt.Errorf("no se pudo configurar Windsurf"), "repite mem install --yes --agents windsurf")
		}
	}
	if sel.scope == "project" && sel.has("cline") {
		if setupCline(target) {
			addStep("Integración Cline", nil, "")
		} else {
			addStep("Integración Cline", fmt.Errorf("no se pudo configurar Cline"), "repite mem install --yes --agents cline")
		}
	}
	// windsurf y cline se retiraron de la instalación automática (feature 021):
	// creaban .windsurf/ y .cline/ en la raíz de TODO proyecto para alojar un
	// único JSON de configuración. Siguen soportados por la vía explícita:
	// `mem setup-mcp --agents windsurf,cline`.

	// 6. Nivel de compresión por defecto (feature 033): una instalación nueva
	// arranca en max, porque toda omisión es recuperable. Una existente
	// conserva su nivel (ausente = structural, el comportamiento anterior).
	if err := applyInstallCompressionDefault(deps, target, nuevaInstalacion); err != nil {
		fmt.Printf("  ⚠️  Nivel de compresión: %v\n", err)
		addStep("Nivel de compresión", err, "repite mem install --yes")
	} else {
		addStep("Nivel de compresión", nil, "")
	}

	// Permisos privados en todo el store global, no solo en el proyecto que
	// se abre: `mem update` ejecuta install, así que cada actualización cierra
	// también los proyectos que no se vuelven a abrir (C-001 de acr_ad72cce1).
	if n, err := persistence.HardenGlobalStore(); err != nil {
		fmt.Printf("  ⚠️  Permisos del store global: %v\n", err)
		addStep("Permisos del almacén", err, "revisa permisos del almacén y repite mem install --yes")
	} else if n > 0 {
		fmt.Printf("  🔒 Permisos privados restaurados en %d entradas del store global\n", n)
		addStep("Permisos del almacén", nil, "")
	} else {
		addStep("Permisos del almacén", nil, "")
	}

	// La selección queda guardada: las reinstalaciones (también la de
	// `mem update`) la reutilizan sin preguntar (FR-023).
	if s := deps.SettingsRepo.Read(target); strings.Join(s.Agents, ",") != strings.Join(sel.agents, ",") || s.AgentScope != sel.scope {
		s.Agents, s.AgentScope = sel.agents, sel.scope
		if err := deps.SettingsRepo.Write(target, s); err != nil {
			fmt.Printf("  ⚠️  No se pudo guardar la selección de agentes: %v\n", err)
			addStep("Selección de agentes", err, "repite mem install --yes")
		} else {
			addStep("Selección de agentes", nil, "")
		}
	} else {
		addStep("Selección de agentes", nil, "")
	}

	// 7. Apply autoApprove settings if configured
	settings := deps.SettingsRepo.Read(target)
	if settings.AutoApprove {
		deps.SettingsRepo.ApplyAutoApprove(target, settings)
		fmt.Println("  ✅ Auto-approve aplicado desde settings")
	}

	fmt.Println()
	fmt.Println("🎉 gomemory instalado. Ahora puedes:")
	fmt.Println()
	fmt.Println("   cd", target)
	for _, line := range installNextSteps() {
		fmt.Println(line)
	}
	fmt.Println("\nResumen:")
	reporter := console.NewReporter(os.Stdout, console.DetectMode(console.DetectEnv(), opts.yes) == console.ModeRich)
	for _, result := range results {
		reporter.Done(result)
	}
}

// installBinary resuelve qué binario usa el proyecto. Con un global en el PATH
// no se copia nada (FR-001) y una copia de gomemory que ya estuviera se retira
// (FR-003); sin global, el proyecto recibe su copia como hasta ahora (FR-002).
// El paso devuelto lleva al resumen final el aviso de retirada (FR-004a).
func installBinary(target, self string) (string, console.StepResult) {
	step := console.StepResult{Name: "Binario", Status: console.StepOK}
	if global, ok := resolveGlobalBinary(target); ok {
		step.Detail = "global " + global
		if c, retired, err := retireLocalCopy(target, global); retired {
			step.Detail = retiredNotice(c, global)
			fmt.Printf("  ✅ %s\n", step.Detail)
		} else if err != nil {
			fmt.Printf("  ⚠️  No se pudo retirar %s: %v → bórralo a mano\n", c.Path, err)
			step.Status, step.Manual = console.StepWarn, "rm "+c.Path
			step.Detail = fmt.Sprintf("no se pudo retirar %s: %v", c.Path, err)
		}
		fmt.Printf("  ✅ Usa el binario global %s; no se copia al proyecto\n", global)
		return global, step
	}

	destBin := filepath.Join(target, memBinaryName())
	selfInfo, selfErr := os.Stat(self)
	destInfo, destErr := os.Stat(destBin)
	if selfErr == nil && destErr == nil && os.SameFile(selfInfo, destInfo) {
		fmt.Printf("  ✅ Binario ya es el actual (%s), no se reemplaza\n", destBin)
	} else {
		if destErr == nil {
			if err := os.Remove(destBin); err != nil {
				fail("eliminar binario anterior: %v", err)
			}
		}
		if err := copyFile(self, destBin); err != nil {
			fail("copiar binario: %v", err)
		}
		if err := os.Chmod(destBin, 0755); err != nil {
			fail("ajustar permisos del binario: %v", err)
		}
		fmt.Printf("  ✅ Binario copiado a %s\n", destBin)
	}
	fmt.Println("  ℹ️  Sin `mem` global en el PATH: este proyecto usa su propia copia. Para una sola")
	fmt.Println("      instalación que se actualiza con `mem update`, instala gomemory con scripts/install.sh")
	fmt.Println("      (o mueve el binario a un directorio del PATH) y vuelve a ejecutar `mem install`.")
	step.Detail = "copia en " + destBin
	return destBin, step
}

// installNextSteps son las indicaciones finales de install. Nombran el binario
// del PATH: una copia local ya no es la forma recomendada (FR-005).
func installNextSteps() []string {
	return []string{
		"   mem              # Abrir TUI",
		"   mem --help       # Ver todos los comandos",
		"",
		"   Las reglas de trabajo y la constitución quedaron guardadas en la memoria del",
		"   proyecto: el agente recibe las reglas automáticamente en get_context() y aplica",
		"   la constitución con /constitution. Ya no se generan archivos de instrucciones.",
		"",
		"   Para poner las de tu equipo:  mem docs export rules -o reglas.md",
		"                                 mem docs import rules reglas.md",
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}

func runIn(dir, bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

const integrationMarker = "## Memoria Persistente"

// integrationVersionMarker es un alias local de domain.ProtocolVersionMarker
// (feature 019): la fuente única vive en domain/protocol.go para que este
// instalador y el inspector de cobertura (adapters/primary/setup) nunca
// puedan divergir sobre cuál es la versión vigente del bloque de protocolo.
const integrationVersionMarker = domain.ProtocolVersionMarker
const universalInstructionsMarker = domain.UniversalInstructionsVersionMarker
const universalInstructionsEndMarker = "<!-- gomemory-universal-agent-instructions-end -->"

// integrationEndMarker delimita el final del bloque de protocolo desde la
// versión v8 (feature 019, FR-015). Los bloques v1..v7 no lo llevan: para
// esos, protocolEnd aplica la regla de límite de data-model.md §6 (siguiente
// encabezado de nivel 2 tras el propio título del bloque, o EOF).
const integrationEndMarker = "<!-- gomemory-protocol-end -->"

// TemplatesFS contiene los templates embebidos (preámbulo de reglas de trabajo
// y constitución). Lo inyecta infrastructure/main.go vía go:embed. Si es nil
// (p. ej. en algunos tests), embeddedTemplate devuelve "" y el instalador
// degrada con gracia: omite el preámbulo/constitución sin fallar.
var TemplatesFS fs.FS

func embeddedTemplate(name string) string {
	if TemplatesFS == nil {
		return ""
	}
	data, err := fs.ReadFile(TemplatesFS, "templates/"+name)
	if err != nil {
		return ""
	}
	return string(data)
}

// composeAgentFile garantiza que el archivo de agente contenga, en orden, el
// baseline universal y luego el bloque dinámico del protocolo de memoria.
// Es idempotente y actualiza únicamente los bloques gestionados por gomemory.
// Devuelve el contenido resultante y si hubo cambios.
func composeAgentFile(existing, universal, integration string) (string, bool) {
	out := existing
	changed := false

	// 1. Baseline universal, SIEMPRE antes del protocolo de memoria. A
	// diferencia de las reglas específicas de un proyecto, este bloque vive en
	// las instrucciones globales y no se reinyecta por hook en cada turno.
	if universal != "" {
		block := strings.TrimRight(universal, "\n")
		if idx := universalInstructionsStart(out); idx != -1 {
			end := universalInstructionsEnd(out, idx)
			if strings.TrimSpace(out[idx:end]) != strings.TrimSpace(block) {
				out = strings.TrimRight(out[:idx], "\n") + "\n\n" + block + "\n" + strings.TrimLeft(out[end:], "\n")
				changed = true
			}
		} else if idx := protocolStart(out); idx != -1 {
			out = strings.TrimRight(out[:idx], "\n") + "\n\n" + block + "\n\n" + out[idx:]
			changed = true
		} else {
			out = strings.TrimRight(out, "\n") + "\n\n" + block + "\n"
			changed = true
		}
	}

	// 2. Protocolo de memoria (versionado). Reemplaza bloques de cualquier
	// versión anterior si existen (ver protocolStart/protocolEnd), preservando
	// TODO el contenido ajeno que quede antes y después del bloque viejo
	// (feature 019, FR-015 — antes de esto, out[:idx] + integration descartaba
	// sin avisar cualquier cosa que la persona tuviera después del bloque).
	if !strings.Contains(out, integrationVersionMarker) {
		if idx := protocolStart(out); idx != -1 {
			end := protocolEnd(out, idx)
			before := strings.TrimRight(out[:idx], "\n")
			after := strings.TrimLeft(out[end:], "\n")
			if after != "" {
				out = before + "\n" + integration + "\n" + after
			} else {
				out = before + "\n" + integration
			}
		} else {
			out = strings.TrimRight(out, "\n") + "\n" + integration
		}
		changed = true
	}

	return out, changed
}

func universalInstructionsStart(content string) int {
	if loc := domain.UniversalInstructionsVersionPattern.FindStringIndex(content); loc != nil {
		return loc[0]
	}
	return strings.Index(content, universalInstructionsMarker)
}

func universalInstructionsEnd(content string, start int) int {
	if loc := strings.Index(content[start:], universalInstructionsEndMarker); loc != -1 {
		end := start + loc + len(universalInstructionsEndMarker)
		if end < len(content) && content[end] == '\n' {
			end++
		}
		return end
	}
	return len(content)
}

// versionMarkerPattern reconoce el marcador de versión del protocolo sin
// importar el número de versión (v1, v2, v3...), para poder ubicar el
// comienzo real del bloque instalado aunque sea de una versión anterior a
// integrationVersionMarker.
var versionMarkerPattern = domain.ProtocolVersionPattern

// protocolStart devuelve el índice donde empieza el bloque del protocolo de
// memoria: el marcador de versión (de la versión que esté instalada, para no
// dejar huérfana la línea del marcador viejo al subir de versión) o, si no
// hay marcador, el heading legado sin versionar; -1 si no existe ninguno.
func protocolStart(content string) int {
	if loc := versionMarkerPattern.FindStringIndex(content); loc != nil {
		return loc[0]
	}
	return strings.Index(content, integrationMarker)
}

// legacyH2Pattern reconoce un encabezado de nivel 2 al inicio de línea, para
// ubicar dónde termina un bloque de protocolo legado (sin marcador de fin).
var legacyH2Pattern = regexp.MustCompile(`(?m)^## `)

// protocolEnd devuelve el índice (offset en bytes de content) donde termina
// el bloque de protocolo que arranca en start (el valor de protocolStart), de
// modo que content[end:] sea exactamente el contenido posterior que pertenece
// a la persona (data-model.md §6, feature 019).
//
//   - Bloque v8+: termina justo después de integrationEndMarker (consumiendo
//     su salto de línea final, si lo hay).
//   - Bloque legado (v1..v7, sin marcador de fin): el marcador de versión va
//     seguido, en la línea siguiente y sin salto de línea en medio
//     (buildIntegrationBlock), del título propio del bloque (nivel 2). El
//     contenido ajeno empieza recién en el PRÓXIMO encabezado de nivel 2
//     después de ese título — no en el propio título, que es parte del
//     bloque — o al final del archivo si no hay ninguno.
//
// Ante cualquier forma inesperada (el marcador es la última línea, o el
// título propio es la última línea) se corta en el final del archivo: es
// preferible arrastrar una línea extra del bloque viejo a devorar contenido
// de la persona por error.
func protocolEnd(content string, start int) int {
	if loc := strings.Index(content[start:], integrationEndMarker); loc != -1 {
		end := start + loc + len(integrationEndMarker)
		if end < len(content) && content[end] == '\n' {
			end++
		}
		return end
	}

	firstNL := strings.IndexByte(content[start:], '\n')
	if firstNL == -1 {
		return len(content)
	}
	afterMarker := start + firstNL + 1

	secondNL := strings.IndexByte(content[afterMarker:], '\n')
	if secondNL == -1 {
		return len(content)
	}
	afterOwnHeading := afterMarker + secondNL + 1

	if loc := legacyH2Pattern.FindStringIndex(content[afterOwnHeading:]); loc != nil {
		return afterOwnHeading + loc[0]
	}
	return len(content)
}

func buildIntegrationBlock() string {
	bt := "`"
	lines := []string{
		"",
		integrationVersionMarker,
		integrationMarker + " (" + bt + "mem" + bt + ") — Protocolo Activo",
		"",
		"Este proyecto tiene el servidor MCP " + bt + "gomemory" + bt + " conectado. Este protocolo es OBLIGATORIO",
		"y SIEMPRE ACTIVO — no esperes a que el usuario lo pida explícitamente.",
		"",
		"### Herramientas MCP disponibles",
		"- " + bt + "save_memory(title, type, content, filepath?)" + bt + " — guarda una memoria",
		"- " + bt + "search_memories(query, limit?)" + bt + " — busca en memorias del proyecto",
		"- " + bt + "list_memories(limit?)" + bt + " — lista memorias recientes",
		"- " + bt + "get_memory(id)" + bt + " — obtiene una memoria específica",
		"- " + bt + "forget_memory(id)" + bt + " — borra una memoria puntual (irreversible)",
		"- " + bt + "judge_memories(id_a, id_b, verdict, confidence, reasoning)" + bt + " — veredicto imparcial entre dos memorias en conflicto",
		"- " + bt + "start_session()" + bt + " / " + bt + "end_session(summary?)" + bt + " — gestiona la sesión de trabajo",
		"- " + bt + "get_context()" + bt + " — contexto completo del proyecto en markdown",
		"- " + bt + "get_plan_context()" + bt + " — método de descomposición atómica + historial, para modo plan",
		"",
		"Grafo de código propio del proyecto: " + bt + strings.Join(domain.MCPCodeTools, bt+", "+bt) + bt + ".",
		"",
		"Si el servidor MCP " + bt + "codebase-memory-mcp" + bt + " está conectado, úsalo SIEMPRE para " +
			"exploración de código en vez de leer archivos a mano: " +
			bt + strings.Join(domain.CodebaseMemoryMCPDiscoveryTools, bt+", "+bt) + bt + ". " +
			"Para explorar el código usa las herramientas del grafo; para entregar un plan usa el árbol " +
			"de tareas atómicas. Lo que descubras con el grafo alimenta las hojas del árbol. " +
			"Si no está conectado, esta guía no aplica — no hay nada que invocar.",
		"",
		"Si el MCP no está disponible en el agente actual, usa el CLI equivalente (si " + bt + "mem" + bt + " no está en el PATH, usa " + bt + "./mem" + bt + "):",
		bt + `mem save -t "título" -y tipo "contenido"` + bt + ", " + bt + `mem search "tema"` + bt + ", " + bt + "mem context" + bt + ", " + bt + "mem plan-context" + bt + ", " + bt + "mem session start|end" + bt + ", " + bt + "mem forget <id>" + bt + ", " + bt + "mem judge -r <veredicto> -m \"razón\" <id1> <id2>" + bt + ".",
		"",
		"### GUARDAR PROACTIVAMENTE — no esperes a que el usuario lo pida",
		"Llama a " + bt + "save_memory" + bt + " (o " + bt + "mem save" + bt + ") INMEDIATAMENTE después de:",
		"- Una decisión técnica o de arquitectura",
		"- Un bug corregido (incluye causa raíz)",
		"- Un patrón o convención establecida",
		"- Un descubrimiento no obvio sobre el código",
		"- El usuario confirma o rechaza un enfoque propuesto",
		"- El usuario expresa una preferencia o corrige tu forma de interactuar (" + bt + "type=preference" + bt + ") — esto incluye memoria interactiva de sesión (estilo, tono, flujo de trabajo); no la guardes fuera de gomemory. Una preferencia es una REGLA FIJA, no un historial de incidentes: si la misma corrección se repite, usa " + bt + "topic_key" + bt + " (o el mismo título) para ACTUALIZAR esa memoria en vez de crear una nueva. No repitas ejemplos citados de la conducta incorrecta en el contenido — reforzarían el patrón en vez de corregirlo.",
		"",
		"Autochequeo después de CADA tarea: \"¿Tomé una decisión, corregí un bug, descubrí algo",
		"o establecí una convención? Si sí → " + bt + "save_memory" + bt + " AHORA.\"",
		"",
		"### Juez imparcial (memorias en conflicto)",
		"Si el contexto muestra " + bt + "## Conflictos sin resolver" + bt + ", o notas dos memorias que se",
		"contradicen al buscar, no asumas que la más reciente tiene razón: relee el código/archivo",
		"fuente actual para verificar cuál refleja los hechos reales, y registra el veredicto con",
		bt + "judge_memories" + bt + " (o " + bt + "mem judge" + bt + "), explicando en el razonamiento qué verificaste.",
		"",
		"### Privacidad",
		"Si vas a guardar un secreto, token o credencial, envuelve esa parte en",
		bt + "<private>...</private>" + bt + " — nunca se persiste.",
		"",
		"### Al entrar en modo plan:",
		"Llama " + bt + "get_plan_context()" + bt + " (o " + bt + "mem plan-context" + bt + ") ANTES de redactar el plan, en cuanto",
		"se dé cualquiera de estas tres situaciones: entras en un modo de planificación; la",
		"persona invoca un comando de planificación; o la solicitud pide un plan, un enfoque o",
		"una estrategia antes de tocar código.",
		"Devuelve el método de descomposición atómica y el historial del proyecto: aplica ese",
		"método al redactar. En modo plan, entrega el árbol de tareas y **detente** — no ejecutes.",
		"",
		"### Al inicio de cada sesión:",
		"1. Llama " + bt + "get_context()" + bt + " (o " + bt + "mem context" + bt + ") para cargar el contexto histórico",
		"2. Si no hay sesión activa, llama " + bt + "start_session()" + bt + " (o " + bt + "mem session start" + bt + ")",
		"",
		"### Al cerrar la sesión (antes de decir \"listo\"):",
		"Llama " + bt + "end_session(summary)" + bt + " (o " + bt + `mem session end -s "..."` + bt + ") con un resumen de lo realizado.",
		"",
		"### Consultar memoria:",
		"- " + bt + "search_memories(query)" + bt + " (o " + bt + `mem search "tema"` + bt + ") cuando el usuario pregunte por trabajo previo",
		"- " + bt + "mem" + bt + " abre la TUI interactiva",
		"",
		integrationEndMarker,
	}
	return strings.Join(lines, "\n")
}

// settingsFileExists dice si el proyecto ya tiene settings.json.
func settingsFileExists(target string) bool {
	_, err := os.Stat(filepath.Join(target, ".memory", "settings.json"))
	return err == nil
}

// applyInstallCompressionDefault escribe context_compression_level=max solo en
// una instalación nueva y solo si nadie lo fijó ya. Nunca toca una instalación
// existente: cambiar su comportamiento sin avisar está fuera de alcance.
func applyInstallCompressionDefault(deps *Deps, target string, nueva bool) error {
	if !nueva || deps.SettingsRepo == nil {
		return nil
	}
	s := deps.SettingsRepo.Read(target)
	if s.ContextCompressionLevel != "" {
		return nil
	}
	s.ContextCompressionLevel = domain.CompressionLevelMax
	if err := deps.SettingsRepo.Write(target, s); err != nil {
		return err
	}
	fmt.Println("  ✅ Compresión de contexto: max (instalación nueva; cámbiala con mem settings --compression-level)")
	return nil
}
