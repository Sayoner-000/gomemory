# Constitución Técnica

**Versión:** 2.0.0
**Fecha de corte:** 2026-09-24

> Esta constitución aplica a todo proyecto nuevo. Codifica lecciones aprendidas en proyectos anteriores y define valores predeterminados técnicos, arquitectónicos, operativos y de seguridad. Un proyecto puede extender estas reglas, pero no relajarlas sin una excepción explícita, fechada y justificada en su propio `constitution.md`.

Aplica a proyectos en **Python**, **Go**, **Java con Quarkus o Spring Boot**, **Vite con TypeScript**, **SQLite**, **PostgreSQL** y cualquier combinación compatible.

---

## 0. Jerarquía normativa y actualización del catálogo

Las palabras **DEBE**, **NO DEBE**, **OBLIGATORIO**, **PROHIBIDO**, **RECOMENDADO** y **OPCIONAL** son normativas.

Orden de precedencia:

1. Seguridad, cumplimiento y requisitos regulatorios.
2. Esta constitución.
3. `constitution.md` del proyecto.
4. `docs/ARQUITECTURA.md` y registros de decisión arquitectónica.
5. Especificaciones de cada funcionalidad.

### Política Context7

Context7 se usa para consultar documentación vigente y ejemplos compatibles, pero **no es un escáner de vulnerabilidades ni reemplaza las fuentes oficiales**. Para cada dependencia nueva o actualización:

1. Resolver el identificador exacto de la librería en Context7.
2. Consultar documentación correspondiente a la versión candidata.
3. Confirmar la versión estable en la fuente oficial del proyecto o registro del ecosistema.
4. Verificar compatibilidad con el runtime y con el BOM o archivo de bloqueo.
5. Ejecutar el escáner de vulnerabilidades del ecosistema.
6. Registrar la decisión y la fecha de consulta en el pull request.

No se permite citar `latest` como versión. La referencia reproducible es una versión estable concreta, un archivo de bloqueo y, para imágenes, preferiblemente un digest.

### Cadencia

- Renovar este catálogo al menos una vez por trimestre.
- Renovarlo antes si aparece una vulnerabilidad crítica, termina el soporte de una versión o cambia una versión LTS.
- No adoptar versiones alpha, beta, milestone, release candidate, early access o preview en producción.
- Un proyecto existente actualiza dependencias mediante un cambio separado, con pruebas y plan de reversión.

---

## 1. Stack por lenguaje

Las versiones de esta sección son la línea base estable a la fecha de corte. Los parches se actualizan mediante herramientas automáticas y siempre pasan por CI.

### 1.1 Python y FastAPI

| Capa | Tecnología | Línea base |
|---|---|---:|
| Runtime | Python | 3.13.x |
| API web | FastAPI | 0.141.x |
| Servidor ASGI | Uvicorn | versión estable compatible |
| Validación | Pydantic | 2.x estable |
| Configuración | pydantic-settings | 2.x estable |
| PostgreSQL | psycopg 3 con interfaz async | estable |
| SQLite | aiosqlite | estable |
| SQL opcional | SQLAlchemy Core asyncio | 2.0.x, sin ORM por defecto |
| Pruebas | pytest + pytest-asyncio | estable |
| Cliente de pruebas | HTTPX | estable |
| Linter y formato | Ruff | estable |
| Tipos | mypy | estable, modo estricto gradual |
| Gestión | uv | estable, con `uv.lock` |
| Cobertura | coverage.py o pytest-cov | estable |

Reglas:

- Código compatible con la versión mínima declarada en `pyproject.toml`.
- `async` se usa en límites de I/O. No debe envolverse código síncrono en `async` sin necesidad.
- Una operación bloqueante no se ejecuta en el event loop.
- Ruff define longitud máxima de 100 caracteres.
- `pyproject.toml` es la fuente única de configuración. No se mantiene `requirements.txt` manual si se usa `uv.lock`.
- Pydantic se limita a DTO, validación de bordes y configuración. El dominio utiliza tipos propios o `dataclass` cuando sea suficiente.

