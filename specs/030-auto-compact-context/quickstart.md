# Quickstart: validar la compactación sin pérdida de memoria

**Feature**: `030-auto-compact-context`

Guía de validación de extremo a extremo. Los tests unitarios y de contrato son
necesarios pero no bastan (regla de trabajo 2): cada historia se da por
cumplida solo después de verla en el cliente en ejecución.

## Prerrequisitos

```bash
cd /Users/josegomezj/home/rcw/gomemory
go build -o /tmp/mem-030 ./infrastructure/
/tmp/mem-030 version
```

Para las pruebas en vivo, instalar el binario con `scripts/install.sh` (no con
`cp` directo: macOS mata con SIGKILL un binario sobrescrito en el mismo inodo),
refrescar la integración de cada cliente (`mem update` o la TUI) y confirmar
con `mem doctor` que los hooks nuevos quedaron registrados.

Los ajustes del proyecto viven en `.memory/settings.json`. Para las pruebas se
editan con `jq`; la persona los cambia desde la TUI (Configuración).

Clientes de referencia: claude 2.1.268, codex 0.154.0, opencode 1.18.30.

---

## Q0. Confirmar las capacidades contra el cliente en ejecución

La tabla R1 de [research.md](research.md) sale de documentación y tipos. Antes
de implementar cada integración, confirmar su celda:

| Celda | Cómo se confirma | Resultado esperado |
|-------|------------------|--------------------|
| claude C6 | Registrar un `PostCompact` temporal que vuelque stdin a un archivo; compactar a mano. | stdin trae `compact_summary` no vacío. |
| codex C3 | `SubagentStop` temporal que vuelque stdin y devuelva `{}`. | stdin trae `last_assistant_message`. |
| opencode C3 | Registrar en el plugin el `input.tool` de `tool.execute.after` tras delegar una tarea. | El identificador es `task` y `output.output` trae el texto final. |
| opencode C6 | Tras `session.compacted`, listar los mensajes de la sesión. | Existe un mensaje con `summary: true` y partes de texto. |

Si una celda no se confirma, se declara como no disponible con su motivo en
`KnownAgents` (INV-C1) y la historia usa el respaldo. No se fuerza el canal.

## Q1. US1: memoria de la sesión alrededor de la compactación

```bash
cd "$(mktemp -d)" && git init -q
/tmp/mem-030 session start
/tmp/mem-030 save -t "Decisión A" -y decision "Usar X porque Y"
/tmp/mem-030 save -t "Hallazgo B" -y discovery "Z ocurre cuando W"
/tmp/mem-030 hook compaction-context
/tmp/mem-030 hook post-compact
```

**Esperado**:
- `compaction-context` muestra «## Memoria de esta sesión» con las dos
  entradas, cada una con `get_memory <id>`, y termina con la orden de
  persistir.
- `post-compact` muestra, en orden: los pasos de recuperación, la memoria de la
  sesión y el contexto de proyecto en modo índice.
- Con una sesión nueva sin memorias: aparece la nota «Esta sesión aún no guardó
  memorias propias».

## Q2. SC-002: tamaño del texto posterior a la compactación

Sobre un proyecto con 100 memorias o más (por ejemplo, este repositorio):

```bash
mem hook post-compact | wc -c          # versión instalada anterior (línea base)
/tmp/mem-030 hook post-compact | wc -c  # versión nueva
```

**Esperado**: la versión nueva ocupa como mucho el 50 % de la línea base y no
supera `Settings.Budget`.

## Q3. US2: persistencia del resumen y sesión abierta

```bash
echo '{"compact_summary":"Resumen de prueba"}' | /tmp/mem-030 hook compact-summary
echo '{"compact_summary":"Resumen de prueba"}' | /tmp/mem-030 hook compact-summary
/tmp/mem-030 session list | head -3
/tmp/mem-030 save -t "Después" -y learning "Tras compactar"
```

**Esperado**: una sola sesión activa, con `summary = "Resumen de prueba"`, sin
`ended_at`. La memoria «Después» lleva el `session_id` de esa misma sesión
(FR-023).

**En vivo (claude)**: compactar a mano una sesión real. Verificar que el
resumen de la sesión queda guardado sin que el agente llame a nada (C6).
**En vivo (codex)**: compactar y verificar que el agente llama a
`save_session_summary` como primera acción (respaldo, sin C6).

## Q4. US3: captura pasiva

