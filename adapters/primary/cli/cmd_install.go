package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	// 4. Update AGENTS.md or CLAUDE.md
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
		content := string(data)

		if strings.Contains(content, integrationVersionMarker) {
			continue
		}

		if idx := strings.Index(content, integrationMarker); idx != -1 {
			content = strings.TrimRight(content[:idx], "\n")
			if err := os.WriteFile(fpath, []byte(content+"\n"+integrationBlock), 0644); err != nil {
				continue
			}
			fmt.Printf("  ✅ %s actualizado a protocolo de memoria vigente\n", fname)
			updated = true
			continue
		}

		f, err := os.OpenFile(fpath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			continue
		}
		f.WriteString(integrationBlock)
		f.Close()
		fmt.Printf("  ✅ %s actualizado con instrucciones de memoria\n", fname)
		updated = true
	}

	if found > 0 && !updated {
		fmt.Printf("  ✅ Integración ya presente en AGENTS.md/CLAUDE.md\n")
	}
	if found == 0 {
		created := 0
		for _, fname := range []string{"AGENTS.md", "CLAUDE.md"} {
			dst := filepath.Join(target, fname)
			if err := os.WriteFile(dst, []byte(defaultAgentFile(fname)), 0644); err != nil {
				continue
			}
			fmt.Printf("  ✅ %s creado con protocolo de memoria\n", fname)
			created++
		}
		if created == 0 {
			fmt.Printf("  ⚠️  No se pudo crear AGENTS.md/CLAUDE.md en el proyecto destino.\n")
		}
	}

	// 5. MCP server config for all agents
	fmt.Printf("  🔌 Configurando MCP para agentes...\n")
	setupOpenCode(target, destBin)
	setupClaude(target, destBin)
	setupCursor(target, destBin)
	setupWindsurf(target, destBin)
	setupCline(target, destBin)
	setupCodex(target, destBin)

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
const integrationVersionMarker = "<!-- gomemory-protocol-v2 -->"

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
		"- " + bt + "start_session()" + bt + " / " + bt + "end_session(summary?)" + bt + " — gestiona la sesión de trabajo",
		"- " + bt + "get_context()" + bt + " — contexto completo del proyecto en markdown",
		"",
		"Si el MCP no está disponible en el agente actual, usa el CLI equivalente:",
		bt + `./mem save -t "título" -y tipo "contenido"` + bt + ", " + bt + `./mem search "tema"` + bt + ", " + bt + "./mem context" + bt + ", " + bt + "./mem session start|end" + bt + ".",
		"",
		"### GUARDAR PROACTIVAMENTE — no esperes a que el usuario lo pida",
		"Llama a " + bt + "save_memory" + bt + " (o " + bt + "./mem save" + bt + ") INMEDIATAMENTE después de:",
		"- Una decisión técnica o de arquitectura",
		"- Un bug corregido (incluye causa raíz)",
		"- Un patrón o convención establecida",
		"- Un descubrimiento no obvio sobre el código",
		"- El usuario confirma o rechaza un enfoque propuesto",
		"",
		"Autochequeo después de CADA tarea: \"¿Tomé una decisión, corregí un bug, descubrí algo",
		"o establecí una convención? Si sí → " + bt + "save_memory" + bt + " AHORA.\"",
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
// sin depender del cwd ni de archivos en disco — todo el contenido vive
// en el binario vía buildIntegrationBlock().
func defaultAgentFile(fname string) string {
	title := "# Instrucciones para agentes AI"
	if fname == "CLAUDE.md" {
		title = "# Instrucciones para Claude Code"
	}
	return title + "\n" + buildIntegrationBlock()
}

type MCPEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type OpenCodeConfig struct {
	MCPServers map[string]MCPEntry `json:"mcpServers,omitempty"`
}