### 1.2 Go

| Capa | Tecnología | Línea base |
|---|---|---:|
| Runtime | Go | 1.27.x |
| HTTP | `net/http` o Chi | biblioteca estándar primero |
| PostgreSQL | pgx v5 | estable |
| SQLite | `database/sql` + driver mantenido | estable |
| Consultas tipadas | sqlc | opcional y recomendado |
| Migraciones | golang-migrate | estable |
| Pruebas | `testing` + Testify | estable |
| Linter | golangci-lint | estable compatible con Go |
| Seguridad | govulncheck | última estable |
| Formato | gofmt + gofumpt | obligatorio |

Reglas:

- El `go.mod` declara la versión mínima de Go.
- Se prefiere biblioteca estándar antes de agregar una dependencia.
- Todo I/O acepta `context.Context` como primer parámetro.
- No se guardan contextos dentro de estructuras.
- Los errores se envuelven con contexto usando `%w`.

### 1.3 Java con Quarkus o Spring Boot

Cada servicio Java elige **un solo framework**. No se mezclan Quarkus y Spring Boot en el mismo artefacto.

| Capa | Quarkus | Spring Boot |
|---|---|---|
| Runtime | Java 25 LTS | Java 25 LTS |
| Framework | Quarkus 3.33 LTS | Spring Boot 4.1.x |
| API | Quarkus REST | Spring Web MVC; WebFlux solo si el flujo completo es reactivo |
| Validación | Jakarta Validation | Jakarta Validation |
| Persistencia predeterminada | JDBC directo o jOOQ | `JdbcClient`/JDBC directo o jOOQ |
| PostgreSQL | extensión JDBC oficial | driver PostgreSQL administrado por BOM |
| SQLite | driver JDBC mantenido | driver JDBC mantenido |
| Migraciones | Flyway con SQL versionado | Flyway con SQL versionado |
| Pruebas | JUnit 5 + REST Assured + Testcontainers | JUnit 5 + Spring Boot Test + Testcontainers |
| Calidad | Spotless + Error Prone o equivalente | Spotless + Error Prone o equivalente |
| Seguridad | OWASP Dependency-Check o SCA corporativo | OWASP Dependency-Check o SCA corporativo |
| Build | Maven Wrapper | Maven Wrapper |

Reglas Java:

- Maven Wrapper es obligatorio y se versiona. Gradle solo se acepta mediante decisión arquitectónica.
- El BOM del framework gobierna versiones transitivas. No se sobrescriben versiones administradas sin justificación.
- Se usa `record` para DTO inmutables cuando aplique.
- El dominio no depende de anotaciones HTTP, JPA, CDI o Spring.
- Se prohíbe Lombok en proyectos nuevos. Si un proyecto existente lo usa, debe documentar su continuidad.
- No se usa JPA/Hibernate por defecto. ORM requiere justificación en `docs/ARQUITECTURA.md`.
- Quarkus usa CDI para integración del framework, pero el composition root conserva el control explícito del grafo de aplicación.
- Spring Boot usa configuración explícita con métodos `@Bean`. Se evita component scanning sobre dominio y aplicación.
- Compilación nativa es opcional. Si se habilita, CI debe probar JVM y nativo.

### 1.4 Vite y TypeScript

| Capa | Tecnología | Línea base |
|---|---|---:|
| Runtime de herramientas | Node.js LTS | línea LTS activa |
| Gestor | pnpm | estable, con lockfile |
| UI predeterminada | React | 19.2.x |
| Build | Vite | 8.3.x |
| Lenguaje | TypeScript | 6.x estable |
| Pruebas | Vitest | 4.1.x o parche compatible |
| Componentes | Testing Library | estable |
| Linter | ESLint con configuración plana | estable |
| Formato | Prettier | estable |
| CSS | Tailwind CSS | 4.x estable |

