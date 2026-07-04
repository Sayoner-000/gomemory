# Research: Registro global de gomemory

Todas las incógnitas del Technical Context ya se resolvieron durante la sesión de planificación con el usuario, contrastando directamente contra el código de este repo y contra la configuración real del entorno de desarrollo. No quedan `NEEDS CLARIFICATION` pendientes.

## 1. Dónde vive la base de datos de un proyecto

**Decision**: Mover `mem.db` de `<repo>/.memory/mem.db` a un store global del usuario: `$XDG_DATA_HOME/gomemory/projects/<clave>/mem.db` (Linux/macOS, con fallback a `~/.local/share/gomemory` si `XDG_DATA_HOME` no está seteado) o `%LOCALAPPDATA%\gomemory\projects\<clave>\mem.db` (Windows).

**Rationale**: `.memory/` ya se gitignora en la instalación actual (`cmd_install.go` la añade a `.gitignore`) — nunca viajó con el repositorio. Moverla fuera del árbol de trabajo no cambia ningún contrato de datos existente, solo elimina el rastro físico dentro del proyecto (`git status` deja de mostrar nada relacionado con gomemory). Es el mismo patrón que ya usan herramientas de desarrollo comparables (caches de IDE, `direnv`, `asdf`) para estado por-proyecto que no debe versionarse.

**Alternatives considered**:
- Mantener `.memory/` dentro del repo y solo quitar el requisito de pre-instalación: rechazado porque no resuelve completamente el objetivo de "cero huella en el repo" que motivó la comparación con `codebase-memory-mcp`.
- Un único archivo SQLite global compartido por todos los proyectos (usando la columna `project` que ya soporta el esquema): rechazado porque reduce el aislamiento físico actual sin necesidad — el esquema ya soporta multi-proyecto en un archivo, pero un archivo por proyecto sigue siendo más simple de razonar, migrar y borrar de forma independiente.

## 2. Cómo se identifica un proyecto

**Decision**: La identidad de proyecto pasa de `filepath.Base(root)` (nombre de carpeta) a la ruta absoluta del git root (subiendo directorios desde el cwd buscando `.git`, igual que hoy `FindRoot()` sube buscando `.memory/`); si no hay `.git`, se usa el cwd absoluto. La clave de filesystem se deriva como slug legible (últimos segmentos de la ruta) + hash corto (`sha256[:8]` de la ruta completa) para evitar colisiones.

**Rationale**: `filepath.Base(root)` ya es ambiguo hoy (dos repos con la misma carpeta final colisionarían si compartieran árbol de directorios), pero el impacto era invisible porque cada proyecto tenía su propio archivo `.memory/mem.db` físicamente separado. En un store global compartido por todos los proyectos de la máquina, esa ambigüedad se vuelve un bug real de mezcla de memorias entre proyectos distintos.

**Alternatives considered**:
- URL del remote de git (`origin`): sobrevive a mover/renombrar la carpeta local, pero falla o requiere fallback en repos sin remote configurado (caso común en proyectos nuevos o puramente locales) — rechazada por el usuario explícitamente por esta razón.
- Ruta absoluta sin hash (solo sanitizada): rechazada porque nombres de proyecto muy largos o con caracteres especiales podrían producir nombres de directorio inválidos o inmanejables; el hash acota la longitud de forma determinista.

## 3. Cómo se registra el servidor MCP a nivel global por agente

**Decision**: Empezar por Claude Code, usando el mecanismo de scope de usuario ya soportado nativamente (confirmado en este mismo entorno: `~/.claude.json` tiene una clave top-level `mcpServers` que aplica a todos los proyectos, sin importar el cwd — es el mecanismo que ya usa `codebase-memory-mcp` para estar disponible sin instalación). Codex ya usa un archivo global (`~/.codex/config.toml`) y solo necesita simplificar su tabla a una entrada sin `cwd` por proyecto. OpenCode, Cursor, Windsurf y Cline se evalúan caso por caso (ver Paso 3 del plan original); donde no exista soporte de scope de usuario, se mantiene el registro por-proyecto actual como fallback, sin regresión.

**Rationale**: Es el mecanismo ya verificado empíricamente en la máquina de desarrollo (no una suposición): `~/.claude.json.mcpServers` contiene hoy `codegraph`, `gomemory` (entrada de otro proyecto) y `codebase-memory-mcp`, confirmando que el scope de usuario es un mecanismo real y ya usado por herramientas comparables.

**Alternatives considered**:
- Mantener `.mcp.json` por proyecto y solo automatizar su creación: rechazada porque no elimina el paso por-repo que es el problema central reportado.

## 4. Colisión de nombre detectada en el registro global existente

**Decision**: `~/.claude.json` ya tiene una entrada `"gomemory": {"command": "/home/admindocker/data/chicken_tools_sdd/ct", "args": ["mcp"]}` de otro proyecto. El usuario confirmó eliminar esa entrada y dejar el `mem` de este repo como única fuente de verdad bajo la clave `gomemory` en scope global. Es una acción manual de una sola vez, ejecutada antes de automatizar el registro global — no algo que el instalador deba resolver de forma automática ni silenciosa en futuras ejecuciones.

**Rationale**: Sobrescribir automáticamente una entrada de otra herramienta sin confirmación sería una acción destructiva sobre configuración compartida por todo el sistema del usuario — exactamente el tipo de acción que este mismo proyecto ya trata con cautela (ver FR-008 del spec: requiere confirmación explícita ante colisión de nombre).

**Alternatives considered**:
- Registrar gomemory bajo una clave distinta (ej. `gomemory-mem`) para evitar tocar la entrada existente: rechazada por el usuario — prefiere una única fuente de verdad bajo el nombre canónico `gomemory`.

## 5. Migración de datos legados

**Decision**: Migración transparente en el primer uso (lazy) más un comando explícito `mem migrate` para forzarla. Si existe `<root>/.memory/mem.db` y no existe aún el archivo en el store global, se mueve (no se copia) al store global, incluyendo `-wal`/`-shm` si existen. Si ambos existen, no se sobrescribe nada automáticamente — se advierte y se prefiere el store global, dejando el archivo legado intacto para inspección manual.

**Rationale**: Cero pérdida de datos es un requisito duro (spec FR-004, SC-002). Mover en vez de copiar evita tener dos copias divergentes del mismo dato después de la migración. El caso "ambos existen" se trata de forma conservadora (no-op con warning) porque es una situación anómala (reinstalación parcial previa, o `.memory/mem.db` commiteado por error) que no debe resolverse adivinando cuál de las dos fuentes es la correcta.
