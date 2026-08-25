# Plan de implementación: Consolidación de hooks de Codex

**Rama**: `main` | **Fecha**: 2026-08-24 | **Especificación**: [spec.md](spec.md)

**Entrada**: Especificación de `specs/026-consolidate-codex-hooks/spec.md`

## Resumen

Consolidar todos los hooks locales de Codex en `/root/.codex/config.toml`: inventariar ambas fuentes, normalizar cada
definición por evento, filtro y acción, eliminar equivalencias duplicadas y migrar sin reglas ligadas a proveedores o
comandos concretos. La transición será recuperable y renovará la confianza únicamente cuando cambie la identidad o
procedencia de un hook.

## Contexto técnico

**Lenguaje/versión**: Go 1.25; TOML y JSON de configuración; Codex CLI 0.149.1

**Dependencias principales**: `encoding/json`, `github.com/pelletier/go-toml/v2`; cargador de configuración y sistema
de hooks de Codex

**Persistencia**: archivos locales bajo `/root/.codex`; no se modifica la base de datos del proyecto

**Pruebas**: pruebas unitarias de migración e idempotencia, carga TOML estructural, `codex doctor --json`, inspección
estática y sesiones reales de Codex

**Plataforma objetivo**: instalación local Linux de Codex para el usuario `root`

**Tipo de proyecto**: extensión del adaptador CLI de Codex en GoMemory y migración de configuración operativa

**Objetivos de rendimiento**: cada hook conserva sus límites actuales y ninguna identidad equivalente se ejecuta más
de una vez por evento

**Restricciones**: una sola fuente activa; no tocar modelo, MCP ni confianza del proyecto; no modificar destinos de
hooks; preservar campos compatibles desconocidos; permitir rollback completo

**Escala/alcance**: todos los eventos, grupos y acciones presentes en las dos representaciones actuales, incluidas sus
entradas de confianza relacionadas

## Comprobación de la constitución

*Puerta evaluada antes de la investigación y nuevamente después del diseño.*

- **Arquitectura hexagonal**: No aplica; no se modifica código de dominio, aplicación ni adaptadores.
- **SQLite con SQL directo**: No aplica; no hay cambios de persistencia del proyecto.
- **Pruebas primero**: Cumple de forma proporcional mediante un contrato de configuración y validaciones estáticas y
  de sesión definidas antes del cambio. No se alteran pruebas existentes.
- **Configuración y entorno**: Cumple; el cambio corrige una duplicación en configuración operativa y no introduce
  valores de entorno en el código de gomemory.
- **Principios operativos**: Cumple simplicidad, causa raíz, alcance mínimo, idempotencia y validación antes de retirar
  el origen antiguo.
- **Documentación en español**: Cumple en todos los artefactos de la feature.

**Resultado previo a Fase 0**: APROBADO, sin excepciones.

**Resultado posterior a Fase 1**: APROBADO. El diseño no incorpora código, dependencias, bases de datos ni interfaces
públicas nuevas; el contrato se limita al formato de configuración consumido por Codex.

## Estructura del proyecto

### Documentación de esta funcionalidad

```text
specs/026-consolidate-codex-hooks/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── hooks-config.md
├── checklists/
│   └── requirements.md
└── tasks.md                  # Se generará con $speckit-tasks
```

### Superficie operativa afectada

```text
/root/.codex/
├── config.toml               # Única fuente final de hooks
├── hooks.json                # Se retira tras validar la migración
└── <destinos de hooks>       # Se consumen sin modificarlos

/tmp/
└── codex-hooks-consolidation-<marca-tiempo>/
    ├── config.toml
    └── hooks.json
```

**Decisión de estructura**: La lógica distribuible vive junto al adaptador de configuración global de Codex en
`adapters/primary/cli`. `setupCodexGlobal` coordina respaldo, escritura atómica y retirada de la fuente heredada; el
migrador opera únicamente sobre estructuras JSON/TOML y no conoce proveedores ni comandos concretos.

## Secuencia de implementación

1. Capturar evidencia previa: hashes y copias de ambos archivos, salida local de diagnóstico y conteo estructural de
   definiciones mediante un lector TOML.
2. Inventariar ambas fuentes y construir el `config.toml` consolidado conforme a
   [contracts/hooks-config.md](contracts/hooks-config.md), deduplicando identidades normalizadas y preservando campos.
3. Conservar confianza solo cuando fuente, posición e identidad no cambien; retirar estados obsoletos y dejar que
   Codex autorice cualquier hook migrado o reubicado.
4. Instalar el `config.toml` consolidado mediante reemplazo atómico, manteniendo el JSON y el respaldo; validar de
   inmediato la configuración activa con carga estricta antes de retirar el JSON heredado.
5. Mover `hooks.json` fuera de su ruta reconocida, iniciar una sesión y autorizar cada hook migrado desde su nueva
   procedencia cuando Codex lo solicite, sin calcular ni copiar hashes manualmente.
6. Ejecutar la guía [quickstart.md](quickstart.md), confirmar los resultados y eliminar el respaldo temporal solo si la
   persona usuaria decide que ya no necesita rollback.

## Seguimiento de complejidad

No hay violaciones constitucionales ni complejidad adicional que justificar.
