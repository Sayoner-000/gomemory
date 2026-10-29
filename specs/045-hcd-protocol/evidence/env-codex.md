# Evidencia T102 — escenario 7 con `$hcd` en Codex (SC-008)

Estado: **ejecutada** (2026-10-28). Codex CLI 0.164.0 recorrió `HCD-001` (`fix`) de `start` a `completed` con las skills `hcd-*`.

## Montaje

- `codex exec` y `codex exec resume <sesión>` (sesión `01a18509-…`), sandbox `workspace-write`. Sin `--dangerously-bypass-approvals-and-sandbox`.
- Proyecto aislado con el mismo bug que T074. Skills `hcd-*` en `.agents/skills/` del proyecto; binario de la rama.
- La persona la interpretó el orquestador (otra sesión de Claude Code), incluidas las citas.

## Resultado

| Paso | Resultado |
|---|---|
| Propuesta y `start` | `fix`, implementador «Jose» registrado en el estado de control (confirma la corrección de `define`) |
| Aclaración | El agente abrió CL-01 sobre el contrato de `Resume` y esperó la respuesta antes de validar |
| Aprobación citada | «apruebo 6a7d7bdb6ebd» |
| `slice`, `tasks`, `run`, `report` | VS-01 y T-001 registrados al primer intento con las plantillas de las skills; `go test` PASS; sujeto `tree:9c2f0160121b` calculado con `mem hcd subject` |
| `verify` | PASS, C-01..C-03 verificados, evidencia vigente |
| `close` | «cierro HCD-001 6a7d7bdb6ebd» → `completed`; no existe `specs/` |
| Desvíos | ninguno |

## Requisitos de entorno que encontró la ejecución

Los dos van a la guía de uso; ninguno es un defecto del motor:

1. **Sandbox de Codex.** Con `workspace-write`, Codex no puede escribir en `~/.local/share/gomemory/`, donde vive la base de cada proyecto, y `mem hcd` falla (turno 1). Hay que añadir ese directorio como raíz escribible: `codex --add-dir ~/.local/share/gomemory` o, en `~/.codex/config.toml`, `[sandbox_workspace_write] writable_roots = ["~/.local/share/gomemory"]` (ruta absoluta).
2. **Versión de `mem`.** El shell de Codex usa el `PATH` del perfil, no el del proceso que lo lanza. Con un `mem` sin `hcd` en el `PATH`, la skill se detiene ("Sin motor no hay avance"), que es lo correcto (turno 2). Además, el hook global de gomemory retira un `./mem` local al arrancar; el agente usó entonces el binario de la rama por ruta. En una instalación normal (`mem update` a una versión con HCD) el problema no existe.

## Transcripción (mensaje final de cada turno, literal)

### Turno 1

> Persona: $hcd inicia una corrección: Resume en store.go entra en pánico con un id inexistente

El inicio quedó bloqueado: `mem hcd resume` falló por permisos al preparar su directorio en `~/.local/share/gomemory/projects/`. No se creó la unidad ni se modificó código.

La [skill hcd](.agents/skills/hcd/SKILL.md) exige actuar «solo dentro de `allowed_actions`»; el fallo impidió obtenerlas.

Siguiente paso: restablecer el acceso y reintentar `$hcd`. El motor no devolvió `next_action`.

### Turno 2

> Persona: Reintenta: ya tienes acceso de escritura a ~/.local/share/gomemory. … sí, inicia la unidad.

El acceso ya funciona, pero `mem 2.40.3` no reconoce `hcd` y no hay un `./mem` ejecutable. No se creó la unidad.

Quedó guardada tu autorización para iniciar:
- Tipo: `fix`
- Título: «Corregir pánico de Resume en store.go con un ID inexistente»

La [skill hcd](.agents/skills/hcd/SKILL.md) exige «Sin motor no hay avance».

Siguiente paso: disponer del motor con soporte HCD y reintentar `$hcd`; no se obtuvo `next_action`.

### Turno 3

> Persona: Hay un ./mem con soporte HCD en la raíz … Sí, inicia la unidad fix … el implementador soy yo, Jose.

Creada `HCD-001`, tipo `fix`, con Jose como implementador. Registré la definición y reproduje el pánico: `s[:1]` falla sobre la cadena vacía del ID ausente.

Para completar la definición, ¿qué debe devolver `Resume` ante un ID inexistente o una sesión vacía?

- **A — Recomendada:** `""`, conservando la firma actual.
- **B:** un error, cambiando la firma a `(string, error)`.

La [skill hcd-clarify](.agents/skills/hcd-clarify/SKILL.md) exige aclarar las decisiones que cambian el contrato. Siguiente paso del motor: `hcd-clarify`, para resolver `CL-01`.

### Turno 4

> Persona: A.

Registrada **A**. La definición de `HCD-001` está validada, sin pendientes, en la revisión **`6a7d7bdb6ebd`**.

[Consultar definición completa](/private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/42314baa-f27e-4c26-9cf3-9a6ed957918e/scratchpad/hcdlab/codex/hcd/features/001-corregir-p-nico-de-resume-en-store-go-con-un-id-inexistente/definition.md)

- ID inexistente o sesión vacía → `""`, sin pánico.
- Se conserva `Resume(id string) string`.
- Se conserva `Resume("s1") == "p"`.
- Jose implementará la corrección y las pruebas de regresión.

¿Apruebas la definición **`6a7d7bdb6ebd`** o solicitas cambios?

La [skill hcd-approve](/private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/42314baa-f27e-4c26-9cf3-9a6ed957918e/scratchpad/hcdlab/codex/.agents/skills/hcd-approve/SKILL.md) exige «una elección explícita» sobre la revisión final. Siguiente paso del motor: **`hcd-approve`**, porque la definición espera aprobación.

### Turno 5

> Persona: apruebo 6a7d7bdb6ebd. Continúa: un slice, una tarea, implementa con run, go test, subject, report; después verifica.

**PASS.** Completé `VS-01` con una tarea `T-001`, ejecutada mediante `run`.

- `Resume` devuelve `""` para IDs inexistentes o sesiones vacías.
- `go test ./... -count=1 -v` pasó.
- Sujeto: `tree:9c2f0160121b`.
- Report registrado; C-01, C-02 y C-03 verificados con evidencia vigente, sin pendientes.

Siguiente paso del motor: **`hcd-close`**, porque falta la aceptación citada del cierre. La unidad permanece abierta.

### Turno 6

> Persona: cierro HCD-001 6a7d7bdb6ebd. Después muestra el estado final, el implementador registrado y confirma que no existe specs/.

**HCD-001 cerrada correctamente.**

- Estado final: `completed`.
- Revisión aceptada: `6a7d7bdb6ebd`.
- Implementador registrado: **Jose**.
- Verificación: **PASS**, sin pendientes ni bloqueos.
- Confirmado: **no existe `specs/` en la raíz del proyecto**.

El motor mantiene `hcd-close` como `next_action` por la etapa de cierre; la unidad ya está completada y solo permite consultas.

