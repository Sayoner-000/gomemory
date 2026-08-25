# Tareas: Consolidación de hooks de Codex

**Entrada**: Documentos de diseño en `specs/026-consolidate-codex-hooks/`

**Prerrequisitos**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/hooks-config.md`, `quickstart.md`

**Pruebas**: La especificación exige validación estática, carga estricta y sesiones reales. Las comprobaciones se
definen y ejecutan antes de retirar la fuente heredada.

**Organización**: Las tareas se agrupan por historia de usuario. La implementación modifica configuración local de
Codex, no el código fuente de gomemory.

## Formato: `[ID] [P?] [Historia] Descripción`

- **[P]**: Puede ejecutarse en paralelo porque usa artefactos distintos y no depende de una tarea incompleta.
- **[Historia]**: Historia de usuario cubierta por la tarea.
- El directorio `/tmp/codex-hooks-consolidation.XXXXXX/` representa el directorio real creado con `mktemp -d`; su ruta
  resuelta debe registrarse y reutilizarse durante toda la ejecución.

## Fase 1: Preparación

**Propósito**: Confirmar prerrequisitos y establecer un punto de recuperación antes de cualquier cambio.

- [X] T001 Verificar Codex CLI, permisos y disponibilidad de los destinos requeridos por el inventario de hooks; registrar resultados en `/tmp/codex-hooks-consolidation.XXXXXX/prerequisites.md`
- [X] T002 Crear el respaldo recuperable de `/root/.codex/config.toml` y `/root/.codex/hooks.json`, con hashes SHA-256 en `/tmp/codex-hooks-consolidation.XXXXXX/baseline.sha256`

---

## Fase 2: Fundamentos bloqueantes

**Propósito**: Definir las pruebas y capturar la línea base antes de activar la configuración consolidada.

**⚠️ CRÍTICO**: Ninguna modificación de `/root/.codex/config.toml` puede comenzar hasta completar esta fase.

- [X] T003 [P] Guardar el diagnóstico previo de `codex doctor --json` en `/tmp/codex-hooks-consolidation.XXXXXX/doctor-before.json`
- [X] T004 [P] Extraer con un lector TOML la configuración ajena a `hooks.SessionStart` y `hooks.state` en `/tmp/codex-hooks-consolidation.XXXXXX/non-hooks-before.json`
- [X] T005 Definir antes del cambio las aserciones de fuente única, conteos, confianza, eventos y rollback en `/tmp/codex-hooks-consolidation.XXXXXX/acceptance-checks.md`

**Punto de control**: Existen respaldo, hashes, línea base y criterios de prueba ejecutables.

---

## Fase 3: Historia de usuario 1 — Iniciar Codex sin configuración ambigua (Prioridad: P1) 🎯 MVP

**Objetivo**: Dejar `config.toml` como única fuente activa y eliminar la advertencia de carga doble.

**Prueba independiente**: Una sesión nueva inicia sin la advertencia sobre `hooks.json` y `config.toml`; la inspección
estructural encuentra los hooks únicamente en `config.toml`.

### Pruebas para la historia de usuario 1

- [X] T006 [US1] Ejecutar las aserciones de fuente única contra el estado previo y registrar el fallo esperado en `/tmp/codex-hooks-consolidation.XXXXXX/us1-red.md`

### Implementación de la historia de usuario 1

- [X] T007 [US1] Inventariar ambas fuentes y construir `/root/.codex/config.toml.candidate` según `specs/026-consolidate-codex-hooks/contracts/hooks-config.md`, conservando una aparición por identidad normalizada y solo estados cuya fuente, posición e identidad no cambien
- [X] T008 [US1] Validar estructuralmente `/root/.codex/config.toml.candidate`, instalarlo mediante reemplazo atómico en `/root/.codex/config.toml` y guardar `codex --strict-config doctor --json` en `/tmp/codex-hooks-consolidation.XXXXXX/doctor-candidate.json`; restaurar el respaldo si la carga no es válida
- [X] T009 [US1] Mover `/root/.codex/hooks.json` al directorio `/tmp/codex-hooks-consolidation.XXXXXX/`, abrir una sesión nueva y registrar en `/tmp/codex-hooks-consolidation.XXXXXX/us1-green.md` que existe una sola fuente y no aparece la advertencia de carga doble

**Punto de control**: US1 funciona de forma independiente; el MVP elimina la ambigüedad sin perder ninguna definición
única del inventario.

---

## Fase 4: Historia de usuario 2 — Preservar todos los hooks existentes (Prioridad: P2)

**Objetivo**: Autorizar cada hook migrado o reubicado y demostrar que todos conservan su semántica y efecto observable.

**Prueba independiente**: El inventario normalizado anterior y posterior contiene las mismas identidades únicas y cada
acción produce una sola evidencia observable con sus filtros, límites y opciones originales.

### Pruebas para la historia de usuario 2

- [X] T010 [US2] Definir y preparar un observador adecuado para cada identidad del inventario, con salida unificada en `/tmp/codex-hooks-consolidation.XXXXXX/hook-events.jsonl`

### Implementación de la historia de usuario 2

- [X] T011 [US2] Revisar y autorizar cada hook migrado o reubicado desde `/root/.codex/config.toml`, dejando que Codex persista hashes no vacíos y habilitados sin copiarlos ni calcularlos manualmente
- [X] T012 [US2] Ejecutar los eventos aplicables con los observadores activos y documentar preservación de identidades, campos y efectos en `/tmp/codex-hooks-consolidation.XXXXXX/us2-verification.md`

**Punto de control**: US2 demuestra que la consolidación conserva todos los hooks únicos sin acoplarse a proveedores.

---

## Fase 5: Historia de usuario 3 — Evitar ejecuciones equivalentes duplicadas (Prioridad: P3)

**Objetivo**: Demostrar que cada identidad normalizada se ejecuta exactamente una vez para cada evento aplicable.

**Prueba independiente**: La matriz de eventos contiene una sola evidencia por identidad aplicable y no existen grupos
normalizados duplicados en TOML.

### Pruebas para la historia de usuario 3

- [X] T013 [US3] Validar con un lector TOML que `/root/.codex/config.toml` contiene exactamente una aparición por identidad normalizada, y registrar el resultado en `/tmp/codex-hooks-consolidation.XXXXXX/us3-structure.md`

### Implementación de la historia de usuario 3

- [X] T014 [US3] Ejecutar cada evento inventariado, registrando fuente, sesión y conteo observable por identidad en `/tmp/codex-hooks-consolidation.XXXXXX/event-matrix.md`

**Punto de control**: Las tres historias están verificadas y ningún evento multiplica hooks equivalentes.

---

## Fase 6: Cierre y comprobaciones transversales

**Propósito**: Confirmar invariantes, recuperabilidad y cumplimiento completo de la guía de validación.

- [X] T015 [P] Comparar el estado no relacionado con la línea base y registrar equivalencia semántica y diff limitado a hooks en `/tmp/codex-hooks-consolidation.XXXXXX/non-hooks-after.json`
- [X] T016 Ejecutar `specs/026-consolidate-codex-hooks/quickstart.md`, comprobar hashes y rollback sin activarlo, y consolidar toda la evidencia en `/tmp/codex-hooks-consolidation.XXXXXX/verification.md`
- [X] T017 Conservar `/tmp/codex-hooks-consolidation.XXXXXX/` para rollback hasta recibir decisión explícita de eliminación y registrar su ruta final en `specs/026-consolidate-codex-hooks/tasks.md`

---

## Dependencias y orden de ejecución

### Dependencias por fase

- **Preparación (Fase 1)**: Sin dependencias; T001 precede a T002 porque fija el directorio real de trabajo.
- **Fundamentos (Fase 2)**: Depende de T002 y bloquea cualquier modificación; T003 y T004 pueden ejecutarse en
  paralelo, seguidas de T005.
- **US1 (Fase 3)**: Depende de Fundamentos; T006 → T007 → T008 → T009.
- **US2 (Fase 4)**: Depende de T009 porque los hooks migrados deben estar cargados desde la fuente consolidada; T010 → T011 → T012.
- **US3 (Fase 5)**: Depende de T011 para que ambos hooks estén autorizados; T013 → T014.
- **Cierre (Fase 6)**: Depende de US1, US2 y US3; T015 puede iniciarse tras T014 y T016 consolida toda la evidencia.

### Dependencias entre historias

```text
Preparación → Fundamentos → US1 (fuente única) → US2 (hooks preservados) → US3 (matriz de eventos) → Cierre
```

- **US1 (P1)**: Entrega el MVP y no depende de otras historias.
- **US2 (P2)**: Requiere la fuente única de US1 para verificar las nuevas posiciones y procedencias.
- **US3 (P3)**: Requiere ambos hooks autorizados para medir los cuatro eventos sin falsos negativos.

### Oportunidades de paralelismo

- T003 y T004 pueden ejecutarse en paralelo después del respaldo.
- T015 puede preparar la comparación normalizada mientras se recopila la evidencia final de T014, siempre que no
  escriba en `/root/.codex/config.toml`.
- Las mutaciones y pruebas de sesión no se paralelizan para evitar carreras sobre confianza y conteos.

## Ejemplos de ejecución paralela

### Fundamentos

```text
Tarea: "Guardar diagnóstico previo en /tmp/codex-hooks-consolidation.XXXXXX/doctor-before.json"
Tarea: "Extraer configuración no-hooks en /tmp/codex-hooks-consolidation.XXXXXX/non-hooks-before.json"
```

### Cierre

```text
Tarea: "Comparar configuración no-hooks en /tmp/codex-hooks-consolidation.XXXXXX/non-hooks-after.json"
Tarea: "Compilar la última fila de eventos en /tmp/codex-hooks-consolidation.XXXXXX/event-matrix.md"
```

## Estrategia de implementación

### MVP primero: Historia de usuario 1

1. Completar preparación y fundamentos.
2. Ejecutar la prueba roja T006.
3. Consolidar la configuración mediante T007–T009.
4. Detenerse y verificar que desapareció la advertencia antes de autorizar o probar en profundidad los hooks migrados.

### Entrega incremental

1. **US1**: Una sola fuente, sin advertencia y con todas las identidades únicas representadas.
2. **US2**: Confianza renovada y ejecución efectiva de todos los hooks demostrada.
3. **US3**: Ausencia de duplicados demostrada en las cuatro fuentes de evento.
4. **Cierre**: Invariantes, evidencia y rollback verificados.

## Notas

- Las operaciones sobre `/root/.codex` requieren autorización de escritura fuera del repositorio.
- No editar los scripts, ejecutables ni destinos referenciados por los hooks.
- No eliminar el respaldo durante la implementación.
- Si falla la carga estricta o una prueba de sesión, detenerse y aplicar el rollback definido en `quickstart.md`.
- Marcar cada tarea al completarla y anexar la ruta real del directorio temporal en T017.

**Resultado de T017**: el respaldo local se conservó fuera del repositorio durante la verificación. Las instalaciones
públicas generan respaldos con nombre único junto a cada archivo migrado.

## Fase 7: Distribución mediante GoMemory

- [X] T018 Implementar la conversión estructural y agnóstica de hooks JSON a TOML en `adapters/primary/cli/codex_hooks_migration.go`.
- [X] T019 Integrar la consolidación en `setupCodexGlobal`, compartido por `mem install` y `mem setup-mcp --scope global --agents codex`.
- [X] T020 Respaldar ambos archivos, validar el candidato antes de escribir y restaurar `config.toml` si no puede retirarse `hooks.json`.
- [X] T021 Probar preservación de hooks ajenos, deduplicación, campos desconocidos, idempotencia y conservación de JSON inválido.
- [X] T022 Documentar la capacidad distribuida y preparar las notas de versión 2.12.0.
