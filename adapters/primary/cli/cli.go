package cli

import (
	"fmt"
	"os"

	"mem/adapters/primary/tui"
	"mem/version"
)

func LaunchTUI(deps *Deps) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: no se pudo determinar el directorio de trabajo: %v\n", err)
		os.Exit(1)
	}

	project := deps.ProjectRepo.Key(root)

	if err := tui.Run(deps.MemoryRepo, deps.SettingsRepo, deps.MaintenanceRepo, root, project); err != nil {
		fmt.Fprintf(os.Stderr, "Error en TUI: %v\n", err)
		os.Exit(1)
	}
}

func Usage() {
	fmt.Printf("gomemory %s — Memoria colectiva para agentes AI\n", version.Version)
	fmt.Print(`
Uso:
  mem                              Abrir interfaz TUI
  mem init [--force]               Ya no es obligatorio: el store global se crea solo al primer uso
  mem migrate [--force]            Migrar .memory/mem.db legado (instalación por proyecto) al store global
  mem save [flags] <texto>         Guardar un aprendizaje
    -t, --title    Título descriptivo
    -y, --type     Tipo: learning|decision|architecture|bugfix|pattern|discovery|preference
    -f, --filepath Archivo relacionado

  mem capture [flags]              Guardar aprendizaje estructurado (What/Why/Where/Learned)
    -w, --what      ¿Qué se hizo?
    -y, --why       ¿Por qué?
    -f, --where     Archivos afectados
    -l, --learned   ¿Qué se aprendió?
    -t, --type      Tipo (default: learning)
    -i              Modo interactivo

  mem compare [flags] <id1> <id2>  Comparar dos memorias y persistir veredicto
    -r, --relation  related|compatible|scoped|conflicts_with|supersedes|not_conflict
    -c, --confidence  Confianza 0.0-1.0 (default: 1.0)
    -m, --reasoning   Razonamiento del veredicto
  mem compare list [-n N]          Listar relaciones guardadas
  mem judge ...                    Alias de compare

  mem forget <id>                  Borrar una memoria por ID (irreversible)

  mem project                      Detectar proyecto actual y mostrar información

  mem context [-w|--write]         Mostrar contexto de memoria
  mem search <query>               Buscar en la memoria
  mem install [dir]                Instalar gomemory en un proyecto
  mem uninstall [dir] [--yes]      Desinstalar gomemory por completo (reverso de install)
  mem session start                Iniciar nueva sesión
  mem session end [-s|--summary]   Finalizar sesión actual
  mem list [-n|--limit N]          Listar memorias recientes
  mem log [-n|--limit N]           Alias de list
  mem wrap <comando> [args...]     Ejecutar comando y preguntar si guardar
  mem mcp [--root <dir>]           Servidor MCP para agentes AI
  mem hook <evento>                Entrypoint de hooks de agentes (uso interno, invocado por Claude Code/OpenCode)
  mem serve [--port 9735]          Servidor HTTP background para plugins
  mem setup [--port 9735] <agent>  Instalar plugin para opencode|claude-code (flags ANTES del agente)
  mem setup-mcp [--agents a,b,c]   Configurar MCP: opencode, claude, cursor, windsurf, cline, codex, all
  mem settings [--auto-approve=true|false] [--show]
                                    Ver o cambiar auto-approve de las tools MCP
  mem purge [flags]                Vaciar memorias (proyecto actual por defecto)
    --project <nombre>  Proyecto objetivo (default: actual)
    --all                Purgar TODOS los proyectos del archivo
    --type <tipo>        Filtrar por tipo de memoria
    --older-than-days N  Solo memorias más viejas que N días
    --yes                Omitir el prompt de confirmación
  mem compact                      Compactar .memory/mem.db (recupera espacio, no borra nada)
  mem gc [flags]                   Garbage collection a demanda (memorias viejas)
    --project <nombre>  Proyecto objetivo (default: actual)
    --all                Aplicar a todos los proyectos
    --older-than-days N  Umbral de retención (default: 90)
    --yes                Omitir el prompt de confirmación
  mem index [--force]              Indexar el código Go del proyecto (grafo de símbolos)
  mem tui                          Abrir interfaz TUI explícitamente
  mem update [--check] [--version vX.Y.Z]
                                    Actualizar el binario y refrescar la integración del proyecto
  mem version                      Mostrar la versión instalada
  mem help                         Mostrar esta ayuda

Ejemplos:
  mem                              # Abrir TUI
  mem init                         # Primera vez
  mem save -t "usamos SQLite" -y decision "Base de datos SQLite"
  mem capture -w "implementar auth" -y "seguridad" -f "middleware.go" -l "usar JWT"
  mem capture -i                   # Modo interactivo
  mem compare -r supersedes -c 0.9 -m "la nueva decisión reemplaza a la anterior" 1 2
  mem project
  mem context --write
  mem search "autenticación"
  mem install ~/proyectos/mi-app   # Instalar en otro proyecto
  mem setup-mcp --agents codex,cursor
  mem settings --show
  mem session start
  mem session end -s "Implementado módulo de búsqueda"
`)
}