Reglas:

- `strict: true`, `noUncheckedIndexedAccess: true` y `exactOptionalPropertyTypes: true`.
- Vite transpila, pero no reemplaza la verificación de tipos. CI ejecuta `tsc --noEmit`.
- Se prohíbe `any` explícito salvo excepción localizada y comentada.
- Se usa un solo gestor de paquetes y un solo lockfile.
- No se admiten pre-releases en producción.

### 1.5 Persistencia

| Motor | Uso predeterminado | Regla |
|---|---|---|
| SQLite | prototipos, desarrollo y servicios de una sola instancia | no se usa para escrituras concurrentes de múltiples réplicas |
| PostgreSQL 18.x | producción, concurrencia y múltiples instancias | versión exacta de parche en imagen y mantenimiento periódico |
| Migraciones | SQL versionado mediante runner mantenido | no ejecutar migraciones destructivas automáticamente al arrancar |

---

## 2. Contenerización y entornos aislados

Cada proyecto se ejecuta en su propio stack. `network_mode: host` está prohibido.

```text
{project}/
├── Dockerfile
├── compose.yaml
├── .dockerignore
└── caddy/
    └── Caddyfile
```

Reglas:

- `compose.yaml` no declara el campo obsoleto `version`.
- Red privada `{project}_network` con driver `bridge`.
- Comunicación por nombre de servicio, nunca por IP fija.
- Solo se publican al host los puertos necesarios.
- Datos persistentes en volúmenes nombrados con prefijo del proyecto.
- Bind mounts solo para desarrollo.
- Configuración no secreta mediante `.env`; secretos de producción mediante el gestor de secretos de la plataforma.
- Las imágenes usan build multi-stage, usuario no root, sistema de archivos de solo lectura cuando sea viable y capacidades Linux mínimas.
- Healthcheck obligatorio. `depends_on` no sustituye reintentos con backoff en la aplicación.
- Si la imagen de runtime no incluye `wget` ni `curl` (por ejemplo, una imagen distroless), el healthcheck usa un binario o subcomando propio del servicio en lugar de añadir utilidades a la imagen.
- Las imágenes se fijan por versión de parche y digest en producción.
- Se genera SBOM y se firma la imagen cuando la plataforma lo permita.

Ejemplo:

```yaml
name: ${COMPOSE_PROJECT_NAME}

services:
  app:
    build:
      context: .
      target: runtime
    ports:
      - "${APP_PORT}:8080"
    env_file:
      - .env
    networks:
      - app_network
    depends_on:
      db:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health/ready"]
      interval: 10s
      timeout: 3s
      retries: 5
      start_period: 20s

  db:
    image: postgres:18.6-alpine3.24
    env_file:
      - .env
    networks:
      - app_network
    volumes:
      - db_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $$POSTGRES_USER -d $$POSTGRES_DB"]
      interval: 5s
      timeout: 3s
      retries: 10
      start_period: 10s

networks:
  app_network:
    driver: bridge

volumes:
  db_data:
```

`.dockerignore` excluye como mínimo `.git/`, `.env*`, caches, artefactos, entornos virtuales, `node_modules/`, reportes, pruebas y documentación no requerida en runtime. Se permite incluir `.env.example`.

---

## 3. Arquitectura hexagonal obligatoria

```text
src/{project}/
├── domain/
├── application/
│   └── ports/
├── adapters/
└── infrastructure/
```

Para Java se usa la misma separación bajo `src/main/java/{basePackage}/`. Para Go puede aplicarse bajo `internal/`.

| Capa | Puede depender de | No puede depender de |
|---|---|---|
| `domain` | runtime y tipos propios | HTTP, DB, mensajería, framework, Pydantic, Jakarta, Spring, Quarkus |
| `application` | `domain` y ports propios | adapters e infrastructure |
| `adapters` | `application`, `domain` y cliente externo | presentación HTTP concreta |
| `infrastructure` | todas | ninguna capa superior adicional |

