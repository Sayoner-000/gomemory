# Tareas: Operación agnóstica de gomemory

**Entrada**: Artefactos de diseño de `/specs/025-gomemory-agnostic-operation/`  
**Prerrequisitos**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md),
[data-model.md](data-model.md), [contracts/](contracts/) y [quickstart.md](quickstart.md)

**Pruebas**: Obligatorias. La especificación define escenarios comprobables y la constitución exige
TDD; cada tarea de prueba se escribe y observa en fallo antes de la implementación correspondiente.

**Organización**: Las tareas están agrupadas por historia para que cada incremento pueda verificarse
de forma independiente.

## Formato: `[ID] [P?] [Historia] Descripción`

- **[P]**: puede hacerse en paralelo porque afecta archivos distintos y no depende de una tarea sin
  terminar.
- **[USn]**: historia de usuario a la que pertenece la tarea.

## Fase 1: Preparación

**Propósito**: fijar las fronteras y la línea base sin crear infraestructura nueva.

- [X] T001 [P] Revisar los escenarios y las invariantes del registro MCP en `specs/025-gomemory-agnostic-operation/contracts/codex-mcp-registration.md` antes de modificar `adapters/primary/cli/cmd_mcp_setup_test.go`.
- [X] T002 [P] Revisar el contrato de interacción de texto en `specs/025-gomemory-agnostic-operation/contracts/tui-text-interaction.md` antes de modificar `adapters/primary/tui/tui_test.go`.
- [X] T003 [P] Revisar el contrato de constitución en `specs/025-gomemory-agnostic-operation/contracts/constitution-default.md` antes de modificar `domain/seed_test.go` y `application/usecases/pinned_docs_test.go`.

---

## Fase 2: Fundamentos

**Propósito**: confirmar que no se necesita infraestructura compartida adicional.

No se requieren tareas de infraestructura, modelos persistentes ni migraciones. Las tres historias
son independientes tras la Fase 1.

**Punto de control**: se puede comenzar cualquiera de las historias P1 en paralelo.

---

## Fase 3: Historia de usuario 1 - Iniciar gomemory desde cualquier proyecto registrado (Prioridad: P1) 🎯 MVP

**Objetivo**: disponer de un único registro MCP personal de Codex que migre registros por proyecto sin
alterar servidores ajenos ni depender de rutas que puedan desaparecer.

**Prueba independiente**: preparar configuración con registros heredados, incluyendo rutas inexistentes
y un servidor ajeno; ejecutar la configuración dos veces y comprobar que queda una sola entrada
compartida, una copia recuperable y permisos conservados.

### Pruebas de la historia de usuario 1

- [X] T004 [US1] Añadir casos de migración de registros `gomemory_*`, preservación byte a byte de contenido ajeno e idempotencia en `adapters/primary/cli/cmd_mcp_setup_test.go`.
- [X] T005 [US1] Añadir casos de respaldo exacto, conservación de permisos, tabla global preexistente y solicitudes concurrentes en `adapters/primary/cli/cmd_mcp_setup_test.go`.
- [X] T006 [US1] Añadir una aserción de que la desinstalación conserva el registro personal compartido en `adapters/primary/cli/cmd_uninstall_test.go`.

### Implementación de la historia de usuario 1

- [X] T007 [US1] Cambiar el registro de Codex por proyecto por una única entrada personal sin directorio de trabajo en `adapters/primary/cli/cmd_mcp_setup.go`.
- [X] T008 [US1] Implementar la migración selectiva de secciones heredadas, conservando configuración ajena, en `adapters/primary/cli/cmd_mcp_setup.go`.
- [X] T009 [US1] Implementar respaldo exclusivo, preservación de permisos y reemplazo seguro de `config.toml` en `adapters/primary/cli/cmd_mcp_setup.go`.
- [X] T010 [US1] Actualizar el mensaje de desinstalación para declarar el alcance compartido del registro en `adapters/primary/cli/cmd_uninstall.go`.
- [X] T011 [US1] Ejecutar la prueba independiente de migración y `go test ./adapters/primary/cli -count=1` conforme a `specs/025-gomemory-agnostic-operation/quickstart.md`.

