# Implementation Plan: Socrática → Spec Kit — preparación guiada y validada

**Branch**: `main` | **Date**: 2026-10-24 | **Spec**: [spec.md](spec.md)

**Input**: Especificación `specs/042-socratica-specify-preparation/spec.md`.

## Summary

Ampliar Socrática con `mode=specify` para preparar una necesidad, comprobar campos y relaciones, revisar una entrada determinista y entregarla a Spec Kit solo tras aprobación explícita. Gomemory controla las condiciones de avance; el agente conduce la conversación, comprueba disponibilidad e invoca la skill existente.

Se extienden el reductor puro, el caso de uso, el repositorio transaccional, la herramienta MCP y la distribución canónica actuales. La preparación se guarda como agregado opcional; los modos previos mantienen sus contratos. El modo privado nuevo mantiene el contenido efímero y solo reserva metadatos sin texto en el almacenamiento durable. `ack` distingue la primera concesión de sus reintentos mediante `execution_granted` para el modo nuevo.

Las decisiones y alternativas están en [research.md](research.md); los estados y datos en [data-model.md](data-model.md). La planificación termina con estos artefactos: ninguna hoja del árbol se implementa en esta fase.

## Technical Context

**Language/Version**: Go 1.27.0, toolchain 1.27.3, según `go.mod`.

**Primary Dependencies**: MCP Go SDK 1.7.0, modernc.org/sqlite 1.58.0, Bubble Tea 2.0.9 y componentes TUI ya fijados. Biblioteca estándar para validadores, render e identidad exacta. Sin dependencias o proveedores nuevos.

**Storage**: SQLite local existente y repositorio/UnitOfWork de Socrática. Agregado JSON nullable en flujos e historial; política de payload por flujo. Depósito efímero por instancia para contenido de `specify` privado, con reserva SQL sin texto para aislamiento y despacho.

**Testing**: `testing`, suites existentes de dominio, casos de uso, SQLite, MCP, instalación, CLI/TUI y contrato/integración. Pruebas deterministas sin modelos externos; demostración real opt-in por cada agente como aceptación adicional.

**Target Platform**: CLI/servidor MCP local en Linux, macOS y Windows; agentes Claude Code, Codex y OpenCode ya soportados.

**Project Type**: CLI y servidor MCP con TUI y skills distribuidas; conserva arquitectura hexagonal.

**Performance Goals**: Comprobaciones de campos y referencias en O(n + r), con n elementos y r referencias; render local determinista. Sin llamadas al modelo o red desde los validadores ni desde la consulta/dibujado de la TUI. No se añade un objetivo temporal ajeno a la especificación.

**Constraints**: Tres preguntas acumuladas por flujo; un pendiente por proyecto/agente; identidad exacta de revisión/aprobación/entrega; una concesión por autorización; migración aditiva; ninguna preparación privada en disco; cero cambios de contrato público de `refine/scrutiny`.

**Scale/Scope**: Un recorrido nuevo de Socrática, tres agentes, una herramienta MCP ampliada, consulta/reanudación local, un wrapper nuevo con dos destinos. Spec Kit genera `spec.md` y su checklist; no se implementa la necesidad de negocio ni un ejecutor nuevo fuera del agente.

## Constitution Check

*Puerta evaluada antes de cerrar investigación y nuevamente tras el diseño.*

Se consultaron `mem constitution` (base 2.0.0) y `.specify/memory/constitution.md` (enmienda local 2.1.0). La base no reemplaza el archivo local ni su excepción §25.1. No se introducen excepciones nuevas.

| Principio / puerta | Aplicación al diseño | Antes de investigación | Después del diseño |
| --- | --- | --- | --- |
| §3–4, límites hexagonales y composición | Tipos/reglas/render en dominio; coordinación en aplicación; MCP/SQL/memoria/instalación en adaptadores; selección en `infrastructure/container.go`. | PASS | PASS |
| §5, transacciones y SQL explícito | Preparación y revisión se actualizan con la transición y CAS existente; concesión solo después del commit; sin conexiones expuestas por ports. | PASS | PASS |
| §6 + §25.1, migraciones | Columnas aditivas dentro del mecanismo transaccional existente; subir catálogo, actualizar golden y probar desde v2/base vacía. Excepción vigente hasta 2027-01-23. | PASS | PASS |
| §8 y §21, idempotencia | `ack` idempotente separado de primera concesión; revisiones obsoletas rechazadas; reenvíos solo mediante autorización nueva explícita. | PASS | PASS |
| §9 y §15, privacidad | Protección antes de render; cobertura de sanitización/borrado; payload privado efímero y metadatos sin texto; sin prompts en memorias generales. | PASS | PASS |
| §10–11, pruebas por comportamiento | Dominio/UOW con reloj e IDs inyectados; SQLite real local; concurrencia y rollback; fakes y proceso helper multiplataforma donde aplique. | PASS | PASS |
| §0 y §15, dependencias | Decisión Extend/Adopt respaldada por búsqueda local; no se incorpora ni actualiza una dependencia. | PASS | PASS |
| §16–18, convenciones y artefactos | Identificadores/pruebas en inglés; narrativa en español; diseño separado de implementación. `tasks.md` corresponde a la fase siguiente. | PASS | PASS |

