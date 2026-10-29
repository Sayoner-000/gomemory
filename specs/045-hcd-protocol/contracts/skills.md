# Contrato: skills `/hcd`, `$hcd` y secuencia `hcd-*`

**Cubre**: FR-045, FR-045b, FR-050, FR-016a, FR-017a, US5, US8.

## 1. Catálogo

| Skill | Claude Code / OpenCode | Codex | Paso del protocolo | Llama al motor |
|---|---|---|---|---|
| `hcd` | `/hcd <solicitud o id>` | `$hcd …` | Entrada: inicia, retoma o dice el siguiente paso; clasifica unidades sin enfoque y reintenta la memoria pendiente | `start`, `list`, `resume`, `next`, `status`, `focus`, `classify`, `memory sync` |
| `hcd-define` | `/hcd-define` | `$hcd-define` | Intención y definición | `define`, `validate` |
| `hcd-clarify` | `/hcd-clarify` | `$hcd-clarify` | Aclaración y decisiones | `clarify`, `decision` (aceptar o sustituir, siempre con cita) |
| `hcd-approve` | `/hcd-approve` | `$hcd-approve` | Aprobación del alcance | `validate`, `approve` |
| `hcd-slice` | `/hcd-slice` | `$hcd-slice` | Slicing del siguiente incremento | `slice` |
| `hcd-tasks` | `/hcd-tasks` | `$hcd-tasks` | Descomposición atómica | `tasks`, `check` |
| `hcd-implement` | `/hcd-implement` | `$hcd-implement` | Ejecución de tareas autorizadas | `check`, `run`, `report`, `block`, `decision` (solo propuestas) |
| `hcd-verify` | `/hcd-verify` | `$hcd-verify` | Verificación del slice o de la unidad | `verify` |
| `hcd-close` | `/hcd-close` | `$hcd-close` | Cierre, rechazo o cancelación | `verify`, `close` (`--reject`), `cancel` |
| `hcd-exception`, `hcd-playbook`, `hcd-migrate` | — | — | Incrementos posteriores | `exception`, `playbook`, `migrate` |

## 2. Reglas comunes (texto obligatorio en cada `SKILL.md`)

1. **Motor primero.** Al empezar, la skill ejecuta `mem hcd resume <unit> --json --client <agente>` (o `./mem` si `mem` no está en PATH) y actúa solo dentro de `allowed_actions`.
2. **Sin motor no hay avance.** Si ni `mem` ni `./mem` existen, o si el motor responde `internal`, la skill informa la limitación y se detiene. No edita artefactos para simular un estado.
3. **Las mutaciones van por el motor.** La skill redacta el contenido de `definition.md`, `delivery.md` o la evidencia y lo pasa al motor por stdin con `--file -` (por ejemplo, `mem hcd define HCD-001 --file - <<'EOF'`), registrándolo con `define`, `slice`, `tasks` o `report`. Si necesita un archivo, lo escribe fuera del repositorio. Nunca escribe estados, aprobaciones, evidencias ni `verification.md` a mano.
4. **Preguntar antes que suponer** (FR-016a). Ante una ambigüedad que cambia comportamiento, contrato, datos, seguridad o alcance, la skill abre `hcd-clarify`: hace una pregunta a la vez, con 2 a 4 opciones y una recomendación justificada, y registra la respuesta. No elige la interpretación más probable.
5. **Aprobación citada** (FR-017a). Para aprobar, la skill muestra el objeto y su revisión de 12 hexadecimales, pide una elección explícita y pasa la respuesta literal de la persona en `--quote`. Nunca redacta la cita ni aprueba sin una respuesta en la conversación.
6. **Detención localizada** (US8). Ante una ampliación de alcance, una excepción no aprobada, una revisión incompatible, un permiso insuficiente o un resultado incierto, la skill registra un bloqueo o una aclaración sobre la parte afectada (que genera una decisión al resolverse) y continúa solo con trabajo independiente autorizado.
7. **Siguiente paso.** Toda respuesta termina con el `next_action` del motor: nombra la skill siguiente y por qué.
8. **Privacidad.** No pone secretos en artefactos, evidencias ni citas.

## 3. `hcd` (entrada)

- **Sin argumentos**: si hay una unidad activa (`hcd_focus`), hace `resume` de ella. Si no, `list`: ofrece retomar una unidad no terminal o pide la solicitud.
- La skill nunca escribe la unidad activa en un archivo de contexto del agente ni en otro archivo: la marca vive en el motor (`focus`).
- **Con texto libre** («inicia una corrección para …»): propone el tipo y el título, los confirma con la persona, ejecuta `start` y pasa a `hcd-define`.
- **Con id** (`/hcd HCD-046`): `resume` y presentación del paso, los pendientes y la siguiente acción (SC-007).
- Invocar `/hcd` equivale a elegir HCD; no vuelve a preguntar por el enfoque.

## 4. `hcd-implement`

1. `check --task T`. Si responde `atomicity` o `blocked`, la skill no ejecuta y propone dividir la tarea o abrir una aclaración.
2. `run --task T` → `execution_id`.
3. La skill implementa solo el resultado y el alcance de la tarea, y ejecuta su verificación.
4. Escribe la evidencia (procedimiento, entorno, resultado, `subject_ref` del commit o del hash) en un archivo temporal y ejecuta `report --run X --file ev.md`.
5. Si se pierde la respuesta de `report`, repite con el mismo `--operation-id`; nunca ejecuta `run` otra vez sobre la misma tarea.

## 5. Instalación

- Las plantillas canónicas viven en `infrastructure/templates/hcd/<skill>/SKILL.md`, embebidas en el binario.
- `mem install` y `mem update` las instalan con `setup.InstallAgentSkill` para Claude Code, Codex y OpenCode, y con envoltorios `~/.config/opencode/commands/hcd*.md` para OpenCode.
- Una plantilla solo se escribe si el directorio del agente existe y el contenido difiere.
- Un entorno figura como compatible en la documentación solo con evidencia de una invocación real (SC-008).

## 6. Cobertura de comandos

Toda mutación del motor que la persona pueda necesitar tiene una skill que la conduce, para que nunca tenga que escribir comandos de consola (FR-045). La prueba de las plantillas verifica que cada comando mutante del contrato aparece en al menos una `SKILL.md`; `heartbeat` es interno (lo llama el hook) y queda excluido.
