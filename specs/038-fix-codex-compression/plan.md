# Implementation Plan: Compresión compatible según capacidades del agente

**Branch**: `main` (sin crear rama) | **Date**: 2026-10-12 | **Spec**: [spec.md](spec.md)

**Input**: `specs/038-fix-codex-compression/spec.md`, incluida aclaración A: medir respuestas completas.

## Summary

Eliminar la prioridad contradictoria de recuperación sin habilitar la sustitución de resultados que Codex rechaza. Compartir una directiva condicional entre el protocolo permanente/MCP y los recordatorios; migrar el marcador v8 a v9 para actualizar instalaciones existentes. Priorizar recuperación comprimida en origen únicamente cuando cubra la consulta y preserve la evidencia necesaria. Conservar rutas externas para cuerpos, lenguajes o proyectos no cubiertos.

La aceptación requiere tres consultas elegibles en Codex interactivo, selección autónoma de la ruta, reducción positiva del volumen agregado de respuestas completas frente a referencias equivalentes y un caso conductual negativo independiente que demuestre degradación comunicada. La captura de entrada del modelo necesita certificación del adaptador TUI antes de medir. La implementación y esa aceptación no se ejecutan durante este comando.

## Technical Context

**Language/Version**: Go 1.27.0; toolchain go1.27.2 declarada en `go.mod`. Python 3 de biblioteca estándar para el evaluador local propuesto; sus pruebas usan pytest conforme a la constitución.

**Primary Dependencies**: SDK MCP Go v1.7.0, go-toml/v2 v2.4.3 y SQLite v1.58.0 existentes; sin dependencias nuevas de producción ni actualización de versiones. Para pruebas Python, crear en un cambio de dependencias separado un `pyproject.toml` mínimo sin paquete instalable, con grupo `dev` que fije `pytest==9.1.1` y `pip-audit==2.10.1`; generar y versionar `uv.lock` mediante `uv lock` y exportar desde él `scripts/requirements-038-test.txt` con versiones transitivas y hashes. Confirmar Context7, fuentes oficiales, Python 3.13, escaneo `pip-audit --require-hashes` y reversión según la constitución. El evaluador no importa pytest ni pip-audit.

**Storage**: archivos globales gestionados y SQLite existente; informe de aceptación JSON/Markdown local, sin tablas nuevas ni migraciones.

**Testing**: `testing`, pruebas de contrato/integración existentes, pytest local y en `.github/workflows/test.yml` para el evaluador, y sesión TUI real de Codex. La CI exige `uv.lock` actualizado, comprueba que el export con hashes coincida con `scripts/requirements-038-test.txt`, sincroniza el grupo `dev`, ejecuta `python3 -m pytest -q scripts/test_validate_038_evidence.py` y `pip-audit --require-hashes` sobre el export; falla si falta un artefacto, la suite o la auditoría. Aislar HOME, XDG, datos y PATH en pruebas de instalación.

**Target Platform**: plataformas de CLI existentes; aceptación inicial en macOS con Codex instalado y versión registrada. Regresiones de instrucciones para Claude y OpenCode sin modificar sus plugins.

**Project Type**: CLI y servidor MCP con arquitectura hexagonal existente.

**Performance Goals**: ahorro agregado estrictamente positivo en bytes UTF-8 de respuestas completas de tres consultas elegibles, sin pérdida de evidencia requerida. No implica ahorro de tokens facturados ni impone latencia nueva.

**Constraints**: no habilitar `updatedMCPToolOutput`; no alterar motor, umbrales, contratos públicos de herramientas ni permisos; no comparar metadatos con cuerpos como si fueran equivalentes. El grafo propio cubre Go, no todos los lenguajes. No declarar compresión porque el nombre de una herramienta la sugiera.

**Scale/Scope**: una directiva común, dos compositores consumidores, migración del marcador y pruebas; un evaluador de evidencia local y tres consultas reales. Fuentes globales y MCP initialize, sin reintroducir AGENTS por proyecto.

## Constitution Check

Constitución vigente obtenida mediante `mem constitution`, coincidente con `.specify/memory/constitution.md` v2.0.0.

