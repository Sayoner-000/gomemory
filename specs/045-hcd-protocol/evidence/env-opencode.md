# Evidencia T103 — escenario 7 con OpenCode (SC-008)

Estado: **ejecutada** (2026-10-28). OpenCode 2.1.4 (modelo `openai/gpt-6.1-sol`) recorrió `HCD-001` (`fix`) de `start` a `completed` con las skills `hcd-*`.

## Montaje

- `opencode run --standalone --format json -m openai/gpt-6.1-sol --session ses_e7ad12fd0ffe3yW6Ibabq84udb`, 6 turnos.
- Proyecto aislado con el mismo bug que T074. Skills en `.opencode/skills/` y comandos en `.opencode/commands/`. Binario de la rama.
- `opencode.json` del proyecto con permisos explícitos: shell solo para `mem`, `go test`, `go vet`, `ls` y `cat`; edición permitida; `external_directory` y `webfetch` denegados. Sin `--auto`.
- La persona la interpretó el orquestador (otra sesión de Claude Code), incluidas las citas.

## Resultado

| Paso | Resultado |
|---|---|
| `start` | `fix`, implementador «Jose» |
| Aclaración | CL-01 bloqueante sobre el contrato de `Resume`; la respuesta quedó como D-02 |
| Definición | Registrada por stdin (`define --file -`), validada sin pendientes |
| Aprobación | Cita registrada (`approve --quote`) con la revisión `3870249e0ce8` |
| Entrega | `slice` y `tasks` por stdin; `check` admitió T-001; `run`; el agente reprodujo el fallo con una prueba antes del arreglo; `go test` PASS |
| Evidencia | Sujeto `tree:13ad853092a5` con `mem hcd subject`; `report` por stdin |
| `verify` / `close` | PASS; «cierro HCD-001 3870249e0ce8» → `completed`; foco liberado |

## Hallazgos del entorno

1. **Modelo por defecto no disponible.** El modelo predeterminado de la cuenta respondía `model_not_supported`; con `-m openai/gpt-6.1-sol` funcionó. No es un problema de gomemory.
2. **`opencode run` se queda esperando con algunos mensajes.** El mensaje `/hcd inicia …` y otro con comillas invertidas y «…» no produjeron ningún evento en más de 10 minutos; el mismo contenido en texto plano, o «Carga la skill `hcd` y …» (lo que hace el comando `.opencode/commands/hcd.md`), funcionó. No se probó la TUI interactiva. El id de `--session` debe empezar por `ses`; con otro valor el proceso también se queda esperando.
3. **Temporales fuera del proyecto.** Con `external_directory` denegado, el agente no pudo escribir el borrador en el directorio temporal del sistema (turno 3). Se añadió `--file -` al motor y las skills pasaron a usar heredoc (turno 4 en adelante).
4. **La cita es el mensaje completo.** En el turno 5 el agente pasó a `--quote` todo el mensaje de la persona, no solo «apruebo …». Es lo que exige la regla de cita literal.

## Transcripción (respuesta final y herramientas de cada turno)

### Turno 1

> Persona: Carga la skill `hcd` y sigue sus instrucciones con estos argumentos: inicia una corrección: Resume en store.go entra en pánico con un id inexistente

Herramientas:

- `skill · completed · hcd`
- `shell · error · if command -v mem >/dev/null 2>&1; then mem hcd resume --json --client opencode; elif [ -x ./mem ]; then ./mem`

Skill `hcd` cargada. Intenté consultar el motor, pero se denegó el permiso de ejecución (`shell`). No creé ninguna unidad ni modifiqué archivos.

Propuesta pendiente de confirmación:
- **Tipo:** `fix`
- **Título:** Evitar pánico en `Resume` de `store.go` con un ID inexistente.

¿Confirmas el tipo y el título?

**Siguiente paso:** reintentar `hcd` con permiso para consultar el motor; aún no puedo obtener su `next_action` ni avanzar a `hcd-define`.

### Turno 2

> Persona: Si, confirmo el tipo fix con ese titulo. Inicia la unidad con mem hcd start y continua con la definicion. El implementador soy yo, Jose.

Herramientas:

