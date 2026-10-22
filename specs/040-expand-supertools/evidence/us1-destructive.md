# Evidencia US1 (T032): bloqueo destructivo y reglas propias en los 3 harnesses

- **Fecha**: 2026-10-21
- **Binario**: `mem` recompilado desde el árbol de trabajo (`gomemory 2.35.0` + 040 sin publicar), puesto el primero en el `PATH` de cada sesión.
- **Aislamiento**: proyecto temporal con `git init`, `GOMEMORY_DATA_HOME` temporal, e instalación con `HOME` aislado para Codex y OpenCode (se cablean a nivel de usuario). Ninguna configuración real de la persona se modificó; la credencial de Codex se enlazó, no se copió.
- **Instalación**: `mem install <proj> --yes --agents <harness> --scope project`, sin ningún paso de habilitación. Resultado: `hook expand` presente en los 5 eventos (Claude: `.claude/settings.json` con `--emit=claude`; Codex: `config.toml` con `--emit=json`; OpenCode: plugin con `tool.execute.before`).
- **Víctima**: un directorio con `keep.txt` en el scratchpad. Que siga existiendo demuestra el bloqueo.
- **Reglas propias** en `.gomemory/rules/`: `warn-probe` (bash, warn), `prompt-probe` (prompt, block), `stop-probe` (stop, block).

## Claude Code 2.1.306 (`claude -p`, headless)

| Escenario | Resultado literal (recortado) | Veredicto |
|---|---|---|
| `rm -rf <victim>` | `PreToolUse:Bash hook error: gomemory expand · regla destructive-shell: borrado recursivo forzado (rm -rf). Si es intencionado, …`; víctima intacta | PASS |
| warn en pre-tool, sin `--allowedTools` | aviso `[gomemory expand] regla warn-probe: …` visible; Claude Code siguió exigiendo su permiso y el comando no corrió | PASS (G6: no se aprobó en silencio) |
| block en prompt | `UserPromptSubmit operation blocked by hook: gomemory expand · regla prompt-probe: …` | PASS |
| block en stop | La primera ejecución entró en bucle (la regla bloqueaba cada intento de cerrar). **Defecto encontrado y corregido**: se respeta `stop_hook_active` (un solo bloqueo por fin de turno). Repetido: un bloqueo y fin en 10 s | PASS tras corrección |

## Codex 0.163.0 (sesión interactiva, TUI bajo PTY)

| Escenario | Resultado | Veredicto |
|---|---|---|
| `rm -rf <victim>` | `Command blocked by PreToolUse hook: gomemory expand · regla destructive-shell: …`; víctima intacta | PASS |
| block en prompt | el prompt no aparece en el registro de la sesión; `expand.block` registrado para `codex` | PASS |
| block en stop | «La sonda `stop-probe` bloqueó el cierre»; un solo bloqueo y fin | PASS |

Activaciones registradas en el almacén aislado: `codex|expand.block|3` (rm, prompt y stop).

## OpenCode 2.0.32 (`opencode run --standalone`)

| Escenario | Resultado | Veredicto |
|---|---|---|
| `rm -rf <victim>` sin `--standalone` | **No bloqueado**: `opencode run` se conectó al servicio 2.x en segundo plano que ya corría con la configuración real y el plugin anterior (log: `run=61c28d41`, arrancado a las 08:52). No es un fallo de expand; afecta a la verificación y a la instalación real: un servicio en marcha no carga el plugin nuevo hasta reiniciarse | — (documentado) |
| `rm -rf <victim>` con `--standalone` (servidor propio) | el agente informa de que la regla `destructive-shell` lo detuvo; víctima intacta; `opencode|expand.block` registrado | PASS |
| block en prompt | Primer intento: el texto de reemplazo se comportó como una petición y el modelo empezó a investigar. **Corregido**: el reemplazo ahora ordena no actuar. Repetido: una sola línea «El prompt se detuvo antes de enviarse porque la regla `prompt-probe` …» | PASS tras corrección |
| block en stop | Excepción de paridad declarada: OpenCode no puede impedir el fin de turno; el aviso se entrega en el siguiente turno de la misma sesión (en `opencode run` de un solo disparo no hay siguiente turno) | EXCEPCIÓN DECLARADA |

## Hallazgos que pasan a documentación (T075)

1. Tras instalar o actualizar, hay que **reiniciar el servicio de OpenCode 2.x**: el plugin nuevo no se carga en un servicio ya en marcha.
2. Excepciones de paridad de OpenCode: el bloqueo en stop no impide el fin de turno, y el aviso de pre-tool llega en el siguiente turno.
3. `mem expand off rules`, citado en el mensaje de bloqueo, llega en US2 (T036). Hasta entonces, el agente lo señala como inexistente.
