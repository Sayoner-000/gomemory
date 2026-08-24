# Plan de implementación: Operación agnóstica de gomemory

**Rama**: `025-gomemory-agnostic-operation` | **Fecha**: 2026-08-24 | **Spec**: [spec.md](spec.md)

**Entrada**: Especificación de la funcionalidad publicada en v2.11.0.

## Resumen

Formalizar y validar tres capacidades ya entregadas como una operación agnóstica: un registro MCP de
Codex compartido por la cuenta y migrable desde registros por proyecto; interacción textual completa
en la TUI mediante copiar, pegar y recorrer detalles extensos; y una constitución predeterminada sin
identidad particular. El diseño conserva las capas actuales: la CLI administra configuración externa,
la TUI gestiona eventos y presentación, y la aplicación resuelve documentos fijados sin depender de
infraestructura.

## Contexto técnico

**Lenguaje/versión**: Go 1.25.0, toolchain Go 1.25.11  
**Dependencias principales**: Bubble Tea v2 y componentes Charm para TUI; SDK MCP; SQLite sin CGO  
**Almacenamiento**: SQLite para memorias y documentos fijados; archivo de configuración de Codex de
ámbito personal; plantillas embebidas como valores por defecto  
**Pruebas**: `go test` estándar, incluidas pruebas unitarias de CLI y TUI  
**Plataforma objetivo**: CLI y TUI para Linux, macOS y Windows; integración MCP por stdio  
**Tipo de proyecto**: aplicación CLI con servidor MCP y TUI  
**Objetivos de rendimiento**: migración de configuración sin bloqueo perceptible; desplazamiento y
copia inmediatos para memorias de hasta 10.000 caracteres y 500 líneas  
**Restricciones**: sin dependencia de rutas de proyecto, utilidades de portapapeles específicas ni
agentes concretos; preservar contenido ajeno, permisos y personalizaciones  
**Alcance**: tres adaptadores/documentos existentes, sin nuevas tablas ni servicios externos

## Verificación de constitución

| Principio | Evaluación | Evidencia de diseño |
|---|---|---|
| Arquitectura hexagonal | Cumple | La CLI contiene I/O de configuración; la TUI contiene eventos y presentación; el caso de uso de documentos recibe la plantilla por parámetro. |
| SQLite con SQL directo | No aplica | No se agregan entidades persistentes ni migraciones. |
| Testing first | Cumple | Se mantienen pruebas de migración, idempotencia, pegado, desplazamiento, copia íntegra y documentos fijados. |
| Configuración y entorno | Cumple | La configuración MCP se obtiene en el ámbito personal previsto; no se versionan ni registran secretos. |
| Simplicidad e idempotencia | Cumple | Una única entrada MCP, transformación selectiva, copia desde el modelo y estado de desplazamiento acotado. |
| Portabilidad | Cumple | La TUI usa capacidades del terminal; no invoca binarios ni rutas exclusivas de una plataforma. |

**Resultado previo a la investigación**: aprobado. No hay violaciones que requieran seguimiento de
complejidad.

## Estructura del proyecto

### Documentación de la funcionalidad

```text
specs/025-gomemory-agnostic-operation/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── codex-mcp-registration.md
│   ├── tui-text-interaction.md
│   └── constitution-default.md
├── checklists/requirements.md
└── tasks.md                         # se crea con speckit-tasks
```

### Código afectado

```text
adapters/primary/
├── cli/
│   ├── cmd_mcp_setup.go
│   ├── cmd_mcp_setup_test.go
│   ├── cmd_uninstall.go
│   ├── cmd_constitution.go
│   └── cmd_docs.go
└── tui/
    ├── tui.go
    ├── tui_docs.go
    └── tui_test.go

application/usecases/
├── pinned_docs.go
└── seed_defaults.go

domain/seed.go
infrastructure/templates/speckit-constitution-gen.md
```

**Decisión de estructura**: no se crean paquetes ni capas nuevas. Cada responsabilidad permanece en
el componente que ya posee su frontera externa o su estado de interacción.

## Verificación posterior al diseño

El diseño detallado en `research.md`, `data-model.md` y los contratos conserva las mismas decisiones
de capa y no añade persistencia, dependencias ni configuración versionada. La verificación de
constitución sigue aprobada.

## Complejidad

No aplica: no hay excepciones a la constitución que justificar.