- `shell · completed · mem hcd resume --json --client opencode`
- `shell · completed · mem hcd start --kind fix --title "Evitar pánico en Resume de store.go con un ID inexistente" --implementer "Jo`
- `skill · completed · hcd-define`
- `shell · completed · mem hcd resume HCD-001 --json --client opencode`
- `execute · completed · `
- `execute · completed · `
- `shell · completed · mem hcd show HCD-001 --json --client opencode`
- `skill · completed · hcd-clarify`
- `shell · completed · mem hcd resume HCD-001 --json --client opencode`
- `shell · completed · mem hcd clarify HCD-001 --open --target "Resultado esperado" --question "¿Qué debe hacer Resume cuando el ID n`
- `shell · completed · mem hcd clarify HCD-001 --open --target "Resultado esperado" --question "¿Qué debe hacer Resume cuando el ID n`
- `shell · completed · mem session end -s "Goal: iniciar corrección HCD de Resume. Discoveries: resultado esperado para ID inexistent`

Creada **HCD-001**, tipo `fix`, con el título confirmado e implementador **Jose**.

Abrí la aclaración bloqueante **CL-01**: ¿qué debe hacer `Resume` cuando el ID no existe?

1. **Devolver un error sin pánico** — recomendado: informa del fallo sin crear datos.
2. Devolver un resultado vacío sin error.
3. Crear una unidad nueva.

**Siguiente paso del motor:** `hcd-define` → `validate`; la definición sigue pendiente y necesita esta respuesta.

### Turno 3

> Persona: Opcion 2: devolver un resultado vacio sin error, conservando la firma. …

Herramientas:

- `shell · completed · mem hcd resume HCD-001 --json --client opencode`
- `shell · completed · mem hcd clarify HCD-001 --answer CL-01 --choice "Opcion 2: devolver un resultado vacio sin error, conservando `
- `shell · completed · mem hcd define --help`
- `read · completed · .`
- `read · completed · hcd`
- `execute · completed · `
- `execute · completed · `
- `read · completed · store.go`
- `read · completed · store_test.go`
- `glob · completed · `
- `read · completed · hcd/features/001-evitar-p-nico-en-resume-de-store-go-con-un-id-inexistente/definition.md`
- `read · completed · go.mod`
- `patch · error · `
- `shell · completed · mem save -t "HCD-001 causa del pánico de Resume" -y bug "Inspección directa: store.go:7-9 declara Resume(id st`
- `shell · completed · mem hcd validate HCD-001 --json --client opencode`
- `shell · completed · mem session end -s "Goal: registrar CL-01 y completar definición HCD-001. Discoveries: Resume recorta s[:1] so`

Respuesta registrada como **D-02 aceptada**: devolver `""` sin error y conservar la firma.

**Bloqueo:** se denegó crear el archivo temporal requerido. La definición sigue incompleta; `validate` confirma pendientes en la revisión `109b22aa8e77`. No corresponde solicitar aprobación todavía.

¿Autorizas la escritura temporal fuera del repositorio?

**Siguiente paso del motor:** `hcd-define` → `validate`, para completar la definición y después mostrarte la nueva revisión para aprobación.

### Turno 4

> Persona: Actualice el motor y las skills: … --file - y lee de stdin con un heredoc. …

Herramientas:

- `skill · completed · hcd-define`
- `shell · completed · mem hcd resume HCD-001 --json --client opencode`
- `shell · completed · mem hcd define HCD-001 --file - --expected-revision 3 --operation-id hcd001-define-stdin-01 --json --client op`
- `shell · completed · mem hcd validate HCD-001 --json --client opencode`
- `shell · completed · mem save -t "Artefactos HCD por stdin" -y learning "El motor y la skill hcd-define actualizados permiten regis`
- `shell · completed · mem session end -s "Goal: completar y validar definición HCD-001 por stdin. Discoveries: skill y motor aceptan`

Definición de **HCD-001** registrada por stdin y **validada sin pendientes ni bloqueos**.

**Revisión: `3870249e0ce8`**
- ID inexistente: devolver `""` sin pánico, conservando la firma.
- Mantener `Resume("s1") == "p"`.
- Añadir prueba de regresión.

¿Apruebas la definición de **HCD-001**, revisión **`3870249e0ce8`**, o solicitas cambios?

**Siguiente paso:** `hcd-approve`, porque la definición validada espera tu aprobación explícita.

### Turno 5

