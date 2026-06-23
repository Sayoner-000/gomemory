# Constitución Genérica — Proyectos Speckit - GSGR y OTROS.
#Autor: Jose Gomez - Jefe Automatización

Esta constitución aplica a **todo proyecto nuevo** del ecosistema Speckit. Codifica las lecciones aprendidas del proyecto Kolmena Core y establece defaults técnicos, arquitectónicos y operativos. Cada proyecto puede _extender_ esta constitución, pero no _relajar_ sus reglas sin justificación explícita documentada en su propio `constitution.md`.

Aplica a proyectos en: **Python 3.12+**, **FastAPI**, **Go**, **Vite + TS/JS**, **SQLite**/PostgreSQL, y cualquier combinación de ellos.

---

## 1. Stack por Lenguaje

### Python / FastAPI
| Capa | Tecnología | Default |
|------|------------|---------|
| Web framework | FastAPI | >=0.115.0 |
| Validación | Pydantic v2 | >=2.9.0 |
| Settings | pydantic-settings | >=2.6.0 |
| DB Driver | SQLAlchemy (asyncio) | >=2.0.35 |
| Testing | pytest + pytest-asyncio | >=8.3.0 |
| Linter | Ruff | >=0.7.0 |
| Type checker | mypy | >=1.13.0 |
| Lint line length | 100 | — |
| Formateo | Ruff format | — |
| Async | async/await siempre | — |

### Go
| Capa | Tecnología | Default |
|------|------------|---------|
| Versión | Go 1.22+ | >=1.22 |
| Web framework | Chi o stdlib `net/http` | Sin framework pesado |
| DB Driver | `database/sql` + `sqlx` | — |
| Migraciones | `golang-migrate/migrate` | — |
| Testing | `testing` stdlib + `testify` | — |
| Linter | `golangci-lint` | — |
| Codegen | `sqlc` (opcional) | — |

### Vite + TypeScript (Frontend)
| Capa | Tecnología | Default |
|------|------------|---------|
| Framework | React 18+ / Solid / Svelte | Sin preferencia |
| Build | Vite | >=5.0 |
| Language | TypeScript | strict mode |
| Linter | ESLint + Prettier | — |
| Testing | Vitest | — |
| CSS | Tailwind CSS | >=3.0 |

### Persistencia
| Motor | Default | Alternativa |
|-------|---------|-------------|
| SQLite | `database/sql` (Go) / `aiosqlite` (Python) | SQLite es el default para prototipos y proyectos pequeños |
| PostgreSQL | asyncpg / psycopg (Python) / pgx (Go) | Para producción y multi-instancia |
| Migraciones | SQL raw idempotente | Sin ORM (salvo justificación explícita) |

---

## 2. Contenerización y Entornos Aislados

Cada proyecto se ejecuta dentro de su propio stack Docker con redes, volúmenes y servicios independientes. Nunca se usa `network_mode: host`.

### Estructura Docker obligatoria

```
{proyecto}/
├── Dockerfile              # Imagen del servicio principal
├── docker-compose.yml      # Stack completo con servicios, redes y volúmenes
├── .dockerignore
└── caddy/                  # (opcional) Reverse proxy por proyecto
    └── Caddyfile
```

### Reglas

- **Red privada por proyecto**: cada `docker-compose.yml` define su propia red `{proyecto}_network` con `driver: bridge`. Los contenedores se comunican por nombre de servicio, no por IP. Esto evita colisión con redes del host y de otros proyectos.
- **Puertos mapeados explícitamente**: solo los puertos necesarios se exponen al host (`ports:` en docker-compose). Nunca `network_mode: host`.
- **Volúmenes nombrados**: para datos persistentes (DB, uploads). Prefijo `{proyecto}_{volumen}`. Evitar `bind mounts` del host salvo para desarrollo (código fuente).
- **Variables de entorno desde archivo**: `docker-compose.yml` usa `env_file: .env` para inyectar configuración. No hardcodear valores en `environment:`.
- **Multi-stage build**: el Dockerfile usa etapas separadas para build y runtime, minimizando la imagen final.
- **Healthcheck**: cada servicio expone un healthcheck. El `depends_on` condiciona el arranque a `condition: service_healthy`.
- **`.dockerignore` obligatorio**: excluir `node_modules/`, `__pycache__/`, `.git/`, `.env`, `tests/`, `docs/` de la imagen.

### Ejemplo docker-compose.yml

