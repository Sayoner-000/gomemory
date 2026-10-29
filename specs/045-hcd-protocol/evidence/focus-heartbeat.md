# Evidencia T104 — escenario 8: unidad activa, heartbeat y recuperación entre agentes (SC-011)

Estado: **ejecutada** (2026-10-28) en el proyecto aislado de T103, alternando OpenCode, Claude Code y Codex sobre la misma unidad `HCD-001`.

| Paso | Agente | Resultado |
|---|---|---|
| 1. Unidad activa | OpenCode creó HCD-001 → `mem hcd focus --json` devuelve `UnitID: HCD-001`, `Client: opencode` | Cubierto |
| 1b. Retomar sin argumentos | Claude Code, `/hcd` sin argumentos: presentó HCD-001, paso `approval`, rev `3870249e0ce8`, decisiones D-01 y D-02 y siguiente acción | Cubierto |
| 2. Desvío | Claude Code añadió una línea a `go.mod` sin `/hcd-implement` → `status` muestra `files_without_execution`; `revision.state` sigue en 4 (sin decisiones nuevas); `mem search checkpoint` sigue mostrando los checkpoints normales | Cubierto |
| 3. Contexto de arranque en otro agente | Codex copió literalmente la sección `## HCD activo` de su contexto de arranque: unidad, paso, siguiente acción y desvío | Cubierto |
| 3b. `$hcd` en Codex | El shell de Codex usa el `mem` global (2.40.3, sin `hcd`) y la skill se detuvo («Sin motor no hay avance»). Es el requisito de entorno de T102; con `mem update` a una versión con HCD desaparece | Limitación de entorno |
| 4. Nada fuera de `hcd/` | `git status`: solo `go.mod` (cambio de prueba, revertido), `opencode.json` y skills copiadas por el orquestador, y `.memory/`. Ningún archivo de contexto de agente ni `.specify/feature.json` | Cubierto |
| 5. Cierre libera la unidad | OpenCode cerró HCD-001 → `focus` vacío y `mem context` ya no contiene «HCD activo» | Cubierto |

Los hooks de fin de turno (heartbeat) se ejecutaron con el binario de la rama porque el `PATH` del proceso del agente lo tenía primero.

## Respuestas literales

### Claude Code — `/hcd`

**HCD-001 · fix: «Evitar pánico en Resume de store.go con un ID inexistente»**

| Campo | Valor |
|---|---|
| Etapa | `approval` (estado `draft`) |
| Revisión de la definición | `3870249e0ce8` |
| Validación | superada |
| Aclaraciones abiertas | ninguna |
| Bloqueos y comprobaciones pendientes | ninguno |
| Memoria | sincronizada |

**Decisiones registradas**
- **D-01:** se usará el enfoque HCD.
- **D-02 (CL-01):** cuando el ID no existe, `Resume` devuelve un resultado vacío sin error y mantiene su firma `Resume(id string) string`.

La definición está completa y validada. No se ha aprobado nada; eso requiere una respuesta tuya.

**Siguiente paso:** `/hcd-approve`, porque la definición está validada y espera aprobación. Ahí te mostraré la definición con su revisión `3870249e0ce8` y te pediré una elección explícita. Tu respuesta literal quedará registrada como cita.

### Claude Code — cambio fuera de `/hcd-implement`

Añadí `// prueba de desvío` al final de `go.mod`, ahora en la línea 4. No toqué nada más: no ejecuté ningún comando de `mem hcd` ni ninguna otra acción.

Este cambio se hizo fuera del protocolo. HCD-001 sigue en `approval` sin aprobar y no tiene ninguna ejecución concedida, así que el heartbeat debería marcarlo como desvío en su próxima comprobación. Cuando termine la prueba, conviene revertir la línea para que `go.mod` quede limpio.

**Siguiente paso:** `/hcd`. Hará el `resume` de HCD-001 y mostrará cómo informa el motor del desvío en `go.mod`. Después sigue `/hcd-approve`, porque la definición `3870249e0ce8` está validada y espera tu aprobación.

### Codex — contexto de arranque y `$hcd`

## HCD activo

- **HCD-001 · fix · «Evitar pánico en Resume de store.go con un ID inexistente»** — paso `approval`, estado `draft`, rev `3870249e0ce8`
- Siguiente: `/hcd-approve` — la definición está validada y espera aprobación
- Desvío: files_without_execution
- Recuperar todo: `/hcd` o `$hcd`

La unidad activa es **HCD-001** y estamos en el paso **approval**, con estado `draft`.

Intenté ejecutar `$hcd` sin argumentos mediante `mem hcd resume --json --client codex`, pero el CLI respondió `comando desconocido 'hcd'`. Tampoco hay un `./mem` ejecutable en el repositorio, así que no pude consultar el motor HCD ni avanzar. Según el contexto recuperado, el siguiente paso es `/hcd-approve`.