> Persona: apruebo 3870249e0ce8. Continua: un slice y una tarea por stdin, … verifica. Luego pideme el cierre mostrando la revision.

Herramientas:

- `skill · completed · hcd-approve`
- `shell · completed · mem hcd resume HCD-001 --json --client opencode`
- `shell · completed · mem hcd validate HCD-001 --json --client opencode`
- `shell · completed · mem hcd approve HCD-001 --revision 3870249e0ce8 --quote "apruebo 3870249e0ce8. Continua: un slice y una tarea `
- `skill · completed · hcd-slice`
- `shell · completed · mem hcd resume HCD-001 --json --client opencode`
- `shell · completed · mem hcd slice HCD-001 --file - --expected-revision 5 --operation-id hcd001-slice-01 --json --client opencode <`
- `skill · completed · hcd-tasks`
- `shell · completed · mem hcd resume HCD-001 --json --client opencode`
- `shell · completed · mem hcd tasks HCD-001 --slice VS-01 --file - --expected-revision 6 --operation-id hcd001-tasks-01 --json --cli`
- `shell · completed · mem hcd check HCD-001 --task T-001 --json --client opencode`
- `skill · completed · hcd-implement`
- `shell · completed · mem hcd resume HCD-001 --json --client opencode`
- `shell · completed · mem hcd run HCD-001 --task T-001 --expected-revision 7 --operation-id hcd001-run-01 --json --client opencode`
- `shell · error · git status --short`
- `read · completed · store.go`
- `read · completed · store_test.go`
- `patch · completed · `
- `shell · completed · go test ./...`
- `patch · completed · `
- `shell · completed · go test ./...`
- `shell · completed · mem save -t "HCD-001 Resume seguro para ID inexistente" -y bug "Causa raíz: Sessions[id] devuelve cadena vacía`
- `shell · error · git diff --check`
- `shell · error · go version`
- `shell · completed · mem hcd subject HCD-001 --files store.go,store_test.go --json --client opencode`
- `shell · completed · mem hcd report HCD-001 --run X-01a1854f-0e21-7f26-a2f2-cdf79c194f81 --file - --expected-revision 8 --operation`
- `skill · completed · hcd-verify`
- `shell · completed · mem hcd resume HCD-001 --json --client opencode`
- `shell · completed · mem hcd verify HCD-001 --slice VS-01 --expected-revision 9 --operation-id hcd001-verify-01 --json --client ope`
- `skill · completed · hcd-close`
- `shell · completed · mem hcd resume HCD-001 --json --client opencode`
- `shell · completed · mem session end -s "Goal: aprobar HCD-001, preparar un slice y una tarea, ejecutar e implementar, registrar ev`

**Implementado y verificado: PASS.**

- Un slice y una tarea registrados por stdin; `run` ejecutado.
- `Resume` devuelve `""` sin pánico, conservando la firma.
- La prueba reprodujo el fallo antes del arreglo; después, `go test ./...` pasó.
- Sujeto: `tree:13ad853092a5`; report registrado por stdin.
- C-01 y C-02 verificados.

Límites registrados: inspección Git denegada y aviso `files_without_execution`.

**Revisiones:** definición `3870249e0ce8`, entrega `82b5d76736fe`, verificación `f4b75f401389`.

¿Aceptas el cierre de **HCD-001**, revisión **`3870249e0ce8`**, con esta entrega?

**Siguiente:** `hcd-close`; falta tu aceptación explícita.

### Turno 6

> Persona: cierro HCD-001 3870249e0ce8

Herramientas:

- `shell · completed · mem hcd resume HCD-001 --json --client opencode`
- `shell · completed · mem hcd verify HCD-001 --expected-revision 10 --operation-id hcd001-verify-close-01 --json --client opencode`
- `shell · completed · mem hcd close HCD-001 --revision 3870249e0ce8 --quote "cierro HCD-001 3870249e0ce8" --expected-revision 11 --o`
- `shell · completed · mem session end -s "Goal: cerrar HCD-001 con aceptación literal de Jose. Accomplished: verify vigente PASS, C-`

**HCD-001 cerrada** en estado `completed`, revisión `3870249e0ce8`, con tu aceptación literal registrada. Verificación **PASS** y memoria sincronizada.

Siguiente acción del motor: `hcd` → `list`, para retomar otra unidad o iniciar una nueva.