Reglas:

- Toda dependencia externa se representa mediante un port en `application/ports/`.
- `{Name}Port` para servicios y `{Name}Repository` para persistencia.
- Un repositorio no expone conexión, sesión, cursor, entidad ORM ni tipo del driver.
- Para ausencia esperada se usa `None`, `nil`, `Optional` o un tipo resultado explícito. No se lanza una excepción de infraestructura.
- Las reglas se verifican en CI con import-linter en Python, ArchUnit en Java y reglas de paquetes en Go.
- Pydantic, Jakarta Validation y schemas HTTP pertenecen al borde, no al dominio.

---

## 4. Composition root

El wiring ocurre en un único módulo de infraestructura.

- Python: `infrastructure/composition.py`.
- Go: `infrastructure/composition.go`.
- Java: `infrastructure/configuration/` con configuración explícita.

Reglas:

- Una función o configuración raíz construye el grafo completo.
- El código de aplicación recibe dependencias por constructor.
- No se usa un service locator ni acceso global mutable.
- FastAPI `Depends`, CDI y Spring DI se limitan al borde y no aparecen en dominio.
- Los mocks no se habilitan en producción. Su activación exige un perfil de desarrollo o pruebas y un guard que impida arrancar con ellos en producción.
- No se seleccionan adaptadores con condicionales dispersos. La selección vive en el composition root.

---

## 5. Persistencia y transacciones

- SQL parametrizado obligatorio. Se prohíbe concatenar valores.
- SQL directo es el valor predeterminado. Se permiten SQLAlchemy Core, jOOQ o sqlc para tipado y composición sin adoptar ORM.
- ORM exige una decisión arquitectónica con impacto en rendimiento, pruebas, migraciones y límites de dominio.
- Las transacciones se controlan en el caso de uso cuando abarquen más de una operación de repositorio, mediante un port `UnitOfWork`.
- Cada request obtiene su propia conexión o transacción del pool. No se abre una conexión física nueva por cada consulta.
- Escrituras con `commit` explícito y rollback seguro.
- Toda consulta tiene timeout configurable.
- Los repositorios convierten tipos del driver a tipos de dominio.
- Se prohíbe `SELECT *` en código de producción.
- Se revisan índices y planes para consultas críticas.

---

## 6. Migraciones

```text
migrations/
├── V001__initial_schema.sql
├── V002__add_users.sql
└── R__refresh_views.sql
```

Reglas:

- Se usa la convención del runner seleccionado. No se mezcla numeración propia con Flyway o golang-migrate.
- Una migración aplicada es inmutable. Una corrección crea una migración nueva.
- `IF NOT EXISTS` no convierte automáticamente una migración en segura. Se valida el estado resultante.
- Cambios destructivos siguen expandir, migrar y contraer.
- Producción ejecuta migraciones como job separado antes del rollout, con backup y plan de reversión.
- El startup puede migrar solo en desarrollo y pruebas.
- CI prueba desde una base vacía y desde la versión anterior soportada.

---

## 7. API HTTP

Estructura orientativa:

```text
infrastructure/http/
├── app
├── auth
├── middleware
├── errors
└── routes/
```

Reglas:

- Rutas y controladores solo validan, autentican, invocan casos de uso y transforman respuestas.
- Sin lógica de negocio ni SQL en controladores.
- OpenAPI explícito y validado en CI.
- Prefijo versionado: `/api/v1`.
- Identificadores de correlación propagados en logs y llamadas salientes.
- Timeouts, tamaño máximo de body, CORS y rate limiting se configuran explícitamente.
- Endpoints de salud separados: liveness y readiness.
- Errores usan `application/problem+json` conforme a RFC 9457 cuando sea viable.
- No se filtran stack traces, sentencias SQL, tokens ni datos sensibles.

---

## 8. Manejo de errores y resiliencia