PASS es una evaluación del diseño, no evidencia de código implementado o pruebas ejecutadas. La aceptación no se cierra sin evidencias reales por agente. Las exigencias de servicios desplegados no se convierten en infraestructura nueva para una herramienta local.

## Project Structure

### Documentation (this feature)

```text
specs/042-socratica-specify-preparation/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── socratica-flow.md
│   ├── agent-skill.md
│   ├── preparation.schema.json
│   └── example-preparation.json
└── checklists/requirements.md
```

`tasks.md` generado mediante `speckit-tasks` (69 tareas en 9 fases).

### Source Code (repository root)

Rutas existentes y ampliaciones previstas; los archivos marcados como nuevos aún no se crean.

```text
domain/
├── socratica.go
├── socratica_specify.go                 # nuevo: tipos, reglas y transiciones específicas
├── socratica_specify_render.go          # nuevo: texto revisable determinista
└── privacy_maintenance.go
application/
├── ports/socratica_repository.go
└── usecases/
    ├── socratica_flow.go
    └── socratica_specify.go             # nuevo: coordinación de preparación y reportes
adapters/
├── primary/
│   ├── cli/cmd_mcp_socratica_tool.go
│   ├── cli/cmd_socratica.go
│   ├── cli/cmd_mcp_setup.go
│   ├── tui/socratica.go
│   └── setup/agent_skills_setup.go
└── secondary/persistence/
    ├── db.go
    ├── socratica.go
    ├── socratica_schema.go
    ├── socratica_sanitize.go
    └── socratica_ephemeral.go           # nuevo: payload efímero + reserva coordinada
infrastructure/
├── container.go
└── templates/socratica/
    ├── SKILL.md
    └── commands/socratica-hu.md         # nuevo wrapper fino
skills/socratica/SKILL.md
tests/contract/
tests/integration/
docs/socratica.md
docs/DATABASE.md
```

**Structure Decision**: conservar las capas y ports existentes; separar el modo nuevo en módulos enfocados cuando evita agrandar el reductor o adaptador generales. Las pruebas unitarias permanecen junto a su código. Los contracts de esta feature describen la ampliación pública; no se crea una API o servicio independiente.

## Fases de diseño y secuencia de implementación

**Fase 0 terminada**: [research.md](research.md) resuelve reutilización, etapas, revisión exacta, concesión de ejecución, persistencia, privacidad, distribución y evidencia de agentes. No hay decisiones de diseño sin resolver.

**Fase 1 terminada**: [data-model.md](data-model.md), [contrato MCP](contracts/socratica-flow.md), [contrato de agentes](contracts/agent-skill.md), esquema y ejemplo de preparación y [quickstart.md](quickstart.md). Puerta constitucional reevaluada tras definirlos.

El siguiente árbol prepara la fase de tareas. Las hojas son resultados verificables todavía por construir; `✓` denota hoja atómica, no avance realizado. Dependencias explícitas; las ramas independientes pueden desarrollarse en paralelo sin compartir responsabilidad de archivos.

