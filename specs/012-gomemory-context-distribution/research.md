# Research: Distribuir el brazo extensor gomemory-context vía `mem install`, transversal a agentes

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

Sin marcadores `[NEEDS CLARIFICATION]` en `spec.md`. Este documento
registra las decisiones técnicas, todas verificadas contra el
comportamiento real del código y de la CLI `specify` (no supuestos).

## 1. Forma exacta del artefacto de Claude Code

**Decision**: empaquetar el `SKILL.md` tal como lo generó `specify
extension add ... --dev` en la validación en vivo de la spec 011
(`.specify/extensions/gomemory-context/.specify-dev/agent-commands/claude/
speckit-gomemory-context-update/SKILL.md`), copiándolo byte a byte a
`infrastructure/templates/gomemory-context/claude/
speckit-gomemory-context-update/SKILL.md`.

**Rationale**: es el artefacto REAL que Claude Code reconoce — ya
verificado end-to-end (T016 de la spec 011: se invocó el skill generado y
entregó el resumen correcto). Reusarlo evita reimplementar en Go la lógica
de traducción de `specify` (frágil, se desalinearía si spec-kit cambia su
formato) y evita depender de la CLI de terceros en tiempo de instalación.

**Alternatives considered**:
- Reimplementar en Go la transformación comando-fuente → SKILL.md —
  rechazada: duplica lógica de un proyecto de terceros (spec-kit) que
  gomemory no controla ni versiona.
- Depender de `specify extension add` en tiempo de `mem install` —
  rechazada explícitamente por el pedido del usuario ("sin depender de que
  el usuario tenga instalada la CLI de Python `specify`").

## 2. Forma exacta del artefacto de OpenCode

**Decision**: generar el artefacto real (`specify init --integration
opencode` + `specify extension add` sobre una copia de la extensión, en un
directorio de prueba aislado) y empaquetarlo tal cual en
`infrastructure/templates/gomemory-context/opencode/
speckit.gomemory-context.update.md`.

**Rationale**: mismo criterio que la Decisión 1 — verificar contra el
sistema real en vez de inferir el formato. Confirmado en vivo: OpenCode usa
`.opencode/commands/<command>.md` con un frontmatter mínimo (`description:`
únicamente, sin `name`/`compatibility`/`metadata` como Claude) más dos
comentarios HTML de trazabilidad (`<!-- Extension: gomemory-context -->`,
`<!-- Config: .specify/extensions/gomemory-context/ -->`) antepuestos al
cuerpo del comando — formato distinto al de Claude Code, confirmando que no
había una forma "genérica" única a adivinar.

**Alternatives considered**: ninguna — una vez decidido reusar artefactos
reales (Decisión 1), generar el de OpenCode con el mismo método fue directo.

## 3. Mecanismo de distribución: reutilizar `go:embed` + copia diff-aware existente

**Decision**: agregar `infrastructure/templates/gomemory-context/` como
tercer árbol embebido (junto a `plugin/` y `templates/` ya existentes en
`infrastructure/main.go`), y reutilizar `InstallPlugin`/`copyFileOrDir`
(`adapters/primary/setup/setup.go`) para la copia recursiva — la misma
función que ya instala el plugin de OpenCode.

**Rationale**: `copyFileOrDir` ya implementa exactamente la semántica que
pide FR-005/FR-006 (comentario en el código, verificado):
*"si el destino ya tiene exactamente el mismo contenido, no se reescribe
[...] si difiere (versión anterior), se sobrescribe para que install
siempre entregue la versión actual"*. Es la función correcta para
artefactos del framework, distinta a `embeddedTemplate()` (usada para la
constitución, que SÍ es contenido del proyecto y por eso nunca se
sobrescribe si ya existe — ver `cmd_install.go` sección 4b).

**Alternatives considered**:
- Copiar con `embeddedTemplate()` + "no sobrescribir si existe" (patrón de
  la constitución) — rechazada: congelaría la primera versión instalada
  para siempre, violando FR-005 (las correcciones futuras deben
  propagarse).
- Escribir una función de copia nueva desde cero — rechazada: duplicaría
  `copyFileOrDir`, que ya está probada en producción para el plugin de
  OpenCode.

## 4. Permiso de ejecución del script bash

**Decision**: tras copiar el árbol con `InstallPlugin`, aplicar
`os.Chmod(..., 0755)` explícitamente sobre
`scripts/bash/update-gomemory-context.sh` en el destino.

**Rationale**: `copyFileOrDir` escribe todos los archivos con `0644`
(verificado leyendo el código: `os.WriteFile(dstPath, data, 0644)`) —
correcto para el resto de artefactos (config JSON, Markdown, un plugin
`.ts` que un runtime JS interpreta), pero el script bash necesita bit de
ejecución porque el hook de spec-kit lo invoca directamente. Un chmod
explícito y acotado a ese único archivo evita tocar el comportamiento
general de `copyFileOrDir` (usado también por el plugin de OpenCode, que
no necesita ejecutables).

**Alternatives considered**:
- Modificar `copyFileOrDir` para detectar extensión `.sh` y aplicar 0755
  automáticamente — considerado pero descartado por ahora: agregaría una
  regla implícita a una función compartida por otro llamador (el plugin de
  OpenCode) sin necesidad; un chmod explícito en el único lugar que lo
  necesita es más simple y más fácil de auditar.

## 5. Punto de integración en `cmd_install.go`

**Decision**: agregar el nuevo paso justo después de la copia de la
constitución (sección "4b" existente), antes de la sección 5 (configuración
de agentes), gateado por `os.Stat(filepath.Join(target, ".specify"))`.

**Rationale**: mismo lugar lógico donde ya vive la única otra pieza
relacionada con spec-kit que `mem install` distribuye hoy
(`speckit-constitution-gen.md`) — mantiene todo lo relacionado con
spec-kit agrupado y visible en el flujo de instalación.

**Alternatives considered**: ejecutarlo dentro de
`setup.InstallClaudeCode`/`InstallOpenCode` — rechazada: esas funciones
instalan la integración MCP/plugin del agente en sí (concepto distinto),
mezclar la extensión de spec-kit ahí sería confuso y además necesitaría
duplicar el chequeo de `.specify/` en dos sitios.

## 6. Alcance de agentes: solo Claude Code y OpenCode

**Decision**: solo estos dos agentes reciben el brazo extensor en esta
iteración — confirmado leyendo `cmd_install.go`: son los únicos con
`setup.InstallXxx()` (plugin + hooks completos); Cursor/Windsurf/Cline/
Codex (`setupCursor`/`setupWindsurf`/`setupCline`/`setupCodex` en
`cmd_mcp_setup.go`) solo reciben un `mcp.json`/`mcp_config.json` con la
entrada MCP de gomemory, sin ningún mecanismo de comandos/skills propio
que spec-kit pueda aprovechar.

**Rationale**: ya documentado como Assumption en `spec.md` — es una
observación del código existente, no una decisión nueva que tomar.

**Alternatives considered**: N/A (ver Assumptions de spec.md).