- Errores esperados de dominio se modelan con tipos explícitos.
- Errores inesperados conservan causa y contexto, y se traducen una sola vez en el borde.
- Códigos HTTP: 400 para solicitud mal formada (sintaxis o cuerpo ilegible), 401 para falta de autenticación, 403 para falta de autorización, 404 para ausencia, 409 para conflicto con el estado actual y 422 para contenido bien formado que no supera la validación (esquema o reglas de entrada).
- Retries solo para operaciones idempotentes y fallos transitorios, con backoff exponencial, jitter y límite.
- Toda llamada externa tiene timeout.
- Fire-and-forget no significa perder trabajo: tareas críticas o que requieren garantía usan cola u outbox transaccional.
- Notificaciones no críticas pueden fallar sin bloquear, pero dejan métrica y log estructurado.
- Escrituras reintentables exigen clave de idempotencia o restricción equivalente.

---

## 9. Configuración y secretos

- Una clase o estructura raíz de configuración, agrupada por dominio cuando sea necesario.
- Configuración desde variables de entorno o archivos montados por la plataforma.
- `.env` solo para desarrollo y nunca se versiona.
- `.env.example` documenta propósito, ejemplo seguro, obligatoriedad y unidad.
- Secretos no tienen valor predeterminado y nunca aparecen en logs.
- Valores operativos seguros pueden tener defaults documentados. Credenciales, hosts externos y claves no.
- Configuración validada al inicio. La aplicación falla rápido ante ausencia o formato inválido.
- Timeouts, límites y flags se tipan, no se leen repetidamente como strings.
- Producción usa un gestor de secretos. Rotación sin reconstruir la imagen cuando lo permita la plataforma.

---

## 10. Pruebas

```text
tests/
├── unit/
├── integration/
├── contract/
└── architecture/
```

- Python: pytest.
- Go: `go test` y Testify donde aporte claridad. Las pruebas unitarias viven junto al código (`*_test.go`), como exige la herramienta; `tests/` se reserva para integración, contrato y arquitectura.
- Java: JUnit 5, AssertJ, REST Assured y Testcontainers.
- Frontend: Vitest y Testing Library.

Reglas:

- Cobertura mínima global de 80 %, sin reducir la línea base del repositorio.
- Cobertura no sustituye pruebas de comportamiento.
- Dominio y aplicación se prueban sin framework ni red.
- Repositorios se prueban contra el motor real soportado mediante contenedores, no solo mocks.
- Contratos externos se prueban con stubs controlados o pruebas de consumidor/proveedor.
- No se modifican pruebas existentes solo para hacer pasar una implementación. Un cambio legítimo de comportamiento actualiza especificación y prueba en el mismo pull request.
- Pruebas deterministas: sin dependencia del reloj real, orden, red pública o datos compartidos.
- Reloj, generador de UUID y aleatoriedad se inyectan cuando afectan comportamiento.
- CI bloquea tests ignorados sin ticket y fecha de vencimiento.

---

## 11. Adaptadores mock y noop

- Cada port externo debe poder sustituirse en pruebas mediante fake, stub o mock.
- No es obligatorio mantener una implementación mock en producción si un fake de pruebas cubre el contrato.
- Los fakes viven en `tests/fakes/` o en un módulo de soporte de pruebas.
- Noop se utiliza solo para comportamiento realmente opcional.
- Activar un mock mediante variable de entorno en producción está prohibido.
- Los mocks no replican detalles internos del proveedor. Representan el contrato del port.

---

## 12. Observabilidad

Todo servicio debe exponer:

- Logs estructurados JSON en producción.
- Métricas de tasa, errores, duración y saturación.
- Trazas distribuidas con OpenTelemetry para límites HTTP, DB, colas y clientes externos.
- `trace_id` y `correlation_id` en logs.
- Redacción de secretos y datos personales.
- Endpoint de métricas protegido o aislado de la red pública.
- Alertas basadas en SLO, no solo en consumo de recursos.

No se registran bodies completos, credenciales, cookies, tokens ni datos sensibles por defecto.

---

