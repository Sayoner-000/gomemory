<!--
  Sync Impact Report
  Version: (template) v0.0.0 → v1.0.0
  Modified principles: N/A (first constitution — all placeholders filled)
  Added sections: 5 principios, Stack y Tecnología (section 2),
    Desarrollo y Documentación (section 3), Gobernanza
  Removed sections: None
  Templates requiring updates:
    ✅ .specify/templates/plan-template.md (reviewed, no changes needed)
    ✅ .specify/templates/spec-template.md (reviewed, no changes needed)
    ✅ .specify/templates/tasks-template.md (reviewed, no changes needed)
    ✅ .specify/templates/checklist-template.md (reviewed, no changes needed)
  Follow-up TODOs: None
-->

# gomemory Constitution

## Principios Fundamentales

### I. Arquitectura Hexagonal

El código DEBE organizarse en capas con reglas de dependencia estrictas.
La capa de dominio NO DEBE importar infraestructura. Los puertos (interfaces)
se declaran en una capa intermedia y los adaptadores son implementaciones
concretas intercambiables.

- **Dominio**: puro, sin I/O, sin imports de infraestructura
- **Aplicación**: puertos + casos de uso, solo importa dominio
- **Adaptadores**: implementaciones concretas de cada puerto
- **Infraestructura**: composition root, settings, wiring de dependencias

El wiring de dependencias ocurre en UN SOLO LUGAR (composition root),
sin frameworks de DI. Una variable de entorno (`USE_MOCK_ADAPTERS`)
DEBE poder intercambiar todo el stack de adaptadores.

**Prohibiciones**: importar adaptadores desde dominio/aplicación, exponer
el driver de BD al caller de un repositorio, mezclar lógica de negocio
en handlers.

### II. SQLite con SQL Directo

SQLite es el motor de persistencia (vía `modernc.org/sqlite`, sin CGO).
Toda operación de BD usa SQL directo, sin ORM.

- **Migraciones SQL idempotentes**: `CREATE TABLE IF NOT EXISTS`,
  `ALTER TABLE ... IF NOT EXISTS`, numeración secuencial (`001_nombre.sql`)
- **Parámetros bind obligatorios**: nunca f-strings ni concatenación
  para valores SQL
- **`commit()` explícito** en escritura, rollback automático en error
- **WAL mode** para lecturas concurrentes, busy timeout 5s
- **Timestamps UTC-5** (Bogotá/Colombia, sin DST)

**Prohibiciones**: SQL con strings concatenados, exponer el driver/sesión
de BD al caller, compartir sesiones entre requests.

### III. Testing First (NO NEGOCIABLE)

TDD obligatorio: tests se escriben PRIMERO, fallan, y solo entonces se
implementa la solución. Ciclo Red-Green-Refactor estricto.

- **Organización**: `tests/unit/` (con mocks), `tests/integration/`
  (con BD real), `tests/contract/` (contratos entre componentes)
- **Framework**: `testing` stdlib + `testify` (Go)
- **Cobertura ≥ 80%**
- **Cada puerto tiene un mock** en su subdirectorio de adaptadores
- **Tests de repositorio obligatorios** al crear un repo nuevo
- **Tests existentes son intocables**: no se modifican sin autorización
- **Nomenclatura**: `{paquete}_test.go`

**Prohibiciones**: modificar tests existentes sin autorización explícita,
implementar antes de escribir el test.

### IV. Configuración y Entorno

TODO valor que cambie entre entornos DEBE venir de variables de entorno.

- **Una sola struct de config** para todo el proyecto
- **Sin lógica en config**: solo declaración de campos y defaults
- **Singleton**: cargar una vez al arrancar, cachear en memoria
- **`.env` no se versiona**: solo existe `.env.example` como template
  documentado con propósito, default y obligatoriedad de cada variable
- **Documentación en `.env.example`**: TODAS las variables documentadas

### V. Principios Operativos

1. **Simplicidad**: cada cambio debe ser lo más simple posible, impactando
   el mínimo código necesario
2. **Sin parches temporales**: encontrar la causa raíz, nunca soluciones
   superficiales
3. **Código autoexplicativo**: comentar solo el "por qué", nunca el "qué"
4. **Documentar decisiones**: toda decisión arquitectónica se documenta
   con fecha, contexto y tradeoffs
5. **Fallar rápido**: validar inputs en el borde del sistema
6. **Fire-and-forget**: notificaciones y auditoría no bloquean flujo principal
7. **Idempotencia**: toda operación de escritura DEBE poder repetirse
   sin efectos secundarios
8. **Cache de lectura opcional**: TTL fijo 300s, invalidación explícita
   en escritura, fallback transparente a BD
9. **MCP como integración primaria**: exponer funcionalidad vía MCP sobre stdio
10. **Exit code propagado**: `mem wrap` termina con el mismo código del comando

## Stack y Tecnología