```yaml
version: "3.9"

services:
  app:
    build: .
    ports:
      - "${PORT}:8000"
    env_file: .env
    networks:
      - proyecto_network
    depends_on:
      db:
        condition: service_healthy
    volumes:
      - ./src:/app/src

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: ${DB_NAME}
      POSTGRES_USER: ${DB_USER}
      POSTGRES_PASSWORD: ${DB_PASS}
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER} -d ${DB_NAME}"]
      interval: 5s
      timeout: 3s
    networks:
      - proyecto_network
    volumes:
      - proyecto_db_data:/var/lib/postgresql/data

networks:
  proyecto_network:
    driver: bridge

volumes:
  proyecto_db_data:
```

---

## 3. Arquitectura Hexagonal Obligatoria

Todo proyecto DEBE organizarse en 4 capas. No se permiten desviaciones sin documentar en `docs/ARQUITECTURA.md`:

```
src/{proyecto}/
├── domain/         # Puro — sin I/O, sin imports de infraestructura
├── application/    # Ports (interfaces) + casos de uso / orquestación
├── adapters/       # Implementaciones concretas de cada port
└── infrastructure/ # Framework, settings, DB, composition root
```

### Regla de dependencias

| Capa | Importa de | Prohibido importar |
|------|------------|-------------------|
| `domain` | stdlib, Pydantic/schemas | Librerías externas (requests, DB, framework) |
| `application` | `domain` | Adaptadores concretos, infraestructura |
| `adapters` | `application`, libs externas | Framework de presentación (HTTP, CLI) |
| `infrastructure` | Todas | — |

**Invariante verificable:**

```bash
# Python: domain + application NO deben tener imports de infraestructura
grep -r "httpx\|sqlalchemy\|fastapi\|requests" src/{proyecto}/domain src/{proyecto}/application
# → debe retornar vacío

# Go: similar con go vet y análisis de imports
```

### Puertos (interfaces) en application/ports/

Toda dependencia externa se declara como interfaz abstracta en `application/ports.py` (Python) o `application/ports.go` (Go).

```python
# Python
class PlanRepo(ABC):
    @abstractmethod
    async def get_by_intent(self, intent: str) -> Plan | None: ...
```

```go
// Go
type PlanRepo interface {
    GetByIntent(ctx context.Context, intent string) (*Plan, error)
}
```

Reglas:
- **Naming**: `{Nombre}Port` para servicios, `{Nombre}Repo` para persistencia.
- **Sin exponer DB driver**: los repos usan sesión internamente.
- **`None` / `nil` para no encontrado**, nunca excepciones de "not found".

---

## 4. Composition Root

El wiring de dependencias ocurre en **un solo lugar**. Sin DI framework:

### Python
```python
# infrastructure/composition.py
@dataclass
class AppDeps:
    repo: PlanRepo
    service: SomeService

async def build_deps(settings: Settings) -> AppDeps:
    db = SessionFactory()
    return AppDeps(
        repo=PgPlanRepo(db),
        service=SomeService(),
    )
```

### Go
```go
// infrastructure/composition.go
type AppDeps struct {
    Repo    PlanRepo
    Service SomeService
}

func BuildDeps(cfg *Config) *AppDeps {
    db := sqlx.MustConnect("sqlite3", cfg.DatabaseURL)
    return &AppDeps{
        Repo:    NewPgPlanRepo(db),
        Service: NewSomeService(),
    }
}
```

Reglas:
- **Sin DI frameworks** (inspector, dig, wire, dependency_injector, etc.).
- **Una sola función** que construye todo el grafo de dependencias.
- **Mock switching**: una variable de entorno (ej. `USE_MOCK_ADAPTERS`) intercambia todo el stack.

---

## 5. Persistencia

### SQLite (default para prototipos, dev, proyectos pequeños)

```python
# Python
import aiosqlite

class PgPlanRepo(PlanRepo):
    async def get(self, id: int) -> Plan | None:
        async with aiosqlite.connect("dev.db") as db:
            db.row_factory = aiosqlite.Row
            cursor = await db.execute("SELECT * FROM plans WHERE id = ?", (id,))
            row = await cursor.fetchone()
            return Plan(**dict(row)) if row else None
```

```go
// Go
type PlanRepoSQLite struct {
    db *sql.DB
}

func (r *PlanRepoSQLite) Get(ctx context.Context, id int) (*Plan, error) {
    row := r.db.QueryRowContext(ctx, "SELECT * FROM plans WHERE id = ?", id)
    var p Plan
    err := row.Scan(&p.ID, &p.Name /* ... */)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, nil
    }
    return &p, err
}
```