## 13. Cache

- Cache-aside para lectura es opcional.
- TTL predeterminado de 300 segundos, configurable.
- La clave incluye versión de esquema y tenant cuando aplique.
- Escrituras invalidan o actualizan explícitamente.
- Fallo de cache degrada a la fuente de verdad cuando sea seguro.
- Se evita cache local mutable en despliegues con múltiples réplicas.
- Se previene cache stampede con jitter, single-flight o bloqueo distribuido según escala.
- Nunca se cachean secretos, decisiones de autorización sin TTL estricto ni datos personales sin análisis.

---

## 14. Frontend

```text
src/
├── api/
├── components/
├── features/
├── hooks/
├── pages/
├── stores/
├── types/
└── utils/
```

Reglas:

- Organización por feature cuando crece el producto.
- Componentes de presentación sin reglas de negocio.
- Estado global solo para estado realmente compartido. Estado del servidor mediante una solución de consulta/cache justificada.
- Clientes HTTP centralizados, tipados y con cancelación.
- Contratos generados desde OpenAPI cuando reduzcan duplicación.
- Accesibilidad WCAG 2.2 AA como objetivo mínimo.
- Pruebas por comportamiento y rol accesible, no por estructura interna.
- Presupuesto de bundle y auditoría de dependencias en CI.
- No se exponen secretos en variables de build del navegador.

---

## 15. Seguridad de aplicación y cadena de suministro

### Dependencias

- Python: `uv lock`, `pip-audit` y hashes reproducibles.
- Go: `go mod tidy`, `govulncheck` y verificación de módulos.
- Java: BOM del framework, Maven Enforcer, OWASP Dependency-Check o SCA corporativo.
- TypeScript: `pnpm-lock.yaml`, `pnpm audit` y una herramienta SCA.
- Renovate o Dependabot crea actualizaciones pequeñas y periódicas.
- CI falla por vulnerabilidades críticas o altas con solución disponible.
- Una excepción incluye CVE, componente, versión, exposición, compensación, responsable y fecha máxima de revisión.

### Imágenes y artefactos

- Sin tag `latest`.
- Versión de parche y digest en producción.
- Trivy, Grype, Docker Scout o escáner corporativo en CI.
- SBOM en CycloneDX o SPDX.
- Usuario no root y mínimo contenido en runtime.
- Artefactos construidos una vez y promovidos entre ambientes.
- Firma y verificación con Sigstore/Cosign cuando la plataforma lo soporte.

### Reglas generales

- Autenticación y autorización se verifican en el servidor.
- Principio de mínimo privilegio para DB, red, archivos y cloud.
- TLS en tránsito y cifrado en reposo según clasificación.
- Validación de entrada en el borde y encoding de salida según contexto.
- Protección contra SSRF, inyección, path traversal y deserialización insegura.
- No se implementa criptografía propia.

---

## 16. Calidad, estilo y commits

### Convenciones

| Elemento | Python | Go | Java | TypeScript |
|---|---|---|---|---|
| Módulo/archivo | `snake_case` | `snake_case.go` | clase `PascalCase` | `kebab-case.ts` |
| Tipo/clase | `PascalCase` | `PascalCase` | `PascalCase` | `PascalCase` |
| Función/método | `snake_case` | `PascalCase` exportado | `camelCase` | `camelCase` |
| Constante | `UPPER_SNAKE_CASE` | nombre idiomático | `UPPER_SNAKE_CASE` | `UPPER_SNAKE_CASE` |

- Código, identificadores, mensajes técnicos internos y commits en inglés.
- Documentación, especificaciones, planes y tareas en español latino.
- Python y TypeScript: 100 caracteres.
- Java: 120 caracteres y formato automatizado.
- Go: gofmt/gofumpt; no se impone ancho manual incompatible con el formatter.
- Conventional Commits es recomendado.
- Comentarios explican el porqué, no repiten el código.

---

## 17. CI/CD y puertas de calidad

