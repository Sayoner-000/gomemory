# Quickstart: validar el brazo extensor gomemory ↔ /speckit-specify

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

Guía de validación manual end-to-end. No incluye código de implementación
— referencia los contratos en `contracts/` y el modelo en `data-model.md`.

## Prerrequisitos

- Build de gomemory con los cambios de esta feature (`SpeckitContextDisabled`
  en settings, fila nueva en TUI, flag `--speckit-context` en `mem
  settings`).
- Binario `./mem` presente en la raíz de un proyecto de prueba con spec-kit
  instalado (`.specify/` existente).
- La extensión `.specify/extensions/gomemory-context/` instalada y
  registrada (ver research.md #6 — vía `specify extension install`).
- Al menos una memoria de tipo `decision` o `architecture` guardada en ese
  proyecto (`./mem save -t "..." -y decision "..."`), para que el resumen
  tenga contenido que verificar.

## Escenario 1 — Historia 1: resumen automático al crear una especificación

1. `mem settings --show` → confirmar `Brazo extensor spec-kit: true`.
2. Invocar `/speckit-specify` con una descripción de feature nueva.
3. **Esperado**: antes de que se complete `spec.md`, el flujo muestra que
   se ejecutó el hook `speckit.gomemory-context.update`
   (`EXECUTE_COMMAND`, mandatorio) y su salida incluye el resumen de
   `mem context` — sin haber pedido explícitamente ese resumen.
4. Verificar que ninguna herramienta de lectura de archivos abrió
   manualmente `spec.md` de otras carpetas bajo `specs/` para producir ese
   resumen (FR-002).

## Escenario 2 — Historia 2: secciones separadas por origen

1. Con un proveedor externo de grafo de código disponible (o simulando su
   ausencia), repetir el paso 2 del Escenario 1.
2. **Esperado**: si el proveedor externo está disponible, el resumen trae
   dos bloques rotulados por separado (historial/decisiones de gomemory vs.
   `## Grafo de código externo (<provider>)`). Si no está disponible, solo
   aparece el bloque de gomemory, sin mención al grafo externo.

## Escenario 3 — Historia 4: apagar el interruptor

1. Abrir la TUI de gomemory (`./mem`) → pantalla de configuración.
2. Alternar la fila "Brazo extensor spec-kit" a apagado.
3. `mem settings --show` → confirmar `Brazo extensor spec-kit: false`.
4. Invocar `/speckit-specify` de nuevo.
5. **Esperado**: el hook mandatorio sigue ejecutándose (aparece en el
   flujo), pero su salida es vacía — ningún resumen se incorpora, sin error
   visible.
6. Reactivar el interruptor y repetir el paso 4 — el resumen vuelve a
   aparecer (Acceptance Scenario 3 de Historia 4).

## Escenario 4 — degradación transparente sin gomemory / sin historial

1. En un proyecto con spec-kit pero **sin** `./mem` instalado (ni en PATH),
   invocar `/speckit-specify`.
2. **Esperado**: el flujo de creación de especificación se completa igual
   que sin esta feature, sin mensajes de error (SC-003).
3. En un proyecto nuevo (gomemory recién inicializado, sin memorias),
   repetir — el resumen puede venir vacío o solo con el encabezado, sin
   bloquear ni fallar.

## Escenario 5 — sin efecto en proyectos sin spec-kit

1. En un proyecto que usa gomemory pero **no** tiene `.specify/`, abrir la
   TUI de gomemory.
2. **Esperado**: la fila "Brazo extensor spec-kit" existe (mismo patrón que
   la fila del grafo de código externo) pero alternarla no tiene ningún
   efecto observable, porque no hay hook de spec-kit que la consulte
   (SC-006) — no se agrega ninguna detección activa de `.specify/` de parte
   de gomemory.

## Criterio de éxito del quickstart

Los cinco escenarios cubren, en conjunto, todos los Acceptance Scenarios de
`spec.md` (Historias 1–4) y los Success Criteria SC-001 a SC-007.