### Reglas universales para DB
- **SQL directo, sin ORM** (salvo que el proyecto tenga justificación explícita en `docs/ARQUITECTURA.md`).
- **Migraciones SQL idempotentes**: `CREATE TABLE IF NOT EXISTS`, `ALTER TABLE ... IF NOT EXISTS`.
- **Numeración secuencial** de migraciones: `001_nombre.sql`, `002_nombre.sql`...
- **Parámetros bind obligatorios**: nunca f-strings o concatenación para valores SQL.
- **`commit()` explícito** en escritura. Rollback automático en error.

---

## 6. Testing

### Organización de tests
```
tests/
├── unit/       # Tests de dominio y aplicación (con mocks)
├── integration/# Tests con DB real o servicios externos
└── contract/   # Tests de contrato entre componentes
```

### Reglas
- **pytest** para Python, `go test` para Go, **Vitest** para frontend.
- **Mock de puertos**: cada port tiene un mock (en `adapters/*/mock.go` o `adapters/*/mock.py`).
- **Cobertura ≥ 80%**.
- **Los tests existentes son intocables**: no se modifican sin autorización. Código nuevo trae tests nuevos.
- **Tests de repositorio obligatorios** cuando se crea un repo nuevo.
- **Nombrar tests**: `test_{modulo}_{funcionalidad}.py` / `{paquete}_test.go`.

---

## 7. Settings y Configuración

### Python (pydantic-settings)
```python
class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")
    
    database_url: str = Field(alias="DATABASE_URL")
    log_level: str = Field(default="INFO", alias="LOG_LEVEL")

@lru_cache
def get_settings() -> Settings:
    return Settings()
```

### Go (envconfig / env)
```go
type Config struct {
    DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`
    LogLevel    string `envconfig:"LOG_LEVEL" default:"INFO"`
    Port        int    `envconfig:"PORT" default:"8080"`
}
```

### Reglas
- **Una sola struct/class de config** para todo el proyecto.
- **Cero valores hardcodeados**: toda variable que cambie entre entornos (URLs, puertos, credenciales, keys, timeouts, flags) DEBE venir del entorno. No se aceptan valores fijos en el código.
- **Variables desde entorno** (`.env` para dev, variables de sistema para prod). `.env` NUNCA se versiona — solo existe `.env.example` como template documentado.
- **Sin lógica en config**: solo declaración de campos y defaults. Sin if/else, sin derivaciones.
- **Singleton**: cargar una vez al arrancar, cachear en memoria. Los cambios requieren reinicio del proceso (salvo features con recarga en caliente justificada).
- **Todas las variables documentadas** en `.env.example` con su propósito, default y si son obligatorias u opcionales.

---

## 8. Mock Adapters

Cada interfaz de `application/ports` tiene **al menos una implementación mock/noop** en su subdirectorio de adapters.

```python
# adapters/llm/mock.py
class MockLLM(LLMPort):
    async def chat(self, messages, **kwargs) -> str:
        return "Mock response"
```

Reglas:
- **Mock por subdirectorio**: `adapters/llm/mock.py`, `adapters/repos/mock_repo.py`.
- **Noop pattern** para servicios desactivables: `NoopGuardrails`, `NoopAuditEventAdapter`.
- **Switcheables por variable de entorno** desde composition root.

---

## 9. API HTTP (FastAPI)

### Organización
```
infrastructure/http/
├── app.py           # Factory de la aplicación
├── auth.py          # Dependencias de auth (middleware, tokens)
├── middleware.py    # Middleware personalizado
└── routes/          # Un archivo por dominio
    ├── users.py
    ├── plans.py
    └── health.py
```

### Patrón de ruta
```python
router = APIRouter(prefix="/api/{dominio}", tags=["{dominio}"])

@router.get("/", response_model=List[Schema])
async def listar(request: Request):
    deps = request.app.state.deps
    items = await deps.repo.list_all()
    return [_serialize(i) for i in items]
