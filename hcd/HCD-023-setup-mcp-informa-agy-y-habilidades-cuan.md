# setup-mcp informa agy y habilidades cuando ya están al día

ID: HCD-023
Tipo: fix
Carril: fast
Motivo: Dos fallos con reproducción clara en la máquina de la persona y sin decisiones abiertas

## Intención
Que mem setup-mcp --scope global diga que agy ya está registrado cuando no hay cambios, como hace con los demás agentes, y que la sección Habilidades no quede vacía cuando todas están al día

## Reproducción
1. Con gomemory 2.45.0, `agy` en el PATH y `~/.gemini/config` ya registrado, `mem setup-mcp --scope global` no imprime ninguna línea `agy`, aunque cuenta su registro («4 registro(s)»). Causa: `setupAgyGlobal` (`adapters/primary/cli/cmd_mcp_setup.go`) solo imprime los archivos que cambiaron y, si no cambió ninguno, calla y devuelve `true`. Salida real de la persona el 2026-11-04.
2. En la misma ejecución, con todas las habilidades al día, la salida muestra el título `Habilidades:` sin ninguna línea debajo, porque `skillSummary` no imprime nada cuando la lista de archivos está vacía.

## Alcance
Incluido: `setupAgyGlobal` imprime `✓ agy: ya registrado en ~/.gemini/config (sin cambios)` cuando `InstallAgyGlobal` no cambia ningún archivo; la sección Habilidades imprime `✓ todas al día (sin cambios)` cuando ninguna familia escribió archivos; pruebas de ambos casos.
Excluido: cambiar qué escribe `InstallAgyGlobal` o las habilidades; cambiar el recuento de registros; la salida de `mem update` del binario anterior (los emojis vienen del v2.44.4 que se reemplaza, no del v2.45.0).

## ¿Qué pasa si no lo hacemos?
Quien vuelve a ejecutar `setup-mcp` no ve agy entre los agentes y cree que no se registró, aunque sí lo está; y el panel de Habilidades aparece como un título suelto, que parece un fallo de la salida.

## Alternativa más barata
Comprobar a mano `~/.gemini/config/mcp_config.json` o ejecutar `mem doctor` después de cada `setup-mcp`. Funciona, pero obliga a desconfiar de la salida del propio comando.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → `go test ./adapters/primary/cli -run TestSetupMCP_InformaSinCambios -count=1`
- [x] 2. Verificación completa → `go build ./... && go vet ./... && go test ./adapters/primary/... -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-04 05:37 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestSetupMCP_InformaSinCambios -count=1` (salida 1)
              Agentes (scope global, una vez para todos los proyectos):
                ✓ codex: ~/.codex/config.toml ya tiene el registro global de gomemory
              Integración:
              Habilidades:
              ✓ 2 registro(s) global(es) completados. Reinicia el agente para que los detecte.
          hcd023_setup_sin_cambios_test.go:30: sin cambios, la sección de habilidades no debe quedar vacía:
              Agentes (scope global, una vez para todos los proyectos):
                ✓ codex: ~/.codex/config.toml ya tiene el registro global de gomemory
              Integración:
              Habilidades:
              ✓ 2 registro(s) global(es) completados. Reinicia el agente para que los detecte.
      FAIL
      FAIL	mem/adapters/primary/cli	0.539s
      FAIL
- 2026-11-04 05:38 -05:00 · tarea 1 · verde · vigente · `go test ./adapters/primary/cli -run TestSetupMCP_InformaSinCambios -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.322s
- 2026-11-04 05:38 -05:00 · tarea 2 · verde · vigente · `go build ./... && go vet ./... && go test ./adapters/primary/... -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	17.653s
      ok  	mem/adapters/primary/cli/clikit	0.230s
      ok  	mem/adapters/primary/cli/hcd	2.374s
      ok  	mem/adapters/primary/cli/review	1.615s
      ok  	mem/adapters/primary/cli/socratica	1.662s
      ok  	mem/adapters/primary/console	0.246s
      ok  	mem/adapters/primary/setup	2.351s
      ok  	mem/adapters/primary/tui	7.477s

## Siguiente
Cerrada con aprobación sobre la revisión 3e5b25741d11.