| Puerta | Antes de investigación | Después del diseño |
|--------|-----------------------|--------------------|
| Capas y simplicidad | PASS: cambios en dominio textual/adaptadores existentes | PASS: directiva pura en dominio, sin I/O ni imports de adaptadores |
| Dependencias | PASS: sin cambios de dependencias de producción | PENDING: `pyproject.toml`, `uv.lock`, export con hashes y `pip-audit` verificables en cambio separado antes de T019 |
| Seguridad | PASS: preservar controles y contenido ajeno | PASS: evidencia pública/sintética; informes sin secretos; no ampliar permisos |
| Pruebas | PENDING: pytest exigido por la constitución | PENDING: suite pytest local y obligatoria en CI, además de contratos, instalación aislada, degradaciones y sesión real |
| Idioma y contratos | PASS: narrativa en español | PASS: identificadores en inglés y contratos verificables |
| Persistencia | PASS: no cambio de datos | PASS: sin migraciones; métricas históricas no usadas como prueba de sesión |
| Validación proporcional | PASS: no asumir motor roto | PASS: aceptación bloqueada si faltan evidencia comparable o TUI real |

No se solicita excepción constitucional: el evaluador sigue usando solo biblioteca estándar y pytest/pip-audit se limitan al entorno de pruebas. La puerta de dependencias/pruebas permanece pendiente hasta versionar `uv.lock` y el export con hashes, ejecutar suite y auditoría, y observar su resultado obligatorio en CI. Las demás puertas no afirman compilación, cobertura ≥80 % ni seguridad de una implementación aún inexistente: se comprobarán durante implementación, sin omitir CI.

## Project Structure

### Documentation (this feature)

```text
specs/038-fix-codex-compression/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── checklists/requirements.md
└── contracts/
    ├── retrieval-policy.md
    ├── validation-evidence.md
    ├── validation-evidence.schema.json
    ├── validation-evidence.example.json
    └── capture-certification.schema.json
```

Las tareas ejecutables están en [tasks.md](tasks.md); este plan describe su diseño.

### Source Code (repository root)

```text
domain/
├── protocol.go                         # marcador v8 → v9
└── code_retrieval_policy.go             # nueva directiva canónica propuesta
adapters/primary/cli/
├── cmd_install.go                       # buildIntegrationBlock/composeAgentFile
├── cmd_hook.go                          # memoryProtocolReminder
├── cmd_mcp.go                           # initialize consume buildIntegrationBlock
├── cmd_mcp_code_tools.go                # referencia; no cambio requerido
├── protocol_codex_test.go
├── cmd_install_protocol_test.go
└── code_retrieval_policy_test.go         # nuevo contrato de superficies
adapters/primary/setup/
├── codex_setup.go                       # referencia; no habilitar rewrite
└── atomic_plan_global_test.go
tests/
├── integration/protocol_block_test.go
└── contract/                           # regresiones de sincronía existentes
scripts/
├── validate_038_evidence.py             # nuevo evaluador offline propuesto
├── test_validate_038_evidence.py         # pruebas pytest
└── requirements-038-test.txt             # export con hashes generado desde uv.lock
pyproject.toml                             # grupo dev de pruebas, sin paquete instalable
uv.lock                                    # lock Python canónico, cambio separado
.github/workflows/test.yml               # puerta pytest de 038
```

**Structure Decision**: usar una constante pura `domain.CodeRetrievalPolicy` para el texto compartido; no crear router ejecutable, detector de versiones o estado global nuevo. Las instrucciones explican la decisión según capacidades realmente disponibles. Los prefijos concretos de herramientas los maneja el harness. Cambiar los dos párrafos contradictorios por la misma directiva; conservar bootstrap de descubrimiento porque disponibilidad no significa selección.

## Phase 0 — Research

[research.md](research.md) resuelve fuentes, límites de recuperación propia, propagación, migración y medición. Investigación R01 delegada con autorización Octopus; el resto se resolvió directamente con CodeGraph y contratos MCP. `codebase-memory-mcp` no lista este repositorio actual; su índice antiguo `go_memory` no se usa como fuente actual. Documentación vigente de Codex consultada con Context7, sin inferir soporte de flags/eventos recientes en 0.162.1.

La medición tiene una puerta de factibilidad explícita: en Codex 0.162.1 no se ha identificado aún una interfaz que exporte por llamada el resultado de herramienta tal como llega al modelo en TUI. T024 exige identificar interfaz y oráculo independiente de entrada del modelo, crear un certificado conforme al esquema y superar dos probes byte a byte antes de T025; T020/T022 deben rechazar un `capture_verified` afirmativo sin certificado verificable. Si no existe esa capacidad, la aceptación queda `blocked` por `measurement_unavailable`. No se presupone una ruta de éxito disponible.

## Phase 1 — Design & Contracts

- [data-model.md](data-model.md): modelos lógicos de política y evidencia; no esquema nuevo de persistencia.
- [contracts/retrieval-policy.md](contracts/retrieval-policy.md): prioridad, suficiencia, límites y preservación.
- [contracts/validation-evidence.md](contracts/validation-evidence.md), sus archivos `schema.json`/`example.json` y [capture-certification.schema.json](contracts/capture-certification.schema.json): entrada v1, certificación comprobable del adaptador TUI, codec, equivalencia, caso conductual negativo independiente, informe y estados.
- [quickstart.md](quickstart.md): pruebas futuras y aceptación interactiva reproducible.

