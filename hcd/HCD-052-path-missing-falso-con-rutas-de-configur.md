# PATH-MISSING falso con rutas de configuración de agentes relativas al HOME

ID: HCD-052
Tipo: fix
Carril: fast
Motivo: Fix acotado con causa localizada en la lente PATH-MISSING; la persona eligió la Vía rápida

## Intención
Que mem hcd analyze no avise PATH-MISSING de una ruta citada que no existe en el repositorio cuando su primer segmento es un directorio de configuración de agente en el HOME (.claude, .codex, .gemini, .config, .cursor, .windsurf, .cline), igual que ya ignora las que empiezan por ~/

## Reproducción
Al cerrar HCD-051, `mem hcd analyze` avisó «PATH-MISSING: la ruta citada .codex/agents/splash.toml no existe en el repositorio», y lo mismo con `.codex` y `.gemini/config/agents/splash/agent.md`. Son rutas de configuración de agentes en el HOME, escritas sin `~/`. `addCitedPaths` (`domain/hcd_lens.go`) solo descarta las que empiezan por `~` o `/`, y `analyzeLenses` (`domain/hcd_stage.go`) avisa de toda ruta citada que falte en el repositorio. Prueba: `go test ./domain -run TestHCDLens_RepoMissingPaths -count=1`.

## Alcance
Incluido: `analyzeLenses` deja de avisar `PATH-MISSING` de una ruta ausente cuyo primer segmento es un directorio de configuración de agente: `.claude`, `.codex`, `.gemini`, `.config`, `.cursor`, `.windsurf` o `.cline`. El filtro es una función pura de `domain/hcd_lens.go` con su prueba.
Excluido: `CitedPaths` no cambia, porque también enlaza las fichas con los archivos citados y un `.claude/settings.json` del propio repositorio es una ruta legítima. Una ruta de esos directorios que sí existe en el repositorio nunca produce el aviso, así que el filtro no oculta nada real. Tampoco cambia el tratamiento de `~/` ni el de las rutas absolutas.

## ¿Qué pasa si no lo hacemos?
Cada unidad que documenta la instalación de un agente (skills, subagentes, hooks) cierra con avisos falsos. Corregirlos en el documento, por ejemplo escribiendo `~/.codex/...`, cambia la definición y obliga a reaprobar y a verificar todo otra vez.

## Alternativa más barata
Escribir siempre `~/` delante en los documentos. Funciona, pero depende de que cada persona y cada agente lo recuerden, y los criterios suelen citar rutas relativas al HOME de prueba (`.codex/agents/...` bajo un HOME falso). Filtrar en la lente cuesta unas pocas líneas.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → `go test ./domain -run TestHCDLens_RepoMissingPaths -count=1` en `domain/hcd_lens_test.go`
- [x] 2. Mantener en verde las lentes y el motor HCD → `go test ./domain ./application/usecases ./adapters/primary/cli/hcd ./tests/integration -run "HCD|Lens|Analyze" -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-07 07:19 -05:00 · tarea 1 · rojo observado · caducada · `go test ./domain -run TestHCDLens_RepoMissingPaths -count=1` (salida 1)
      --- FAIL: TestHCDLens_RepoMissingPaths (0.00s)
          hcd_lens_test.go:21: RepoMissingPaths = [".codex" ".codex/agents/splash.toml" ".gemini/config/agents/splash/agent.md" ".claude/agents/splash.md" ".config/opencode/agents/splash.md" ".cursor/mcp.json" ".windsurf/mcp_config.json" ".cline/mcp_settings.json" "src/exportar.go" ".github/workflows/nuevo.yml" ".codexx/x.md"], want ["src/exportar.go" ".github/workflows/nuevo.yml" ".codexx/x.md"]
      FAIL
      FAIL	mem/domain	0.341s
      FAIL
- 2026-11-07 07:19 -05:00 · tarea 1 · verde · vigente · `go test ./domain -run TestHCDLens_RepoMissingPaths -count=1` (salida 0)
      ok  	mem/domain	0.189s
- 2026-11-07 07:19 -05:00 · tarea 2 · verde · vigente · `go test ./domain ./application/usecases ./adapters/primary/cli/hcd ./tests/integration -run 'HCD|Lens|Analyze' -count=1` (salida 0)
      ok  	mem/domain	0.133s
      ok  	mem/application/usecases	6.422s
      ok  	mem/adapters/primary/cli/hcd	0.239s
      ok  	mem/tests/integration	4.812s

## Siguiente
Cerrada con aprobación sobre la revisión 539ed277130a.