Orden mínimo del pipeline:

1. Validar formato y lint.
2. Verificar tipos o compilación.
3. Ejecutar pruebas unitarias.
4. Ejecutar pruebas de arquitectura.
5. Ejecutar pruebas de integración y contrato.
6. Verificar cobertura.
7. Escanear secretos, dependencias y código.
8. Construir imagen una sola vez.
9. Escanear imagen y generar SBOM.
10. Publicar artefacto inmutable.
11. Desplegar con estrategia y rollback documentados.
12. Ejecutar smoke tests y verificación de salud.

No se permite saltar controles por cambios de documentación si el pipeline afectado no distingue archivos de forma segura. Producción requiere revisión, trazabilidad y separación de funciones de acuerdo con la política organizacional.

---

## 18. Documentación y especificaciones

Archivos obligatorios:

| Archivo | Propósito |
|---|---|
| `README.md` | propósito, stack, requisitos, arranque y API pública |
| `constitution.md` | extensiones y excepciones del proyecto |
| `docs/ARQUITECTURA.md` | capas, flujos, límites y decisiones |
| `docs/DATABASE.md` | esquema, migraciones, backup y mantenimiento |
| `docs/DEPENDENCIAS.md` | excepciones, CVE y revisiones |
| `docs/OPERACION.md` | despliegue, observabilidad, SLO y recuperación |
| `docs/adr/` | decisiones arquitectónicas fechadas |

Por feature:

```text
specs/{NNN}-{name}/
├── spec.md
├── plan.md
├── tasks.md
├── checklists/
│   └── requirements.md
├── contracts/
├── data-model.md
├── research.md
└── quickstart.md
```

`spec.md`, `plan.md`, `tasks.md` y `checklists/requirements.md` son obligatorios. Los demás dependen del alcance.

Reglas de idioma:

- Narrativa en español latino, clara y en voz activa.
- Identificadores de código en inglés.
- Evitar spanglish cuando exista traducción natural.
- Un término técnico sin traducción clara se explica la primera vez.
- Tareas redactadas como acciones verificables.

---

## 19. Estructura de proyecto

```text
{project}/
├── .env.example
├── .gitignore
├── .dockerignore
├── AGENTS.md
├── README.md
├── constitution.md
├── compose.yaml
├── Dockerfile
├── pyproject.toml | go.mod | pom.xml | package.json
├── uv.lock | go.sum | pom.xml | pnpm-lock.yaml
├── migrations/
├── docs/
│   ├── ARQUITECTURA.md
│   ├── DATABASE.md
│   ├── DEPENDENCIAS.md
│   ├── OPERACION.md
│   └── adr/
├── specs/
├── src/ | internal/ | src/main/java/
├── tests/ | src/test/java/
└── static/
```

`CLAUDE.md`, `COPILOT.md` u otros archivos específicos de agente son opcionales. `AGENTS.md` es el archivo neutral recomendado y no puede contradecir esta constitución.

---

## 20. Prohibiciones absolutas

- Importar adapters o infrastructure desde domain o application.
- Exponer conexiones, sesiones, cursores, entidades ORM o tipos de driver desde repositorios.
- Concatenar valores en SQL.
- Poner lógica de negocio o SQL en rutas, controladores o handlers.
- Usar ORM sin decisión arquitectónica.
- Compartir una transacción mutable entre requests.
- Ejecutar I/O bloqueante en el event loop.
- Guardar secretos o `.env` en Git.
- Hardcodear credenciales, URLs externas, puertos de despliegue o límites operativos dependientes del entorno.
- Usar `network_mode: host`.
- Usar `latest` o una pre-release en producción.
- Añadir dependencia sin necesidad, consulta documental, verificación oficial y escaneo.
- Ignorar una vulnerabilidad alta o crítica sin excepción vigente.
- Desactivar pruebas, linters o scanners para aprobar un pull request.
- Habilitar mocks o endpoints de depuración en producción.
- Registrar tokens, credenciales o datos personales sin control explícito.
- Ejecutar migraciones destructivas automáticamente al arrancar producción.
- Mezclar Quarkus y Spring Boot en el mismo servicio.
- Introducir un segundo gestor de paquetes o lockfile.
- Redactar documentación técnica del proyecto en inglés, salvo contenido generado por herramientas o citas externas.