### Stack congelado (Go)

| Capa | Tecnología | Versión |
|------|------------|---------|
| Lenguaje | Go | >=1.22 |
| CLI | `flag` stdlib + Bubbletea | v3 |
| TUI | `charmbracelet/bubbletea` | última |
| Base de datos | `modernc.org/sqlite` (sin CGO) | última |
| MCP SDK | `modelcontextprotocol/go-sdk` | última |
| Testing | `testing` + `testify` | último |
| Linter | `golangci-lint` | último |
| Lint line length | 120 | `gofumpt` |
| Formateo | `gofmt` / `gofumpt` | — |
| Timezone | UTC-5 (Bogotá, sin DST) | — |

El binario DEBE ser autocontenido (~16MB), sin dependencias runtime,
portable entre Linux, macOS y Windows.

### Dependencias y Seguridad

- **Pin de versiones**: fijar con versión exacta o rango menor
  (ej. `v1.2.3` o `>=v1.2.0,<v1.3`)
- **Scanner en CI**: `govulncheck` DEBE ejecutarse en CI y fallar
  en vulnerabilidades CRITICAL o HIGH
- **Sin dependencias desactualizadas**: si una librería tiene más de
  2 versiones menores detrás, debe actualizarse
- **Registro de excepción**: si una dependencia con CVE no puede
  actualizarse, documentar en `docs/DEPENDENCIAS.md` con CVE, riesgo,
  plan de mitigación y fecha de revisión
- **Revisión trimestral** de dependencias

## Desarrollo y Documentación

### Contenerización

El proyecto NO requiere Docker para operación normal (CLI autocontenido).
Si se usa Docker para desarrollo/pruebas:
- Red bridge privada, puertos mapeados explícitamente
- Volúmenes nombrados, multi-stage build
- Healthchecks obligatorios
- Pin de tags específicos en imágenes base, nunca `latest`

### Estilo y Convenciones

- **Imports Go**: stdlib → terceros → proyecto (con separación por espacios)
- **Naming**: `PascalCase` para exportados, `camelCase` para privados,
  `snake_case` para archivos
- **Line length**: 120 caracteres (gofumpt/gofmt)
- **Linter**: `golangci-lint` con configuración en `.golangci.yml`

### Manejo de Errores

- **`nil` para "no encontrado"**, nunca error de "not found"
- **Errores reales** solo para condiciones inesperadas (DB caída, bug)
- **Fire-and-forget**: fallos de notificaciones/auditoría no bloquean
  el flujo principal

### Documentación

Toda la documentación técnica, especificaciones, planes y tareas dentro de
`docs/` y `specs/` se escribe **en español latino**, con lenguaje claro
y accesible.

- **Archivos obligatorios**: `README.md`, `docs/ARQUITECTURA.md`,
  `docs/DATABASE.md` (si aplica)
- **Especificaciones (`specs/`)**: `spec.md` + `plan.md` + `tasks.md`
  + `checklists/requirements.md`
- **Sin spanglish**: preferir "interfaz" sobre "interface", "archivo"
  sobre "file". Términos sin traducción clara van en cursiva con
  explicación la primera vez.
- **Nombres técnicos** (variables, endpoints, tablas) pueden ir en inglés
  pero la narrativa es en español.

### Prohibiciones Absolutas

- Importar adaptadores concretos desde dominio o aplicación
- Exponer el driver/sesión de BD al caller de un repositorio
- Usar f-strings o concatenación para valores SQL
- Modificar tests existentes sin autorización explícita
- Cachear en proceso valores que cambian en caliente
- Usar ORM sin justificación documentada en `docs/ARQUITECTURA.md`
- Mezclar lógica de negocio en handlers
- Hardcodear valores de configuración en el código fuente
- Versionar `.env`
- Escribir documentación técnica en inglés

## Gobernanza

### Procedimiento de Enmienda

1. Cualquier cambio a esta constitución DEBE ser documentado en un
   PR con:
   - Descripción clara del cambio propuesto
   - Justificación: por qué se necesita
   - Impacto: qué principios o secciones se modifican
   - Plan de migración si aplica
2. La aprobación requiere revisión y aceptación explícita
3. Tras aprobar, actualizar versión, fechas y este documento

### Política de Versionado

- **MAJOR**: cambios incompatibles (principios eliminados o redefinidos)
- **MINOR**: nuevos principios o secciones agregadas
- **PATCH**: aclaraciones, correcciones, refinamientos no semánticos

### Revisión de Cumplimiento

- Todo PR DEBE verificar cumplimiento contra esta constitución
- La sección "Constitution Check" en `plan-template.md` es el punto
  de control obligatorio
- La complejidad no justificada DEBE ser reportada y requerir
  aprobación explícita

**Versión**: 1.0.0 | **Ratificado**: 2026-06-21 | **Última Enmienda**: 2026-06-21
