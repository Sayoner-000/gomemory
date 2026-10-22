# Contrato: `mem expand`

Se implementa en `adapters/primary/cli/cmd_expand.go` como `runExpand(deps, args, expandIO{out, err, in, tty}) int`, con E/S inyectable igual que `runSocratica` (memoria 555).

| Comando | Efecto | Salida | Código |
|---|---|---|---|
| `mem expand` / `mem expand status [--json]` | solo lectura | sección por harness (cableado ✓/✗, leído del archivo real de configuración), estado de cada supertool, reglas cargadas y errores de carga, activaciones de 7 días (`GuardSince`) y último informe de shield (resumen) | 0 |
| `mem expand off` | `expand_disabled=true` | `expand desactivado (emergencia). Reactiva con: mem expand on` | 0 |
| `mem expand on` | `expand_disabled=false`, vacía `expand_off` (reactiva **todo**, también las supertools apagadas una a una; la confirmación lo dice) | confirmación | 0 |
| `mem expand off <supertool>` | añade a `expand_off` | confirmación | 0; 2 si el nombre no es válido |
| `mem expand on <supertool>` | quita de `expand_off` | confirmación | 0; 2 si el nombre no es válido |
| `mem expand rules [--json]` | solo lectura | reglas efectivas (de serie y propias) y errores de carga | 0 |
| `mem expand rules --check [--event bash\|file\|prompt\|stop] [--json]` | solo lectura; evalúa stdin con las mismas reglas que el hook (sin `--event`: diff como `file`) | veredicto, regla y motivo | 0 allow, 1 warn, 2 block, 3 entrada ilegible |
| `mem expand instincts list [--json] [--all]` | solo lectura | slug, tipo, confianza, alcance, observaciones y estado; por defecto omite los `rejected` | 0 |
| `mem expand instincts confirm <slug>` | `status=confirmed` + `save_memory type=preference topic_key=instinct:<slug>` | confirmación | 0; 1 si no existe |
| `mem expand instincts reject <slug>` | `status=rejected` | confirmación | 0; 1 si no existe |
| `mem expand instincts evolve <slug>` | propuesta del LLM + `console.Confirm` + `InstallAgentSkill` | propuesta y rutas escritas | 0; 1 si falla la generación o no hay harness; 2 sin TTY; 3 si se cancela |
| `mem expand <otro>` | — | ayuda breve | 2 |

## `mem doctor --shield [--json]`

- Añade la sección `Shield` con los hallazgos agrupados por severidad y los archivos `skipped`. Con `--json`, añade `"shield": {hash, scanned_at, findings[], skipped[]}` a la salida existente sin romper sus claves (las claves nuevas son aditivas).
- Código de salida: 2 si hay algún critical y 1 si hay algún high; si no, el código actual de doctor.
- Guarda el informe en `.memory/expand-shield-last.json` y la huella en `.memory/.expand-shield-hash`.

## Ayuda

- `expand` entra en `helpIndex` (`cli.go:51`) y en la ayuda detallada. La prueba de índice bidireccional (memoria 572) cubre la nueva entrada.

## Salida sin TTY

Todos los subcomandos de solo lectura y los interruptores funcionan sin TTY. Solo `evolve` lo exige.