## Árbol de implementación atómica

`✓` significa hoja ejecutable propuesta, no tarea completada. Dependencias explícitas; máximo 19 hojas.

```text
🎯 Cerrar la vía compatible con instrucciones consistentes y evidencia real comparable
├─ [1] Fijar contratos en pruebas
│  ├─ [1.1] ✓ Añadir casos de prioridad/suficiencia → contrato rojo (FR-001–005)
│  ├─ [1.2] ✓ Añadir fixture v8 con bloques ajenos → migración roja (FR-006) (∥)
│  └─ [1.3] ✓ Añadir casos del evaluador → rechazo de pares inválidos (FR-008–011) (∥)
├─ [2] Unificar instrucciones (dep: 1.1)
│  ├─ [2.1] ✓ Definir CodeRetrievalPolicy → texto canónico puro
│  ├─ [2.2] ✓ Consumir directiva en buildIntegrationBlock → protocolo consistente (dep: 2.1)
│  └─ [2.3] ✓ Consumir directiva en memoryProtocolReminder → recordatorio consistente (dep: 2.1)
├─ [3] Actualizar instalaciones (dep: 1.2, 2.2)
│  ├─ [3.1] ✓ Incrementar ProtocolVersionMarker a v9 → reemplazo de v8
│  └─ [3.2] ✓ Verificar instalación repetida aislada → preservación e idempotencia (dep: 3.1)
├─ [4] Verificar compatibilidad (dep: 2.2, 2.3, 3.2)
│  ├─ [4.1] ✓ Verificar initialize y recordatorios → mismo fragmento canónico
│  ├─ [4.2] ✓ Verificar perfiles compatible/desconocido → sin promesas falsas (∥)
│  └─ [4.3] ✓ Verificar retirada de hook Codex → rewrite sigue deshabilitado (∥)
├─ [5] Preparar medición (dep: 1.3)
│  ├─ [5.1] ✓ Implementar evaluador offline → informe conforme al contrato
│  └─ [5.2] ✓ Fijar tres consultas y sus hechos necesarios → manifiesto elegible (dep: 5.1)
├─ [6] Validar sesión real (dep: 4, 5)
│  ├─ [6.1] ✓ Certificar captura por llamada y verificar binario/configuración TUI → entorno registrado o bloqueo explícito
│  ├─ [6.2] ✓ Capturar tres consultas interactivas y degradación negativa → rutas, respuestas y motivo comunicado (dep: 6.1)
│  ├─ [6.3] ✓ Capturar referencias sin compresión equivalentes → pares válidos (dep: 6.2)
│  └─ [6.4] ✓ Evaluar pares y evidencia necesaria → cierre/pending/blocked (dep: 6.3)
└─ [7] Verificar regresión final (dep: 4, 5.1)
   ├─ [7.1] ✓ Ejecutar controles Go, pytest local y puerta CI → resultados de calidad
   └─ [7.2] ✓ Examinar impacto de cambios → llamadores afectados documentados (dep: 7.1)
```

La etapa 6 no puede ejecutarse sin acceso a una TUI real y un adaptador certificado que extraiga las capturas completas. Si faltan, el informe queda `blocked`; no se sustituye por `codex exec`. Los manifiestos elegible y negativo se fijan antes de la sesión de aceptación y no se seleccionan consultas a posteriori para ocultar fallos.

## Cobertura y riesgos

| Requisitos | Diseño/verificación |
|------------|---------------------|
| FR-001–004 | directiva común + pruebas de superficies/perfiles |
| FR-005 | alternativa explícita por cobertura y evidencia insuficiente, más observación negativa real separada del ahorro |
| FR-006 | v8→v9, límites de bloque, repetición aislada y baseline preservado |
| FR-007 | manifiesto previo, TUI real, tres consultas sin tool coaching |
| FR-008 | adaptador TUI certificado o bloqueo, respuesta completa, pares misma fuente/revisión/parámetros y recuperaciones adicionales |
| FR-009 | hechos y referencias predefinidos comprobados por consulta |
| FR-010–011 | causas separadas y estados cerrados por evaluador |

Riesgos de cierre: instrucciones externas ajenas aún pueden contradecir la directiva; el modelo puede ignorarla; respuestas pequeñas pueden no reducirse; resultados Go estructurales pueden ser insuficientes. Mitigación: preservación y advertencia, evaluación de conducta real y comparaciones de alcance equivalente. Ninguno se oculta con un test verde.