```text
🎯 Entregar una preparación validada y aprobada a Spec Kit sin duplicar generación
├─ [1] Extender el contrato de dominio
│  ├─ [1.1] ✓ Definir tipos de preparación → modelo nuevo compilable
│  ├─ [1.2] ✓ Implementar validadores de etapas → casos de campos/referencias/bloqueos verdes (dep: 1.1)
│  ├─ [1.3] ✓ Implementar render exacto → pruebas de identidad determinista verdes (dep: 1.1)
│  └─ [1.4] ✓ Extender el reductor → pruebas de transición e invalidación verdes (dep: 1.2, 1.3)
├─ [2] Extender almacenamiento y confirmación (dep: 1)
│  ├─ [2.1] ✓ Añadir columnas y versión de catálogo → migración/golden verdes
│  ├─ [2.2] ✓ Mapear preparación e historial → round-trip y legado verdes (dep: 2.1)
│  ├─ [2.3] ✓ Coordinar payload efímero y reserva → pruebas de privado/rollback verdes (dep: 2.2)
│  ├─ [2.4] ✓ Distinguir primera confirmación → replay sin concesión adicional (dep: 2.2)
│  └─ [2.5] ✓ Ampliar sanitización de preparación → depuración invalida aprobación antigua (dep: 2.2)
├─ [3] Coordinar el recorrido en aplicación (dep: 1, 2)
│  ├─ [3.1] ✓ Coordinar prepare y next → pruebas de avance controlado verdes
│  ├─ [3.2] ✓ Vincular aprobación a revisión → pruebas de bypass/revisión obsoleta verdes (dep: 3.1)
│  └─ [3.3] ✓ Registrar resultado comunicado → reportes coherentes sin nueva concesión (dep: 3.2)
├─ [4] Presentar el modo nuevo (dep: 3)
│  ├─ [4.1] ✓ Ampliar herramienta MCP → contrato publicado y validaciones verdes
│  ├─ [4.2] ✓ Proteger consulta CLI de specify → cero ejecuciones locales genéricas
│  └─ [4.3] ✓ Adaptar consulta TUI de specify → vista pura con retorno al agente
├─ [5] Conectar la skill (dep: 4.1)
│  ├─ [5.1] ✓ Actualizar cuerpo canónico/publicado → paridad y procedimiento next verdes
│  └─ [5.2] ✓ Distribuir wrapper socratica-hu → instalación/desinstalación idempotentes (dep: 5.1)
└─ [6] Verificar aceptación y operación (dep: 4, 5)
   ├─ [6.1] ✓ Ejecutar puerta automática → build/vet/pruebas/race/cobertura aprobados
   ├─ [6.2] ✓ Demostrar recorrido en Claude Code → spec/checklist y trazabilidad inspeccionados (dep: 6.1)
   ├─ [6.3] ✓ Demostrar recorrido en Codex → spec/checklist y trazabilidad inspeccionados (dep: 6.1)
   ├─ [6.4] ✓ Demostrar recorrido en OpenCode → spec/checklist y trazabilidad inspeccionados (dep: 6.1)
   └─ [6.5] ✓ Documentar operación y compatibilidad → guía con evidencia y límites reales (dep: 6.2, 6.3, 6.4)
```

Autoverificación: 22 hojas, sin decisiones pendientes disfrazadas de tareas. Las demostraciones reales son ejecuciones acotadas de la guía en un solo agente cada una; si falta autenticación, capacidad o permiso operativo se informa como aceptación pendiente, sin sustituirla por un test de `refine`.

## Cobertura y puertas de aceptación

| Alcance | Diseño / hojas | Verificación requerida |
| --- | --- | --- |
| FR-001 a FR-010 | Dominio, preparación; 1 y 3.1 | Cuatro clases de solicitud; omisiones y límite acumulativo; referencias y etapas inválidas. |
| FR-011 a FR-015 | Invalidación/revisión; 1.3–1.4, 3.2 | Editar después de comprobar/aprobar; comparar bytes; cancelar; rechazar bypass de acciones previas. |
| FR-016 a FR-020 | Traspaso y resultado; 2.4, 3.2–3.3, 5.1 | Falta de Spec Kit sin consumo; replay/concurrencia/rollback; resultado real y trazabilidad. |
| FR-021 a FR-024 | Persistencia/privado/presentación; 2, 4.2–4.3 | Migración y recuperación; dos procesos; depuración, retención y pérdida de payload; consulta sin efectos. |
| FR-025 a FR-028 | Distribución y compatibilidad; 4–6 | Tres agentes; paridad de copias; segunda instalación sin escrituras; desinstalación; regresiones de modos anteriores. |

La puerta automática debe incluir pruebas con SQLite real local, múltiples instancias para la reserva privada, actores concurrentes para editar/aprobar/confirmar y fallo de commit. Se usan fakes de agente/proceso helper para pruebas automáticas multiplataforma. Las tres demostraciones reales comprueban los artefactos producidos y registran que no se implementó la solicitud original antes de aprobar.

## Complexity Tracking

No hay violaciones constitucionales nuevas. El depósito efímero y la reserva sin texto son necesarios para satisfacer simultáneamente privacidad, un pendiente por proyecto/agente entre procesos y no reutilización de una autorización consumida. Se descartan memoria sin reserva, agregado privado en disco, otra herramienta MCP y otro motor de flujo; véase R6 en la investigación.

## Estado al cerrar esta planificación

Artefactos de diseño completos y comprobados documentalmente. Se verificaron enlaces locales, referencias de requisitos, ausencia de marcadores pendientes, dependencias de las 22 hojas y espacios en los ocho archivos nuevos de diseño. El esquema se resolvió con `github.com/google/jsonschema-go/jsonschema`, ya fijada en el proyecto: aceptó el ejemplo completo y rechazó actores vacíos, una marca `ready` inyectada y un pendiente resuelto sin nota. La utilidad de comprobación vive únicamente en un directorio temporal; no se añade código de producto ni una dependencia.

Implementación, pruebas del nuevo modo y demostraciones reales pendientes. La configuración YAML se analizó: solo hay hooks opcionales en `before_plan` y `after_plan`, sin hooks obligatorios. No se ejecutaron: el contexto de memoria ya se cargó mediante MCP y la actualización del contexto del agente queda disponible como acción opcional. Próximo comando: `speckit-implement`.