```

### Reglas
- **Sin lógica de negocio en rutas**: solo reciben request, validan entrada, llaman al port/servicio, serializan respuesta.
- **Acceso a deps via `request.app.state.deps`**.
- **`response_model` explícito** para OpenAPI.
- **Serialización inline**: funciones `_serialize_*` en el mismo módulo (no capa separada).

---

## 10. Manejo de Errores

### En capas internas (domain, application)
- **Retornar `None`/`nil`** para "no encontrado" (nunca excepción).
- **Excepciones reales** solo para condiciones inesperadas (DB caída, bug).

### En orquestación (nodos, workers, handlers)
- **Nunca propagar errores de I/O** al caller: loguear como WARNING y retornar estado seguro.
- **Fire-and-forget**: fallos de notificaciones/auditoría no bloquean el flujo principal.

### En API HTTP
- **`HTTPException`** con código y mensaje claro.
- **Errores de validación**: 422 con detalle del campo.
- **No encontrado**: 404.
- **Conflicto**: 409.

---

## 11. Convenciones de Estilo

### Imports (Python)
Orden estricto: `from __future__` → stdlib → blanco → third-party → blanco → proyecto.

### Naming
| Elemento | Convención | Ejemplo |
|----------|------------|---------|
| Módulos | `snake_case` | `memory_recall.py` |
| Clases Python | `PascalCase` | `PgPlanRepo` |
| Interfaces Python | `{Nombre}Port(ABC)` | `LLMPort` |
| Interfaces Go | `{Nombre}Repo` | `PlanRepo` |
| Structs Go | `PascalCase` | `PgPlanRepo` |
| Archivos Go | `snake_case.go` | `plans_pg.go` |
| Constantes | `UPPER_SNAKE` en Python, `camelCase` en Go | `_RECALL_LIMIT` |
| Tests | `test_{modulo}.py` / `{modulo}_test.go` | — |

### Line length
- Python: **100** caracteres (configurado en Ruff).
- Go: **120** (estándar `gofumpt`).
- TypeScript: **100**.

---

## 12. Cache

### Reglas generales
- **Cache de lectura (CQRS-lite)**: opcional, activable por feature.
- **TTL fijo**: 300s (5 min) por defecto.
- **Invalidación explícita**: en cada escritura del recurso cacheado.
- **Fallback transparente**: si cache falla, leer de DB.
- **Nunca cachear en proceso** valores que cambian en caliente.

---

## 13. Migraciones

### Formato
```
migrations/
├── 001_initial_schema.sql
├── 002_add_users_table.sql
└── ...
```

### Reglas
- **SQL puro**, no DSL de ORM.
- **Idempotentes**: usar `IF NOT EXISTS`, `IF EXISTS`.
- **Numeración secuencial**: 3 dígitos, cero a la izquierda.
- **Runner automático**: en startup de la aplicación.

---

## 14. Frontend (Vite + TypeScript)

### Estructura
```
src/
├── api/        # Llamadas HTTP (un archivo por dominio)
├── components/ # Componentes UI reutilizables
├── hooks/      # Custom hooks
├── pages/      # Páginas/rutas
├── stores/     # Estado global (Zustand, Context)
├── types/      # TypeScript types/interfaces
└── utils/      # Funciones helper
```

### Reglas
- **TypeScript strict mode** en `tsconfig.json`.
- **Componentes puros**: reciben props, renderizan UI. Sin lógica de negocio.
- **Lógica en hooks o stores**, no en componentes.
- **API calls**: funciones tipadas en `api/`, no dispersas en componentes.
- **Tailwind CSS** para estilos (salvo justificación).
- **Tests con Vitest** para hooks y utilidades.

---

## 15. Documentación técnica

### Archivos obligatorios en la raíz
| Archivo | Propósito |
|---------|-----------|
| `README.md` | Qué hace, stack, arranque rápido, API pública |
| `constitution.md` | (opcional) Extensiones/especificidades de esta constitución |
| `docs/ARQUITECTURA.md` | Capas, flujo, decisiones de diseño |
| `docs/DATABASE.md` | Schema, queries de analítica, mantenimiento |

### Idioma: español latino

Toda la documentación técnica, especificaciones, planes y tareas dentro de `docs/` y `specs/` se escribe **en español latino**, con lenguaje claro y accesible.

- **Lenguaje claro**: oraciones cortas, voz activa, términos sencillos. Pensar en un developer que no habla inglés técnico avanzado.
- **Sin spanglish**: evitar mezclar inglés y español. Preferir "interfaz" sobre "interface", "archivo" sobre "file", "desplegar" sobre "deploy". Si un término técnico no tiene traducción clara (ej. "endpoint", "middleware"), se usa en cursiva y se explica la primera vez.
- **Especificaciones (`spec.md`)**: describir el problema y la solución en español. Los nombres técnicos (variables, endpoints, tablas) pueden ir en inglés, pero la narrativa es en español.
- **Tasks y planes**: redactados como instrucciones accionables en español. Ej: "Agregar validación de email en el endpoint de registro" en lugar de "Add email validation to register endpoint".

### Archivos obligatorios por feature (bajo `specs/`)
```
specs/{NNN}-{nombre}/
├── spec.md          ← [OBLIGATORIO] especificación funcional
├── plan.md          ← [OBLIGATORIO tras planning] plan de implementación
├── tasks.md         ← [OBLIGATORIO tras planning] tareas accionables
├── checklists/
│   └── requirements.md ← [OBLIGATORIO] criterios de aceptación
├── contracts/
│   └── api-{dominio}.md ← [OPCIONAL] contratos de API
├── data-model.md    ← [OPCIONAL] modelo de datos
├── research.md      ← [OPCIONAL] investigación previa
└── quickstart.md    ← [OPCIONAL] guía rápida
```

---

## 16. Estructura de Proyecto (template)

```
{proyecto}/
├── .env.example
├── .gitignore
├── CLAUDE.md               # Instrucciones para agentes AI
├── README.md
├── constitution.md          # Extensiones a esta constitución
├── pyproject.toml           # (Python) o go.mod / package.json
├── requirements.txt         # (Python) Pinned deps
├── Dockerfile
├── docker-compose.yml
├── migrations/
│   └── 001_initial_schema.sql
├── docs/
│   ├── ARQUITECTURA.md
│   └── DATABASE.md
├── specs/                   # Features y cambios
├── src/{proyecto}/
│   ├── __init__.py
│   ├── domain/
│   ├── application/
│   ├── adapters/
│   │   ├── repos/
│   │   ├── cache/
│   │   └── ...
│   └── infrastructure/
│       ├── settings.py / config.go
│       ├── db.py / db.go
│       ├── composition.py / composition.go
│       └── http/ (si aplica)
│           ├── app.py / server.go
│           ├── auth.py / auth.go
│           ├── middleware.py
│           └── routes/
├── tests/ (Python) / *_test.go (Go)
│   ├── unit/
│   ├── integration/
│   └── contract/
└── static/ (si aplica, frontend build)
```

---

## 17. Stack Congelado (resumen multi-lenguaje)

| Capa | Python | Go | TypeScript |
|------|--------|----|------------|
| Web | FastAPI 0.115+ | Chi / net/http | — |
| DB | SQLAlchemy 2.0+ asyncio | sqlx | — |
| SQLite | aiosqlite | database/sql | — |
| Tests | pytest 8+ | testing + testify | Vitest |
| Lint | Ruff 0.7+ | golangci-lint | ESLint |
| Types | mypy 1.13+ | — | strict TS |
| Migrations | SQL raw | golang-migrate | — |

---

## 18. Gestión de Dependencias y Seguridad

Toda dependencia del proyecto debe validarse contra el repositorio Context7 para garantizar que se usa la versión más reciente y libre de vulnerabilidades conocidas.

### Reglas

- **Validación Context7 obligatoria**: al añadir o actualizar una dependencia, consultar Context7 para verificar la versión más reciente estable y el estado de seguridad de la librería.
- **Sin dependencias desactualizadas**: si una librería tiene más de 2 versiones menores por detrás de la última estable, debe actualizarse. Si tiene vulnerabilidades públicas conocidas (CVE), el update es obligatorio antes de mergear.
- **Pin de versiones**: las dependencias se fijan con versión exacta (ej. `fastapi>=0.115.0,<0.116`) para evitar roturas por updates automáticos, pero dentro de un rango que permita patches de seguridad.
- **Scanner de vulnerabilidades en CI**: integrar `pip-audit` (Python), `govulncheck` (Go) o `npm audit` (TS) en el pipeline de CI. El build falla si se detectan vulnerabilidades CRITICAL o HIGH sin fix disponible.
- **Registro de decisión**: cuando una dependencia con CVE conocida no puede actualizarse (breaking change mayor, incompatibilidad), documentar la excepción en `docs/DEPENDENCIES.md` con: CVE, versión afectada, riesgo, plan de mitigación y fecha de revisión.
- **Revisión periódica**: programar una revisión de dependencias cada 3 meses. Context7 es la fuente de verdad para determinar si una librería tiene versión más reciente.

### Imágenes Docker

Las imágenes base del Dockerfile también son dependencias y se gestionan con las mismas reglas de seguridad.

- **Pin de tag específico**: usar siempre un tag de versión explícito (ej. `python:3.12-slim`, `golang:1.22-alpine`). Nunca `latest`. El tag debe tener versión menor fija, no solo major (ej. `postgres:16-alpine`, no `postgres:alpine`).
- **Validación de imagen base**: antes de fijar o actualizar una imagen base, consultar Context7 (o Docker Hub oficial) para verificar:
  - Versión más reciente estable disponible
  - CVE abiertas conocidas de esa versión
  - Imagen slim/alpine como preferencia (menor superficie de ataque)
- **Multi-stage build**: separar etapa de build (con SDK/compiilador) de etapa runtime (slim/alpine). La imagen final solo contiene lo necesario para ejecutar.
- **Scanner de imágenes**: integrar `trivy` o `docker scout` en CI para escanear la imagen construida. El build falla si se detectan vulnerabilidades CRITICAL o HIGH.
- **Registro de excepción**: cuando una imagen base con CVEs no puede actualizarse, documentar en `docs/DEPENDENCIAS.md` con: CVE, versión, riesgo, plan de mitigación y fecha de revisión.
- **Revisión periódica**: incluir las imágenes base en la revisión trimestral de dependencias.

### Flujo para añadir nueva dependencia

```
1. Identificar necesidad → documentar qué problema resuelve
2. Consultar Context7 para la versión más reciente estable
3. Verificar CVE conocidas de esa versión
4. Añadir a pyproject.toml / go.mod / package.json con pin de versión
5. Ejecutar scanner local (pip-audit / govulncheck / npm audit)
6. Documentar en PR por qué se necesita y qué alternativa se descartó
```

---

## 19. Prohibiciones Absolutas

- **PROHIBIDO** importar adapters concretos desde `application/` o `domain/`.
- **PROHIBIDO** exponer el DB driver/sesión al caller de un repositorio.
- **PROHIBIDO** usar f-strings/string concatenation para valores SQL.
- **PROHIBIDO** modificar tests existentes sin autorización explícita.
- **PROHIBIDO** cachear en proceso valores que cambian en caliente.
- **PROHIBIDO** añadir dependencias sin justificación.
- **PROHIBIDO** usar ORM sin justificación documentada en `docs/ARQUITECTURA.md`.
- **PROHIBIDO** mezclar lógica de negocio en rutas HTTP o handlers.
- **PROHIBIDO** compartir sesiones de DB entre requests (cada operación abre su propia sesión).
- **PROHIBIDO** hardcodear valores de configuración (URLs, puertos, credenciales, timeouts, flags) en el código fuente. Todo debe venir de variables de entorno.
- **PROHIBIDO** versionar `.env`. Solo se versiona `.env.example` con valores placeholder y documentación de cada variable.
- **PROHIBIDO** usar `network_mode: host` en docker-compose. Cada proyecto define su propia red bridge aislada.
- **PROHIBIDO** escribir documentación técnica, specs o tareas en inglés. Todo se redacta en español latino con lenguaje claro.
- **PROHIBIDO** añadir o actualizar una dependencia sin validar su versión contra Context7 primero.
- **PROHIBIDO** mergear código con dependencias que tengan CVEs CRITICAL o HIGH sin fix disponible, salvo excepción documentada en `docs/DEPENDENCIES.md`.
- **PROHIBIDO** usar `latest` como tag de imagen Docker en producción. Toda imagen base debe tener un tag de versión explícito.
- **PROHIBIDO** omitir el escaneo de vulnerabilidades en imágenes Docker (trivy / docker scout) en el pipeline de CI.

---

## 20. Principios Operativos

1. **Simplicidad primero**: cada cambio debe ser lo más simple posible. Impactar el mínimo código.
2. **Sin soluciones temporales**: encontrar la causa raíz. Nunca parches superficiales.
3. **Código > comentarios**: el código debe ser autoexplicativo. Comentar solo el "por qué", nunca el "qué".
4. **Tests primero** (TDD): escribir el test antes de la implementación cuando sea práctico.
5. **Documentar decisiones**: toda decisión arquitectónica o tradeoff se documenta en `docs/ARQUITECTURA.md` con fecha y contexto.
6. **Fallar rápido**: validar inputs en el borde del sistema. No propagar datos inválidos a capas internas.
7. **Fire-and-forget para no crítico**: notificaciones, auditoría, eventos secundarios nunca bloquean el flujo principal.
8. **Idempotencia**: toda operación de escritura debe poder repetirse sin efectos secundarios.
9. **Aislamiento Docker**: cada proyecto corre en su propio stack con redes bridge privadas. Los servicios nunca asumen que corren en el host ni que tienen acceso a redes de otros proyectos.
