# Contrato — `get_plan_context()` / `mem plan-context`

**Feature**: 013-atomic-plan-mode
**Tipo**: interfaz pública (herramienta MCP + comando de línea de comandos)

Es la única superficie nueva que la feature expone. Devuelve, en una sola llamada, el
método de descomposición atómica y el contexto histórico del proyecto, para que el agente
pueda invocarla una vez al entrar en modo plan (FR-001).

---

## 1. Herramienta MCP

### Firma

| Campo | Valor |
|-------|-------|
| Nombre | `get_plan_context` |
| Parámetros | Ninguno |
| Devuelve | Un bloque de texto en Markdown |

### Descripción publicada al agente

La descripción es parte del contrato: es lo que el agente lee para decidir cuándo llamarla.
Debe dejar el disparador inequívoco.

> Obtiene el método de descomposición atómica y el contexto histórico del proyecto para
> planificar. Llámala SIEMPRE al entrar en modo plan, antes de redactar el plan: cuando
> entres en un modo de planificación, cuando la persona invoque un comando de
> planificación, o cuando la solicitud pida un plan, un enfoque o una estrategia antes de
> tocar código. Aplica el método que devuelve al redactar el plan.

### Registro

Se registra junto al resto de herramientas en `registerTools`
(`adapters/primary/cli/cmd_mcp.go`), siguiendo la forma de `get_context`
(línea 296): `mcp.AddTool` con parámetros `struct{}` y un único `mcp.TextContent` de
respuesta.

### Permisos — requisito de activación, no un extra

Sin pre-aprobación, cada planificación queda bloqueada pidiendo permiso: la activación
autónoma deja de ser autónoma. El propio código del proyecto señala esa omisión como "la
causa más común de que el protocolo de memoria no se aplique automáticamente". Hay que
cubrir **los dos agentes**, y su estado de partida es distinto.

#### Claude Code — extender lo que ya existe

`mcp__gomemory__get_plan_context` **debe** añadirse a `ClaudeAutoAllowTools`
(`adapters/primary/setup/claude_code_setup.go`). Es de solo lectura, así que cumple el
criterio declarado de esa lista. `writeClaudePermissions` ya escribe esa lista en
`permissions.allow` de forma idempotente: basta con añadir la entrada.

#### OpenCode — construir lo que no existe

**Estado actual verificado**: OpenCode no tiene ninguna gestión de permisos en gomemory
(ver D11 en `research.md`). `writeOpenCodeMCPFile` escribe solo `{type, command, enabled}`,
y `ApplyAutoApprove` no incluye `opencode.json` entre sus rutas.

**Forma exigida** — clave `permission` de primer nivel, con comodín y excepción:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "permission": {
    "gomemory_*": "allow",
    "gomemory_forget_memory": "ask"
  }
}
```

| Exigencia | Motivo |
|-----------|--------|
| Clave `permission` de primer nivel, valores `allow`/`ask`/`deny` | Es el esquema real de OpenCode, confirmado en su documentación |
| `forget_memory` en `ask`, nunca en `allow` | Replica la exclusión deliberada del lado de Claude Code: es destructiva e irreversible. Un comodín plano abriría una herramienta irreversible de pasada |
| Escribir en ambos ámbitos | `writeOpenCodeMCPFile` documenta que "el esquema es idéntico en ambos scopes"; se invoca desde `InstallOpenCode` e `InstallOpenCodeGlobal` |
| Idempotente y preservando el resto de la configuración | Mismo criterio que `writeOpenCodeMCPFile` ya aplica |

**Prohibido**: extender `ApplyAutoApprove` añadiendo `opencode.json` a su lista de rutas.
Esa función escribe la forma `mcpServers[].autoApprove`, que OpenCode **no entiende**. El
proyecto ya cometió ese error una vez —el comentario de `WriteOpenCodeMCP` explica que una
configuración previa usaba un esquema "que OpenCode ignora por completo (de ahí que las
tools nunca aparecieran)"— y repetirlo daría cero errores visibles y cero efecto.

**Verificación**: no basta con pruebas unitarias. Hay que comprobar contra OpenCode en
ejecución que las herramientas de gomemory no piden aprobación, con `opencode debug config`
— la misma vía con la que el proyecto confirmó empíricamente el comportamiento de
configuración de OpenCode en la feature 005.

---

## 2. Comando de línea de comandos

### Firma

```
mem plan-context
```

| Aspecto | Valor |
|---------|-------|
| Argumentos | Ninguno |
| Banderas | Ninguna |
| Salida | Markdown por salida estándar |
| Código de salida | **Siempre 0** |

Se registra en `dispatcher.go` como `case "plan-context":`, junto a los demás comandos.

### Por qué existe además de la herramienta MCP

Es la vía para todo agente que no tenga el servidor MCP conectado (FR-003). El bloque de
protocolo ya declara hoy este mismo patrón de degradación para el resto de operaciones
("Si el MCP no está disponible en el agente actual, usa el CLI equivalente").

---

## 3. Contrato de salida — común a ambas vías

Las dos rutas devuelven **exactamente el mismo documento**. La única diferencia es el
transporte.

### Estructura

```markdown
# Método de planificación atómica