```bash
printf '%s' '{"last_assistant_message":"Listo.\n\n## Aprendizajes clave\n1. El caché se invalida al escribir la configuración\n2. corto\n3. Los hooks de Codex exigen JSON en SubagentStop\n"}' \
  | /tmp/mem-030 hook subagent-stop --emit=json
# repetir el mismo comando
/tmp/mem-030 list -n 5
```

**Esperado**: la primera ejecución imprime `{}` y crea 2 memorias `learning`
(el ítem «corto» se descarta). La segunda no crea ninguna. Un mensaje sin
sección de aprendizajes no crea ninguna memoria.

## Q5. US4: aviso al agente

```bash
s=.memory/settings.json; [ -f $s ] || echo '{}' > $s
jq '.compact_threshold=10' $s > $s.tmp && mv $s.tmp $s
echo '{}' | /tmp/mem-030 hook turn-end                         # opción apagada
jq '.compact_agent_notice=true' $s > $s.tmp && mv $s.tmp $s
rm -f .memory/.last-compact-nudge
echo '{}' | /tmp/mem-030 hook turn-end                         # claude
rm -f .memory/.last-compact-nudge
echo '{}' | /tmp/mem-030 hook turn-end --emit=json; ls .memory/.pending-agent-notice
```

**Esperado**:
- Con la opción apagada: salida idéntica a la versión anterior (solo
  `systemMessage`).
- `claude`: un solo objeto con `systemMessage` y
  `hookSpecificOutput.additionalContext`; sin `decision`.
- `json`: solo `systemMessage`, y el archivo `.pending-agent-notice` existe. El
  siguiente `user-prompt-submit --emit=json` lo consume y el archivo desaparece.

La huella debe superar el umbral para que el aviso dispare. Antes de cada
`turn-end`, emitir contexto con `/tmp/mem-030 context > /dev/null` o bajar el
umbral lo suficiente.

## Q6. Agnosticismo y diagnóstico

```bash
go test ./tests/contract/... -run 'Compaction|Texts'
/tmp/mem-030 doctor
```

**Esperado**: el test de términos prohibidos pasa (0 nombres de agentes,
clientes ni comandos de cliente) y el diagnóstico lista C1–C6 por agente, con
el motivo de las que no están disponibles.

## Q7. Regresión

```bash
go test ./...
golangci-lint run
```

Todo en verde y sin supresiones nuevas de errcheck.

---

## Evidencia

Cada escenario deja aquí su resultado al ejecutarse (tasks.md: T032, T039,
T050, T060, T066): fecha, versión del cliente, comando o acción, y resultado
observado.

| Escenario | Fecha | Cliente / versión | Resultado |
|-----------|-------|-------------------|-----------|
| Q0 | — | — | pendiente (ver nota en tasks.md, Fase 1) |
| Q1 | 2026-09-10 | CLI, este repo (165+ memorias) | `hook compaction-context` y `hook post-compact` muestran el orden correcto (recuperación → sesión → proyecto) y el puntero `get_memory <id>` por entrada. Ver Q2 para el detalle de tamaño. |
| Q2 | 2026-09-10 | CLI, este repo (165+ memorias, `.memory/settings.json` real) | Medido `mem hook post-compact` (binario instalado, pre-feature) = 17847 caracteres vs el nuevo build = 10787 caracteres → **60 %**, NO el ≤50 % de SC-002. Causa raíz: las secciones «Sinapsis» y «Sesiones Recientes» no se acotan en modo índice por diseño deliberado de la feature 020 (FR-032: "el resto de la estructura ... queda intacta en ambos modos"). El ahorro real viene de que el cuerpo de cada memoria colapsa a un puntero; lo que no colapsa son sinapsis/resúmenes de sesión, que en este proyecto son voluminosos. **SC-002 queda parcialmente cumplido**: hay reducción real y medible, pero no llega al 50% en proyectos con muchas sinapsis/sesiones. Pendiente de decisión: ¿acotar también esas dos secciones en el contexto post-compactación (fuera del alcance de FR-032, que rige get_context) o ajustar el techo de SC-002? No se tocó FR-032 en esta implementación por ser una invariante de otra feature. |
| Q3 | — | — | pendiente |
| Q4 | — | — | pendiente |
| Q5 | — | — | pendiente |
| Q6 | — | — | pendiente |
| Q7 | — | — | pendiente |