---

## 21. Principios operativos

1. **Simplicidad primero:** resolver el problema con el menor número razonable de piezas.
2. **Causa raíz:** no aceptar parches permanentes que oculten el problema.
3. **Contratos explícitos:** tipos, ports, OpenAPI y esquemas son verificables.
4. **Pruebas tempranas:** TDD cuando sea práctico y siempre pruebas con el cambio.
5. **Fallar rápido:** validar configuración y entradas en el borde.
6. **Idempotencia:** toda escritura reintentable debe ser segura.
7. **Observabilidad por diseño:** logs, métricas y trazas forman parte de la funcionalidad.
8. **Seguridad por defecto:** mínimo privilegio, dependencias mínimas y secretos fuera del código.
9. **Automatización reproducible:** wrappers, lockfiles, imágenes inmutables y pipelines versionados.
10. **Despliegue seguro:** migración compatible, healthchecks, rollout gradual y rollback.
11. **Documentar decisiones:** toda excepción tiene contexto, responsable y fecha de revisión.
12. **Aislamiento:** cada proyecto opera en su red, volúmenes y límites propios.

---

## 22. Matriz resumida

| Capa | Python | Go | Java Quarkus | Java Spring Boot | TypeScript |
|---|---|---|---|---|---|
| Runtime | Python 3.13 | Go 1.27 | Java 25 LTS | Java 25 LTS | Node.js LTS |
| Web | FastAPI 0.141 | net/http o Chi | Quarkus REST 3.33 LTS | Spring Boot 4.1 | Vite 8.3 |
| DB | psycopg 3 / aiosqlite | pgx v5 / database/sql | JDBC o jOOQ | JdbcClient/JDBC o jOOQ | cliente HTTP tipado |
| Pruebas | pytest | testing | JUnit 5 | JUnit 5 | Vitest 4.1 |
| Calidad | Ruff + mypy | golangci-lint | Spotless + análisis estático | Spotless + análisis estático | ESLint + tsc |
| Vulnerabilidades | pip-audit | govulncheck | Dependency-Check o SCA | Dependency-Check o SCA | pnpm audit o SCA |
| Migraciones | runner SQL | golang-migrate | Flyway | Flyway | no aplica |

---

## 23. Gobierno de excepciones

Toda relajación crea una sección en `constitution.md` del proyecto con:

- regla afectada;
- motivo y alternativas descartadas;
- riesgo técnico y de seguridad;
- controles compensatorios;
- responsable;
- fecha de aprobación;
- fecha de vencimiento o revisión;
- enlace al ADR y al ticket.

Una excepción vencida bloquea el merge hasta renovarse o eliminarse.

---

## 24. Fuentes de referencia del catálogo

Consultadas el 24 de septiembre de 2026:

- Quarkus releases: https://quarkus.io/releases/
- Spring Boot: https://spring.io/projects/spring-boot
- Vite releases: https://vite.dev/releases
- Vite 8: https://vite.dev/blog/announcing-vite8
- Vitest 4.1: https://vitest.dev/blog/vitest-4-1.html
- FastAPI release notes: https://fastapi.tiangolo.com/release-notes/
- PostgreSQL: https://www.postgresql.org/
- PostgreSQL Official Image: https://hub.docker.com/_/postgres
- Go Official Image: https://hub.docker.com/_/golang
- Context7: consultar el identificador y la documentación de cada librería durante la implementación.

> Las versiones exactas de parche deben confirmarse de nuevo al crear el proyecto. Este documento fija líneas base, no sustituye el lockfile, el BOM, el escáner SCA ni la política de soporte oficial.