<contenido de la plantilla embebida>

---

# Memoria del Proyecto

<salida de ContextBuilder.Build()>
```

### Los tres estados

| Estado | Condición | Cuerpo | Código de salida |
|--------|-----------|--------|------------------|
| **Completo** | Memoria inicializada y `Build()` tiene éxito | Método + separador + contexto | 0 |
| **Degradado** | Memoria sin inicializar, o `Build()` devuelve error | Solo el método | 0 |
| **Silenciado** | `atomic_plan_disabled: true` en la configuración | Vacío | 0 |

### Invariantes

1. **El código de salida es siempre 0.** Ninguna condición de error interrumpe el modo
   plan (FR-034). Un fallo al construir el contexto degrada a "solo método"; nunca propaga
   el error.
2. **El método nunca falta salvo en estado silenciado.** La ausencia de historial es una
   circunstancia, no una preferencia: la Historia 2 de la spec es independiente de la
   Historia 1 y debe seguir entregando valor sin memoria (FR-034).
3. **El contexto se obtiene llamando a `ContextBuilder.Build()`**, nunca reconstruyéndolo.
   Es lo que hace que el presupuesto de `SettingsData.Budget` se aplique (FR-007). Duplicar
   esa lógica rompería el requisito en silencio.
4. **Sin efectos secundarios.** La llamada es de solo lectura: no abre sesión, no escribe
   memorias, no toca el disco.
5. **El método precede al contexto.** El agente debe conocer las reglas antes que el
   material; además, si el contexto se trunca por presupuesto, lo que se pierde es la cola
   del historial y nunca el método.

---

## 4. Contrato de la instrucción de activación

El bloque de protocolo que `mem install` escribe en los archivos de agente sube de
`gomemory-protocol-v5` a `gomemory-protocol-v6` y añade una sección de modo plan.

### Contenido obligatorio de la sección

| Elemento | Exigencia |
|----------|-----------|
| Las tres formas del disparador | Modo plan nativo · comando de planificación explícito · solicitud que pide plan/enfoque/estrategia antes de tocar código |
| La acción | Llamar a `get_plan_context()` o, en su defecto, `mem plan-context` |
| La obligación | Aplicar el método devuelto al redactar el plan |
| El límite | En modo plan: entregar el árbol y detenerse; no ejecutar (FR-020, FR-021) |

### Restricción de tamaño

La sección **debe** ser breve —del orden de 8 líneas—. Vive en el prompt de sistema de
todos los turnos, no solo los de planificación. El método completo llega por la llamada,
no por el bloque (ver D5 en `research.md`).

### Contrato de actualización

Subir el número de versión del marcador es suficiente para que las instalaciones existentes
se actualicen: `versionMarkerPattern` (`cmd_install.go`) reconoce
`<!-- gomemory-protocol-v\d+ -->` con cualquier número, y `composeAgentFile` reemplaza el
bloque entero. No hay que escribir migración (FR-030).

**Idempotencia (FR-029)**: `composeAgentFile` ya devuelve `changed=false` cuando el
marcador de la versión vigente está presente, así que reinstalar no reescribe nada.

---

## 5. Contrato de los envoltorios nativos (opcional)

Capa opcional; la funcionalidad opera sin ella (ver D6 en `research.md`).

| Agente | Artefacto | Ámbito proyecto | Ámbito global |
|--------|-----------|-----------------|---------------|
| Claude Code | Habilidad | `.claude/skills/` | Directorio de habilidades de usuario |
| OpenCode | Comando | `.opencode/commands/` | Directorio de comandos de usuario |

**Exigencias**:

- Contenido **equivalente** al que devuelve `get_plan_context()`, generado desde la misma
  plantilla embebida. Nunca una copia editada a mano (FR-028).
- Distribución mediante `InstallPlugin`, que solo reescribe un archivo si su contenido
  difiere — la idempotencia ya está resuelta y verificada en producción por
  `InstallSpeckitExtension`.
- Su ausencia no degrada la funcionalidad: el disparador del bloque de protocolo sigue
  operando.

---

## 6. Compatibilidad

| Aspecto | Efecto |
|---------|--------|
| Herramientas MCP existentes | Sin cambios. `get_context` sigue igual |
| Comandos existentes | Sin cambios. `mem context` sigue igual |
| Esquema de base de datos | Sin cambios |
| Archivos `settings.json` previos | Siguen siendo válidos; el campo nuevo se omite y la funcionalidad queda activa |
| Bloques de protocolo v5 instalados | Se sustituyen automáticamente al reinstalar |
| Agentes sin MCP | Cubiertos por el comando de línea de comandos |
| Agentes sin modo plan nativo | Cubiertos por las otras dos formas del disparador |