**Punto de control**: Codex puede iniciar gomemory desde cualquier proyecto de la misma cuenta sin
registros por proyecto ni rutas obsoletas.

---

## Fase 4: Historia de usuario 2 - Trabajar con texto completo desde la interfaz de terminal (Prioridad: P1)

**Objetivo**: permitir copiar contenido lógico, pegar en cualquier campo activo y recorrer detalles de
memorias extensas de forma portable.

**Prueba independiente**: usar una memoria de más de 60 líneas en una terminal baja, ir al final,
copiarla desde un desplazamiento intermedio y pegar caracteres multibyte en un campo con foco.

### Pruebas de la historia de usuario 2

- [X] T012 [US2] Añadir una prueba de entrega de texto pegado al campo activo en `adapters/primary/tui/tui_test.go`.
- [X] T013 [US2] Añadir pruebas de límites, salto al final e indicadores para una memoria extensa en `adapters/primary/tui/tui_test.go`.
- [X] T014 [US2] Añadir una prueba de copia íntegra de detalle desplazado, incluidos metadatos opcionales, en `adapters/primary/tui/tui_test.go`.

### Implementación de la historia de usuario 2

- [X] T015 [US2] Centralizar la acción de copia y producir texto semántico sin formato de terminal en `adapters/primary/tui/tui.go`.
- [X] T016 [US2] Enrutar los eventos de pegado al único campo de texto con foco en `adapters/primary/tui/tui.go`.
- [X] T017 [US2] Añadir estado de desplazamiento, cálculo de líneas visuales y límites de navegación al detalle en `adapters/primary/tui/tui.go`.
- [X] T018 [US2] Mostrar las ayudas de copia y desplazamiento en las vistas de documentos y detalle en `adapters/primary/tui/tui.go` y `adapters/primary/tui/tui_docs.go`.
- [X] T019 [US2] Ejecutar la prueba independiente y `go test ./adapters/primary/tui -count=1` conforme a `specs/025-gomemory-agnostic-operation/quickstart.md`.

**Punto de control**: una persona puede leer, copiar y pegar el contenido completo sin depender de
una ruta, utilidad del sistema o tamaño particular de terminal.

---

## Fase 5: Historia de usuario 3 - Reutilizar una constitución sin identidad impuesta (Prioridad: P2)

**Objetivo**: distribuir una constitución base agnóstica y preservar el texto que cada equipo ya haya
personalizado.

**Prueba independiente**: inicializar o restaurar una constitución, verificar que no tiene identidad
particular, importar una versión personalizada y comprobar que una nueva siembra no la reemplaza.

### Pruebas de la historia de usuario 3

- [X] T020 [P] [US3] Añadir una aserción sobre el contenido agnóstico de la plantilla en `domain/seed_test.go`.
- [X] T021 [P] [US3] Añadir pruebas de siembra idempotente, restauración explícita e importación no destructiva en `application/usecases/pinned_docs_test.go`.
- [X] T022 [US3] Añadir una prueba de sincronización que no cree una estructura spec-kit ausente en `adapters/primary/cli/cmd_constitution_test.go`.

### Implementación de la historia de usuario 3

- [X] T023 [US3] Retirar título y atribución particulares de la plantilla predeterminada en `infrastructure/templates/speckit-constitution-gen.md`.
- [X] T024 [US3] Mantener la resolución, siembra y restauración explícita sin sobrescribir documentos existentes en `application/usecases/pinned_docs.go`.
- [X] T025 [US3] Mantener la sincronización opcional de la constitución solo dentro de un proyecto spec-kit existente en `adapters/primary/cli/cmd_constitution.go`.
- [X] T026 [US3] Ejecutar la prueba independiente y `go test ./domain ./application/usecases ./adapters/primary/cli -count=1` conforme a `specs/025-gomemory-agnostic-operation/quickstart.md`.

