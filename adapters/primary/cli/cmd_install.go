package cli

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"mem/adapters/primary/setup"
)

func CmdInstall(deps *Deps, args []string) {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.Parse(args)

	target := "."
	if len(fs.Args()) > 0 {
		target = fs.Args()[0]
	}

	target, err := filepath.Abs(target)
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

	fmt.Printf("📦 Instalando gomemory en %s\n\n", target)

	// 1. Copy binary
	self, err := os.Executable()
	if err != nil {
		fail("obtener ruta del binario: %v", err)
	}

	destBin := filepath.Join(target, "mem")
	selfInfo, selfErr := os.Stat(self)
	destInfo, destErr := os.Stat(destBin)
	sameFile := selfErr == nil && destErr == nil && os.SameFile(selfInfo, destInfo)

	if sameFile {
		fmt.Printf("  ✅ Binario ya es el actual (%s), no se reemplaza\n", destBin)
	} else {
		if _, err := os.Stat(destBin); err == nil {
			if err := os.Remove(destBin); err != nil {
				fail("eliminar binario anterior: %v", err)
			}
		}
		if err := copyFile(self, destBin); err != nil {
			fail("copiar binario: %v", err)
		}
		os.Chmod(destBin, 0755)
		fmt.Printf("  ✅ Binario copiado a %s\n", destBin)
	}

	// 2. Init memory (or verify existing)
	dbName := "mem.db"
	dbPath := filepath.Join(target, deps.ProjectRepo.MemDir(), dbName)
	if _, err := os.Stat(dbPath); err == nil {
		err := deps.ProjectRepo.Init(target)
		if err == nil {
			fmt.Printf("  ✅ Memoria existente verificada\n")
		} else {
			fmt.Printf("  ⚠️  Base de datos dañada, reinicializando: %v\n", err)
			if err := runIn(target, destBin, "init", "--force"); err != nil {
				fmt.Printf("  ⚠️  Error al reinicializar: %v\n", err)
			} else {
				fmt.Printf("  ✅ Memoria reinicializada\n")
			}
		}
	} else {
		if err := runIn(target, destBin, "init"); err != nil {
			fmt.Printf("  ⚠️  Error al inicializar: %v\n", err)
		} else {
			fmt.Printf("  ✅ Memoria inicializada\n")
		}
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
	} else {
		fmt.Printf("  ✅ .gitignore actualizado\n")
	}

	// 4. Update AGENTS.md or CLAUDE.md (preámbulo de reglas + protocolo de memoria)
	preamble := embeddedTemplate("agent-preamble.md")
	integrationBlock := buildIntegrationBlock()

	agentFiles := []string{"AGENTS.md", "CLAUDE.md", "CLAUDE.txt", ".cursorrules", ".windsurfrules"}
	updated := false
	found := 0

	for _, fname := range agentFiles {
		fpath := filepath.Join(target, fname)
		if _, err := os.Stat(fpath); err != nil {
			continue
		}
		found++
		data, _ := os.ReadFile(fpath)
		newContent, changed := composeAgentFile(string(data), preamble, integrationBlock)
		if !changed {
			continue
		}
		if err := os.WriteFile(fpath, []byte(newContent), 0644); err != nil {
			continue
		}
		fmt.Printf("  ✅ %s actualizado (reglas de trabajo + protocolo de memoria)\n", fname)
		updated = true
	}

	if found > 0 && !updated {
		fmt.Printf("  ✅ Integración ya presente en AGENTS.md/CLAUDE.md\n")
	}
	if found == 0 {
		created := 0
		for _, fname := range []string{"AGENTS.md", "CLAUDE.md"} {
			dst := filepath.Join(target, fname)
			if err := os.WriteFile(dst, []byte(defaultAgentFile(fname, preamble)), 0644); err != nil {
				continue
			}
			fmt.Printf("  ✅ %s creado (reglas de trabajo + protocolo de memoria)\n", fname)
			created++
		}
		if created == 0 {
			fmt.Printf("  ⚠️  No se pudo crear AGENTS.md/CLAUDE.md en el proyecto destino.\n")
		}
	}

	// 4b. Copiar la constitución del proyecto (parte del "pack" de trabajo).
	if consti := embeddedTemplate("speckit-constitution-gen.md"); consti != "" {
		cpath := filepath.Join(target, "speckit-constitution-gen.md")
		if _, err := os.Stat(cpath); err != nil {
			if err := os.WriteFile(cpath, []byte(consti), 0644); err != nil {
				fmt.Printf("  ⚠️  Error al copiar la constitución: %v\n", err)
			} else {
				fmt.Printf("  ✅ Constitución copiada a %s\n", cpath)
			}
		} else {
			fmt.Printf("  ✅ Constitución ya presente, no se sobrescribe\n")
		}
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
	if err := setup.InstallOpenCode(target, ref); err != nil {
		fmt.Printf("  ⚠️  opencode: %v\n", err)
	}
	if err := setup.InstallClaudeCode(target, ref); err != nil {
		fmt.Printf("  ⚠️  claude-code: %v\n", err)
	}
	setupCursor(target)
	setupWindsurf(target)
	setupCline(target)
	setupCodex(target)

	// 6. Apply autoApprove settings if configured
	settings := deps.SettingsRepo.Read(target)
	if settings.AutoApprove {
		deps.SettingsRepo.ApplyAutoApprove(target, settings)
		fmt.Println("  ✅ Auto-approve aplicado desde settings")
	}

	fmt.Println()
	fmt.Println("🎉 gomemory instalado. Ahora puedes:")
	fmt.Println()
	fmt.Println("   cd", target)
	fmt.Println("   ./mem            # Abrir TUI")
	fmt.Println("   ./mem --help     # Ver todos los comandos")
	fmt.Println()
	fmt.Println("   Y el agente AI usará la memoria automáticamente al leer AGENTS.md.")
	fmt.Println()
	fmt.Println("   💡 Desde v1.9, gomemory ya no necesita instalarse por proyecto para")
	fmt.Println("      Claude Code/Codex/OpenCode: 'mem setup-mcp --scope global --agents claude,codex,opencode'")
	fmt.Println("      registra el MCP una sola vez para todos tus proyectos, y el store de")
	fmt.Println("      memoria se crea solo al primer uso (mem save/mem mcp), sin 'mem install'.")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

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
const integrationVersionMarker = "<!-- gomemory-protocol-v4 -->"
const workRulesMarker = "<!-- gomemory-workrules-v1 -->"

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
// preámbulo de reglas de trabajo y luego el bloque del protocolo de memoria.
// Es idempotente: si ambos marcadores ya están presentes, no cambia nada.
// Devuelve el contenido resultante y si hubo cambios.
func composeAgentFile(existing, preamble, integration string) (string, bool) {
	out := existing
	changed := false

	// 1. Preámbulo de reglas, SIEMPRE antes del protocolo de memoria.
	if preamble != "" && !strings.Contains(out, workRulesMarker) {
		block := strings.TrimRight(preamble, "\n")
		if idx := protocolStart(out); idx != -1 {
			out = strings.TrimRight(out[:idx], "\n") + "\n\n" + block + "\n\n" + out[idx:]
		} else {
			out = strings.TrimRight(out, "\n") + "\n\n" + block + "\n"
		}
		changed = true
	}

	// 2. Protocolo de memoria (versionado). Reemplaza bloques de cualquier
	// versión anterior si existen (ver protocolStart).
	if !strings.Contains(out, integrationVersionMarker) {
		if idx := protocolStart(out); idx != -1 {
			out = strings.TrimRight(out[:idx], "\n") + "\n" + integration
		} else {
			out = strings.TrimRight(out, "\n") + "\n" + integration
		}
		changed = true
	}

	return out, changed
}

// versionMarkerPattern reconoce el marcador de versión del protocolo sin
// importar el número de versión (v1, v2, v3...), para poder ubicar el
// comienzo real del bloque instalado aunque sea de una versión anterior a
// integrationVersionMarker.
var versionMarkerPattern = regexp.MustCompile(`<!-- gomemory-protocol-v\d+ -->`)

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
		"",
		"Si el MCP no está disponible en el agente actual, usa el CLI equivalente:",
		bt + `./mem save -t "título" -y tipo "contenido"` + bt + ", " + bt + `./mem search "tema"` + bt + ", " + bt + "./mem context" + bt + ", " + bt + "./mem session start|end" + bt + ", " + bt + "./mem forget <id>" + bt + ", " + bt + "./mem judge -r <veredicto> -m \"razón\" <id1> <id2>" + bt + ".",
		"",
		"### GUARDAR PROACTIVAMENTE — no esperes a que el usuario lo pida",
		"Llama a " + bt + "save_memory" + bt + " (o " + bt + "./mem save" + bt + ") INMEDIATAMENTE después de:",
		"- Una decisión técnica o de arquitectura",
		"- Un bug corregido (incluye causa raíz)",
		"- Un patrón o convención establecida",
		"- Un descubrimiento no obvio sobre el código",
		"- El usuario confirma o rechaza un enfoque propuesto",
		"- El usuario expresa una preferencia o corrige tu forma de interactuar (" + bt + "type=preference" + bt + ") — esto incluye memoria interactiva de sesión (estilo, tono, flujo de trabajo); no la guardes fuera de gomemory",
		"",
		"Autochequeo después de CADA tarea: \"¿Tomé una decisión, corregí un bug, descubrí algo",
		"o establecí una convención? Si sí → " + bt + "save_memory" + bt + " AHORA.\"",
		"",
		"### Juez imparcial (memorias en conflicto)",
		"Si el contexto muestra " + bt + "## Conflictos sin resolver" + bt + ", o notas dos memorias que se",
		"contradicen al buscar, no asumas que la más reciente tiene razón: relee el código/archivo",
		"fuente actual para verificar cuál refleja los hechos reales, y registra el veredicto con",
		bt + "judge_memories" + bt + " (o " + bt + "./mem judge" + bt + "), explicando en el razonamiento qué verificaste.",
		"",
		"### Privacidad",
		"Si vas a guardar un secreto, token o credencial, envuelve esa parte en",
		bt + "<private>...</private>" + bt + " — nunca se persiste.",
		"",
		"### Al inicio de cada sesión:",
		"1. Llama " + bt + "get_context()" + bt + " (o " + bt + "./mem context" + bt + ") para cargar el contexto histórico",
		"2. Si no hay sesión activa, llama " + bt + "start_session()" + bt + " (o " + bt + "./mem session start" + bt + ")",
		"",
		"### Al cerrar la sesión (antes de decir \"listo\"):",
		"Llama " + bt + "end_session(summary)" + bt + " (o " + bt + `./mem session end -s "..."` + bt + ") con un resumen de lo realizado.",
		"",
		"### Consultar memoria:",
		"- " + bt + "search_memories(query)" + bt + " (o " + bt + `./mem search "tema"` + bt + ") cuando el usuario pregunte por trabajo previo",
		"- " + bt + "./mem" + bt + " abre la TUI interactiva",
		"",
	}
	return strings.Join(lines, "\n")
}

// defaultAgentFile genera un AGENTS.md/CLAUDE.md universal desde cero,
// sin depender del cwd: el protocolo vive en el binario vía
// buildIntegrationBlock() y el preámbulo de reglas vía go:embed (templates/).
func defaultAgentFile(fname, preamble string) string {
	title := "# Instrucciones para agentes AI"
	if fname == "CLAUDE.md" {
		title = "# Instrucciones para Claude Code"
	}
	content := title + "\n"
	if preamble != "" {
		content += strings.TrimRight(preamble, "\n") + "\n"
	}
	content += buildIntegrationBlock()
	return content
}
