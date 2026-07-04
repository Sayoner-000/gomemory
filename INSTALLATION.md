# Instalación de gomemory v1.11.0

> Repositorio: [github.com/Sayoner-000/gomemory](https://github.com/Sayoner-000/gomemory)

---

## 0. Instalación universal (recomendada)

Un solo comando descarga el binario `mem` y lo deja en el PATH. No requiere Go,
Git ni compilar. Funciona en Linux, macOS y Windows.

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.sh | bash
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.ps1 | iex
```

Variables opcionales (Linux/macOS): `GOMEMORY_VERSION=v1.6.0` para fijar versión,
`GOMEMORY_BIN_DIR=/usr/local/bin` para elegir el destino.

Desinstalar el binario: `curl -fsSL .../install.sh | bash -s -- --uninstall`.

Tras instalar, sigue con **0.1 Registro global** (recomendado, sin pasos por proyecto)
o salta a la sección **4. Instalar en un proyecto** (el flujo clásico, todavía soportado).
Las secciones 1–3 son solo para compilar desde el fuente.

---

## 0.1 Registro global (recomendado — sin instalar por proyecto)

**Claude Code, Codex y OpenCode no necesitan `mem install` por repositorio**.
Con el binario `mem` en el PATH, un solo comando registra gomemory a nivel de
usuario, disponible automáticamente en cualquier proyecto:

```bash
mem setup-mcp --scope global --agents claude,codex,opencode
```

- Para OpenCode, este mismo comando instala también el plugin
  (`~/.config/opencode/plugins/gomemory.ts`) además de registrar el MCP en
  `~/.config/opencode/opencode.json` — no hace falta un paso aparte.
- El store de memoria (`mem.db`) se crea solo, por proyecto, en el primer
  `mem save`/`mem mcp` — no hace falta `mem init` previo.
- La identidad de cada proyecto se deriva de su raíz de git (o del directorio
  actual si no hay `.git`), no del nombre de la carpeta — dos proyectos
  distintos nunca comparten memoria aunque se llamen igual.
- Si ya tenías un proyecto instalado a la manera antigua (`.memory/mem.db`
  dentro del repo), su historial se migra solo al primer uso, o de forma
  explícita con `mem migrate`.
- Cursor, Windsurf y Cline todavía no tienen un mecanismo de registro MCP a
  nivel de usuario conocido — siguen requiriendo `mem setup-mcp --scope
  project --agents cursor,windsurf,cline --target <dir>` por repositorio.

Si `mem setup-mcp --scope global` reporta una colisión de nombre (ya existe
una entrada `gomemory` global de otra herramienta), resuélvela manualmente
antes de reintentar — gomemory nunca sobrescribe esa entrada en silencio.

---

## Prerrequisitos

| Recurso | Requerido | Notas |
|---------|-----------|-------|
| **Go** 1.25+ | Para compilar desde fuente | `go version` para verificar |
| **Git** | Para clonar el repo | `git --version` para verificar |
| CGO | No | `modernc.org/sqlite` = SQLite puro en Go |
| Dependencias runtime | No | Binario autocontenido (~16MB) |

---

## 1. Descargar

```bash
git clone https://github.com/Sayoner-000/gomemory.git
cd gomemory
```

---

## 2. Compilar

```bash
go build -o mem ./infrastructure/
```

Esto produce el binario `mem` en el directorio actual.

### Compilar para otra plataforma

```bash
# macOS Apple Silicon
GOOS=darwin  GOARCH=arm64 go build -o mem-darwin-arm64 ./infrastructure/

# macOS Intel
GOOS=darwin  GOARCH=amd64 go build -o mem-darwin-amd64 ./infrastructure/

# Linux
GOOS=linux   GOARCH=amd64 go build -o mem-linux-amd64 ./infrastructure/

# Windows
GOOS=windows GOARCH=amd64 go build -o mem-windows-amd64.exe ./infrastructure/
```

No se necesita toolchain adicional — el cross-compile funciona out of the box
porque todo el árbol de dependencias es Go puro.

---

## 3. Verificar

```bash
./mem --help
```

Deberías ver la lista de comandos disponibles.

```bash
go vet ./...
go test ./... -v
```

Todos los tests deben pasar.

---

## 4. Instalar en un proyecto (flujo clásico, opcional)

> Para Claude Code/Codex/OpenCode, la sección **0.1 Registro global** reemplaza
> este paso — regístralo una vez y listo para todos tus proyectos. Esta
> sección sigue siendo necesaria para Cursor, Windsurf y Cline (registro por
> proyecto), o si prefieres seguir usando el flujo con binario copiado y
> `AGENTS.md`/`CLAUDE.md` generados.

```bash
# Desde el directorio de gomemory
./mem install /ruta/a/tu/proyecto
```

Esto crea automáticamente en el proyecto destino:

```
proyecto/
├── .memory/               # Base de datos SQLite (gitignorada)
│   ├── mem.db
│   └── context.md
├── AGENTS.md              # Instrucciones de integración
├── CLAUDE.md              # Instrucciones para Claude Code
├── opencode.json           # MCP para OpenCode
├── .mcp.json              # MCP para Claude
├── .cursor/mcp.json       # MCP para Cursor
├── .windsurf/mcp_config.json
├── .cline/mcp_settings.json
├── mem                    # Binario (gitignorado)
└── .gitignore
```

---

## 5. Plugins multi-agente

Los plugins inyectan memoria automáticamente en cada inferencia del agente,
sin necesidad de invocar herramientas MCP manualmente.

### OpenCode

```bash
./mem setup opencode
```

Instala en `~/.config/opencode/plugins/gomemory/plugin.ts`.

**Qué hace**:
- Inicia `mem serve` en background automáticamente
- Crea sesión al iniciar, la cierra al terminar
- Inyecta el Memory Protocol en el system prompt
- Provee contexto de sesiones previas
- Recupera estado después de compactación

**Reinicia OpenCode** para activarlo.

### Claude Code

```bash
mem setup claude-code
```

Configura hooks portables en `.claude/settings.json` e instala el skill en
`.claude/plugins/gomemory/`. Los hooks son subcomandos del binario
(`mem hook <evento>`) — sin `bash`/`curl` ni servidor HTTP, y funcionan en Windows.

**Qué hace**:
- Crea sesión al iniciar (`SessionStart` → `mem hook session-start`)
- Cierra sesión al terminar (`SessionEnd` → `mem hook session-end`)
- Inyecta contexto + recordatorio del protocolo en cada prompt y tras compactación
- Skill de memoria siempre disponible

Los hooks referencian `mem` por PATH (o `${CLAUDE_PROJECT_DIR}/mem` como fallback
por-proyecto), nunca una ruta absoluta de máquina: la config es portable entre
equipos y SO.

**Reinicia Claude Code** para activarlo.

### Verificar instalación de plugins

```bash
# OpenCode
ls ~/.config/opencode/plugins/gomemory/

# Claude Code
ls .claude/plugins/gomemory/scripts/

# Healthcheck del servidor HTTP (auto-iniciado por plugins)
curl http://127.0.0.1:9735/health
```

---

## 6. Servidor HTTP (manual)

El servidor HTTP es auto-iniciado por los plugins, pero también puedes
iniciarlo manualmente:

```bash
./mem serve                # Puerto default 9735
./mem serve --port 19735   # Puerto personalizado
```

Endpoints:

| Endpoint | Método | Descripción |
|----------|--------|-------------|
| `/health` | GET | Healthcheck |
| `/session/start` | POST | Crear sesión |
| `/session/end` | POST | Cerrar sesión con resumen |
| `/context` | GET | Contexto de sesiones previas |

---

## 7. MCP Server

El servidor MCP expone la memoria como herramientas nativas para cualquier
agente compatible con el Model Context Protocol.

```bash
# Desde el directorio del proyecto
./mem mcp --root .
```

Herramientas MCP disponibles:

| Herramienta | Descripción |
|---|---|
| `save_memory` | Guardar aprendizaje, decisión, bugfix o patrón |
| `search_memories` | Buscar en la memoria del proyecto |
| `list_memories` | Listar memorias recientes |
| `get_memory` | Obtener una memoria por ID |
| `forget_memory` | Borrar una memoria por ID (irreversible) |
| `judge_memories` | Registrar veredicto imparcial entre dos memorias |
| `start_session` | Iniciar sesión de trabajo |
| `end_session` | Finalizar sesión con resumen |
| `get_context` | Obtener contexto completo del proyecto |

Configuración multi-agente automática:

```bash
# Configurar todos los agentes detectados
./mem setup-mcp --agents all

# O agentes específicos
./mem setup-mcp --agents opencode,claude,cursor,codex
```

---

## 8. Uso básico

```bash
# TUI interactiva
./mem

# Guardar una memoria
./mem save -t "API REST con Fiber" -y decision "Usamos Fiber por su rendimiento"

# Buscar en la memoria
./mem search "API"

# Sesión de trabajo
./mem session start
# ... trabajar ...
./mem session end -s "Implementé autenticación JWT"

# Contexto completo
./mem context --write    # genera .memory/context.md
```

---

## 9. Actualizar

```bash
cd gomemory
git pull
go build -o mem ./infrastructure/
# Reemplazar el binario en cada proyecto donde esté instalado
cp mem /ruta/a/tu/proyecto/mem
# Reinstalar plugins si hubo cambios
./mem setup opencode
./mem setup claude-code
```

---

## Solución de problemas

### "command not found: go"

Instala Go desde [go.dev/dl](https://go.dev/dl/). Versión mínima: 1.25.

### "go: no such toolchain"

Actualiza Go a 1.25+: `go install golang.org/dl/go1.25@latest && go1.25 download`

### "plugin not found after setup"

```bash
# Verificar que el plugin se instaló en el directorio correcto
ls ~/.config/opencode/plugins/gomemory/
ls .claude/plugins/gomemory/scripts/

# Reinstalar
./mem setup opencode
./mem setup claude-code

# ¿Olvidaste reiniciar el agente? Los plugins se cargan al arranque.
```

### "address already in use"

```bash
# Puerto 9735 ocupado — usar otro puerto
./mem serve --port 19735
./mem setup --port 19735 opencode

# O matar el proceso anterior
lsof -i :9735
kill <PID>
```

### "MCP connection refused"

```bash
# El servidor HTTP debe estar corriendo
./mem serve &
curl http://127.0.0.1:9735/health
```

---

## Más información

| Documento | Descripción |
|-----------|-------------|
| [`docs/architecture.md`](docs/architecture.md) | Arquitectura completa |
| [`docs/PLUGINS.md`](docs/PLUGINS.md) | Sistema de plugins |
| [`docs/MEMORY-PROTOCOL.md`](docs/MEMORY-PROTOCOL.md) | Protocolo de memoria |
| [`docs/MANUAL.md`](docs/MANUAL.md) | Guía paso a paso |
| [`README.md`](README.md) | Features y descripción general |