**Punto de control**: la constitución inicial es reutilizable y una personalización existente solo se
modifica mediante una acción explícita.

---

## Fase 6: Pulido y validación transversal

**Propósito**: verificar que los tres incrementos conservan el comportamiento agnóstico y preparar el
resultado para publicación.

- [X] T027 [P] Verificar que la documentación de release describe el registro compartido, la interacción textual y la constitución agnóstica en `CHANGELOG.md`.
- [X] T028 Ejecutar el recorrido completo de `specs/025-gomemory-agnostic-operation/quickstart.md`, incluidos `go test ./... -count=1`, `go vet ./...` y la compilación de `./infrastructure/`.
- [X] T029 Ejecutar `git diff --check` y contrastar los resultados con los criterios SC-001 a SC-010 de `specs/025-gomemory-agnostic-operation/spec.md`.
- [X] T030 Ejecutar `$speckit-converge` para reconciliar esta lista con el código ya publicado y añadir solamente brechas verificadas en `specs/025-gomemory-agnostic-operation/tasks.md`.

---

## Dependencias y orden de ejecución

### Dependencias de fase

- **Fase 1**: no depende de otras tareas y sus tres revisiones son paralelas.
- **Fase 2**: no contiene trabajo bloqueante adicional.
- **US1, US2 y US3**: pueden comenzar después de sus revisiones de la Fase 1. US1 y US2 son P1 e
  independientes entre sí; US3 es P2 e independiente de ambas.
- **Fase 6**: requiere completar las historias que se desee incluir en la entrega.

### Dependencias por historia

- **US1**: T004–T006 antes de T007–T010; T011 valida el incremento.
- **US2**: T012–T014 antes de T015–T018; T019 valida el incremento.
- **US3**: T020–T022 antes de T023–T025; T026 valida el incremento.

### Oportunidades de paralelismo

- T001, T002 y T003 pueden realizarse a la vez.
- T020 y T021 pueden realizarse a la vez porque afectan archivos de prueba distintos.
- Tras la Fase 1, un equipo puede desarrollar US1, US2 y US3 en paralelo: cada historia tiene
  contratos, pruebas y archivos principales separados.
- T027 puede realizarse en paralelo con la verificación técnica de T028.

## Ejemplos de ejecución paralela

### Historia de usuario 1

```text
Tarea: “Añadir casos de migración e idempotencia en adapters/primary/cli/cmd_mcp_setup_test.go”
Tarea: “Añadir aserción de desinstalación en adapters/primary/cli/cmd_uninstall_test.go”
```

### Historia de usuario 2

```text
Tarea: “Añadir pruebas de pegado, desplazamiento y copia en adapters/primary/tui/tui_test.go”
Tarea: “Revisar las ayudas de documentos en adapters/primary/tui/tui_docs.go”
```

### Historia de usuario 3

```text
Tarea: “Añadir aserción de plantilla agnóstica en domain/seed_test.go”
Tarea: “Añadir pruebas de documento fijado en application/usecases/pinned_docs_test.go”
```

## Estrategia de implementación

### MVP primero

1. Completar la Fase 1.
2. Completar US1, que elimina el fallo de arranque MCP.
3. Ejecutar T011 y validar el contrato del registro compartido.

### Entrega incremental

1. Añadir US1 y validar la migración segura.
2. Añadir US2 y validar interacción textual completa.
3. Añadir US3 y validar una constitución reutilizable.
4. Ejecutar la Fase 6 y converger contra el código publicado.

### Nota para esta funcionalidad ya publicada

Esta lista representa el orden canónico de construcción. Antes de volver a implementar cualquier tarea,
`$speckit-converge` debe comparar cada una con el código de v2.11.0 y conservar solo trabajo que no
esté realmente cubierto.

---

## Fase 7: Convergencia

- [X] T031 Retirar las referencias a Speckit y Kolmena Core de `infrastructure/templates/speckit-constitution-gen.md` y ampliar `domain/seed_test.go` para impedir su reaparición, conforme a FR-020, FR-021 y US3/AC1 (partial).
