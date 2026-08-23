# Tareas — Matriz de canales como fuente única

**Feature**: `specs/022-agent-channel-matrix` · **Plan**: [plan.md](./plan.md)

TDD obligatorio (constitución, principio III): la prueba entra en rojo antes de la
implementación. Ejecución estrictamente secuencial: sin subagentes, sin paralelismo.

## Fase 1 — Dominio: la declaración única

- [X] **T001** Prueba en rojo de INV-1: una celda sin `Path` y sin `NotApplicableReason` es inválida.
- [X] **T002** Prueba en rojo de INV-2 y C6: toda ruta es relativa, sin `..` ni separador inicial.
- [X] **T003** Prueba en rojo de INV-3 y C2: todo `Agent` de la matriz existe en `KnownAgents`.
- [X] **T004** Crear `domain/channel_matrix.go` con `MatrixCell`, `LifecycleActivity` y sus constantes de alcance.
- [X] **T005** Poblar la matriz con las celdas de ámbito de proyecto de los agentes con canales propios.
- [X] **T006** Poblar las celdas de ámbito de usuario.
- [X] **T007** Poblar las celdas heredadas (`Legacy`) que hoy solo se retiran.
- [X] **T008** Declarar motivo en las celdas que no aplican, en vez de omitir la fila.
- [X] **T009** Métodos de consulta: celdas por actividad, por ámbito y por agente.

## Fase 2 — Verificación: cerrar la fuente

- [X] **T010** Crear `tests/contract/channel_matrix_test.go` con C1 (completitud).
- [X] **T011** Añadir C2 (integridad referencial en ambos sentidos).
- [X] **T012** Añadir C4 (ninguna actividad de proyecto referencia celdas de usuario).
- [X] **T013** Añadir C6 (rutas relativas).
- [X] **T014** Verificar que C1 falla de verdad: introducir una celda incompleta, comprobar el mensaje, retirarla.

## Fase 3 — Aislamiento del entorno (va antes de tocar consumidores)

- [X] **T015** Aislar el entorno de la persona en las 5 funciones de `tests/integration/uninstall_integration_test.go`.
- [X] **T016** Aislar el entorno en `TestCmdUninstallAcceptsYesFlagInAnyPosition` de `tests/contract/maintenance_cli_test.go`.
- [X] **T017** Añadir C5: inventario del entorno de la persona antes y después de ejercer una actividad de proyecto.
- [X] **T018** Ejecutar la batería completa y comprobar por inventario que el entorno real queda intacto.

## Fase 4 — Consumidores con defecto verificado

- [X] **T019** Prueba en rojo de C3: simetría entre lo que instala y lo que desinstala.
- [X] **T020** Derivar de la matriz la lista de configuraciones que retira la desinstalación.
- [X] **T021** Derivar de la matriz los artefactos heredados que retira la limpieza.
- [X] **T022** Prueba en rojo de C8: pedir un solo agente no produce artefactos de otro.
- [X] **T023** Derivar del alcance y de la selección el registro de ámbito global; retirar el filtro fijo a un agente.
- [X] **T024** Informar la celda de ámbito de usuario relacionada en vez de tocarla (FR-014).

## Fase 5 — Atar lo no migrado

- [X] **T025** Añadir C7 para los envoltorios del método de planificación.
- [X] **T026** Añadir C7 para los envoltorios de la constitución.
- [X] **T027** Añadir C7 para los destinos de ámbito global.
- [X] **T028** Añadir C7 para los archivos de instrucciones que inspecciona el diagnóstico.

## Fase 6 — Diagnóstico derivado

- [X] **T029** Prueba en rojo: el diagnóstico enumera exactamente las celdas de la matriz.
- [X] **T030** *(alcance reducido, declarado)* El inspector NO se deriva por completo: hacerlo añadiría al informe los canales de registro de servidor y de permisos, llevándolo de 17 a unas 30 filas. Ese rediseño pertenece a la especificación 024. En su lugar, el contrato `TestC7_ElDiagnosticoNoInventaCanales` garantiza que el inspector no pueda reportar un canal que la matriz no declare.

## Fase 7 — Validación contra el binario

- [X] **T031** Ejecutar `go build ./... && go test ./...` en verde.
- [X] **T032** Ejecutar los pasos 4, 5 y 6 de `quickstart.md` contra el binario compilado.
- [X] **T033** Registrar el resultado en memoria y cerrar la sesión.


## Cierre

**Estado**: 33/33. Batería completa en verde (13 paquetes) y validación contra el binario
compilado, no solo contra los tests.

### Lo que encontró la validación contra el binario y los tests no

Los cuatro envoltorios nativos sobrevivían a toda desinstalación. La verificación de dominio
(C3) estaba en verde porque la matriz declaraba que la desinstalación los cubría; el binario
mostró que no los tocaba. Es el caso literal de la regla 2 del proyecto: el contrato describía
la intención y nadie comprobaba el efecto. Cerrado con `removeNativeWrappers`, que retira
archivos y nunca directorios — `.claude/skills` y `.opencode/commands` alojan también
habilidades de otras herramientas y comandos de la persona.

### Límite declarado, fuera de alcance

`mem install` añade `.memory/` y `mem` al `.gitignore` del proyecto y la desinstalación no
revierte esas dos líneas. Queda fuera de esta feature por una razón de alcance, no por olvido:
el `.gitignore` no es un artefacto de agente y no ocupa celda en la matriz. Tras desinstalar,
ambas rutas dejan de existir, así que las líneas quedan inertes.

### Verificación ejecutada

| Comprobación | Resultado |
|---|---|
| `go vet ./...` y `go test ./...` | 13 paquetes en verde |
| C1 falla ante una celda sin declarar | Verificado: la nombra por agente, canal y ámbito |
| C5 falla ante la regresión que borró un complemento real | Verificado reintroduciendo el defecto |
| C7 falla ante una tabla que se separa de la matriz | Verificado moviendo la ruta de un envoltorio |
| Instalar y desinstalar contra el binario | Cero contenido de gomemory en lo que sobrevive |
| Selección de agentes | Pedir un agente produce un archivo, de ese agente |
| El entorno real tras la batería | 6.795 archivos, intactos |
