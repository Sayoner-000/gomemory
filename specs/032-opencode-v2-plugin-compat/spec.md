# Feature Specification: Compatibilidad del plugin de OpenCode con v2 sin romper v1.18

**Feature Branch**: `032-opencode-v2-plugin-compat`

**Created**: 2026-09-24

**Status**: Draft

**Input**: User description: "opencode liberó la version 2.0 y daña la funcionalidad de gomemory para integrarse (https://opencode.ai/v2/docs/build/plugins/migrate-v1). Usuarios con esa versión ven: `Server plugin error · Plugin: ~/.config/opencode/plugins/gomemory.ts · Status: failed · Runtime: server · Error: Plugin must export a default definition with an id and an effect or setup function.` (y lo mismo para `cbm-augment.ts`). Realicemos el análisis para tener la funcionalidad con la versión 2.0, conservando también el legado de la versión 1.18."

## Contexto del problema *(no normativo: explica el porqué)*

gomemory se integra con OpenCode mediante un plugin que el instalador deja en `~/.config/opencode/plugins/gomemory.ts`. Ese plugin cubre lo que el servidor MCP no puede hacer por sí solo:

| Capacidad (v1) | Punto de enganche v1 | Qué aporta a la persona usuaria |
|---|---|---|
| Abrir/cerrar la sesión de trabajo y checkpoint al quedar idle | `event` (`session.created`, `session.idle`, `session.compacted`) | Las memorias quedan asociadas a la sesión correcta |
| Registrar el prompt del turno como procedencia | `chat.message` | Cada memoria guardada sabe qué pidió la persona |
| Inyectar protocolo, contexto histórico, política Octopus, nudge, aviso de compactación y recuperación post-compactación | `experimental.chat.system.transform` | El agente arranca cada turno con la memoria cargada |
| Capturar la salida de subagentes (`task`) | `tool.execute.after` | Lo aprendido por un subagente no se pierde |
| Preservar la memoria de la sesión durante la compactación | `experimental.session.compacting` | La compactación no borra lo pendiente de guardar |

OpenCode 2.0 cambió el contrato de plugins: exige un **export por defecto con `id` y una función `setup`/`effect`**, y los ganchos pasan a registrarse sobre dominios del contexto (`ctx.event.subscribe`, `ctx.session.hook("prompt" | "context")`, `ctx.tool.hook("execute.after")`, …). Los plugins v1 **no se ejecutan** en v2: el plugin falla al cargar y *todas* las capacidades anteriores desaparecen en silencio, salvo el aviso de error. La guía oficial permite un único archivo que declara ambas formas a la vez (definición v2 + `server()` v1), válido desde OpenCode **1.18.29**. v2 además renombra la clave de configuración `plugin` → `plugins` y descubre plugins locales en `.opencode/plugins/`.

La guía de migración no documentaba un equivalente v2 para la compactación. La investigación lo encontró en los tipos oficiales (ver research R-5).

El segundo error (`cbm-augment.ts`) pertenece al proyecto externo `codebase-memory-mcp`, no a gomemory: el repositorio no genera ni instala ese archivo.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - El plugin carga sin error en OpenCode 2.x (Priority: P1)

Una persona con OpenCode 2.x reinstala o actualiza gomemory y abre OpenCode. El plugin gomemory aparece como activo en la lista de plugins, sin el error "Plugin must export a default definition…", y la sesión de trabajo y el contexto histórico funcionan como en v1.

**Why this priority**: Hoy la integración está completamente rota para cualquier usuario de v2; es el bloqueo que motivó la petición.

**Independent Test**: Instalar gomemory en una máquina limpia con OpenCode 2.x, abrir una sesión y comprobar que el plugin figura como cargado y que el primer turno recibe el protocolo y el contexto del proyecto.

**Acceptance Scenarios**:

1. **Given** OpenCode 2.x y gomemory instalado, **When** OpenCode arranca, **Then** el plugin gomemory figura como cargado con un identificador estable y no se muestra ningún error de plugin atribuible a gomemory.
2. **Given** una sesión nueva en OpenCode 2.x, **When** la persona envía su primer mensaje, **Then** el agente recibe el protocolo de memoria y el contexto histórico del proyecto en ese mismo turno.
3. **Given** una sesión nueva en OpenCode 2.x, **When** se crea la sesión y luego queda inactiva, **Then** gomemory registra el inicio de sesión y el checkpoint de turno, igual que en v1.

---

### User Story 2 - Quien sigue en OpenCode 1.18 no nota ningún cambio (Priority: P1)

Una persona que todavía usa OpenCode 1.18.x actualiza gomemory. Todas las capacidades que tenía siguen funcionando exactamente igual.

**Why this priority**: El usuario pidió explícitamente conservar el legado de 1.18; romper a quien no ha migrado sería una regresión.

**Independent Test**: Con OpenCode 1.18.32 (versión actual de la máquina de desarrollo), instalar la nueva versión de gomemory y ejecutar la batería existente de pruebas de extremo a extremo de OpenCode (sesión, prompt, contexto, subagente, compactación).

**Acceptance Scenarios**:

1. **Given** OpenCode 1.18.29 o superior (rama 1.x) y la nueva versión de gomemory, **When** se ejercita cada capacidad de la tabla de contexto, **Then** todas se comportan igual que con la versión anterior de gomemory.
2. **Given** OpenCode 1.x anterior a 1.18.29, **When** se instala gomemory, **Then** el instalador deja una variante que funciona en esa versión (solo v1) o, si no puede determinarla, informa claramente qué versión mínima se soporta.

---

### User Story 3 - Paridad de capacidades en v2, o degradación declarada (Priority: P2)

En OpenCode 2.x, además de cargar, el plugin mantiene cada capacidad de v1: procedencia del prompt, captura de subagentes, preservación y recuperación en compactación. Si alguna no tiene equivalente en v2, gomemory lo declara (no falla en silencio) y el diagnóstico lo muestra.

**Why this priority**: Cargar sin error pero perder la compactación o la procedencia sería un fallo silencioso, justo lo que el proyecto ya penaliza con su matriz de canales.

**Independent Test**: En OpenCode 2.x, provocar un turno, una tarea de subagente y una compactación, y verificar en gomemory que cada capacidad dejó su rastro o que el diagnóstico la marca como no soportada en v2.

**Acceptance Scenarios**:

1. **Given** OpenCode 2.x, **When** un subagente termina una tarea, **Then** su salida llega a gomemory como en v1.
2. **Given** OpenCode 2.x, **When** la persona envía un mensaje, **Then** el prompt queda registrado como procedencia de la sesión activa.
3. **Given** OpenCode 2.x, **When** la sesión se compacta, **Then** la memoria de la sesión se preserva y la recuperación se entrega una sola vez en el turno siguiente; **o bien**, si v2 no ofrece el punto de enganche, el diagnóstico reporta la capacidad como "no disponible en OpenCode 2.x" en vez de "sana".

---

### User Story 4 - El diagnóstico explica qué versión y qué plugin fallan (Priority: P3)

Una persona ve errores de plugin en OpenCode y ejecuta el diagnóstico de gomemory. Este le indica la versión de OpenCode detectada, qué variante del plugin tiene instalada, si es compatible, y cómo repararla. Si detecta plugins de terceros en la misma carpeta con la forma v1 (p. ej. `cbm-augment.ts`), lo avisa aclarando que no son de gomemory.

**Why this priority**: Reduce soporte, pero no restaura funcionalidad por sí mismo.

**Independent Test**: Con un plugin v1-only instalado en OpenCode 2.x, ejecutar el diagnóstico y comprobar que nombra la incompatibilidad y la acción de reparación.

**Acceptance Scenarios**:

1. **Given** un plugin gomemory desactualizado instalado en OpenCode 2.x, **When** se ejecuta el diagnóstico, **Then** informa la incompatibilidad y el comando para reinstalar.
2. **Given** `cbm-augment.ts` con forma v1 en la carpeta de plugins, **When** se ejecuta el diagnóstico, **Then** avisa que ese plugin es de un proyecto externo y que debe actualizarse por su lado, sin modificarlo.

---

### Edge Cases

- OpenCode no está en el PATH o su versión no se puede leer: el instalador instala la variante dual (v1+v2) y lo indica.
- La persona tiene una configuración que registra el plugin bajo la clave v1 `plugin` y actualiza a v2 (clave `plugins`): la configuración de gomemory no debe quedar duplicada ni perdida, y la configuración ajena se conserva.
- Actualización de OpenCode 1.18 → 2.x *después* de instalar gomemory: la variante instalada debe seguir funcionando sin reinstalar (motivo para preferir la variante dual).
- El binario `mem` no está disponible: en v2, igual que en v1, los ganchos degradan en silencio sin romper el turno.
- v2 recarga o descarga el plugin: no quedan suscripciones, temporizadores ni subprocesos colgados.
- Instalaciones heredadas (`~/.config/opencode/plugins/gomemory/` anidado, `.opencode.json` legado) siguen limpiándose como hoy.
- El plugin se instala en el scope de usuario (`~/.config/opencode/plugins/`); si v2 dejara de descubrir esa ruta, el plugin no cargaría aunque sea correcto.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El plugin gomemory DEBE cargar sin error en OpenCode 2.x, exponiendo la forma de definición que v2 exige, con un identificador estable (`gomemory`).
- **FR-002**: El plugin DEBE seguir cargando y comportándose igual en OpenCode 1.18.29+ (rama 1.x).
- **FR-003**: Para OpenCode 1.x anterior a 1.18.29, el instalador DEBE instalar una variante que funcione en esa versión o informar la versión mínima soportada; nunca dejar un plugin que falle al cargar sin avisar.
- **FR-004**: En v2, el plugin DEBE mantener cada capacidad de la tabla de contexto: gestión de sesión (inicio, checkpoint al quedar idle, cierre), procedencia del prompt, inyección de protocolo/contexto/política Octopus/nudge/avisos, captura de salida de subagentes, preservación y recuperación en compactación.
- **FR-005**: Toda capacidad sin equivalente en v2 DEBE quedar declarada como tal en la matriz de canales y en el diagnóstico, en lugar de reportarse como sana o fallar en silencio.
- **FR-006**: La lógica de negocio de cada capacidad DEBE seguir viviendo en `mem` (el plugin solo traduce eventos a llamadas a `mem`), de modo que v1 y v2 compartan comportamiento y no se dupliquen reglas.
- **FR-007**: El plugin en v2 DEBE liberar todos sus recursos (suscripciones a eventos, trabajos pendientes) al descargarse o recargarse.
- **FR-008**: El instalador DEBE registrar gomemory en la configuración de OpenCode con la forma que corresponda a la versión (clave `plugin` en v1, `plugins` en v2, si aplica), de forma idempotente y conservando la configuración ajena; los permisos y la entrada MCP existentes DEBEN seguir funcionando en ambas versiones.
- **FR-009**: El diagnóstico (`mem doctor`) DEBE informar la versión de OpenCode detectada, la variante del plugin instalada, su compatibilidad y la acción de reparación.
- **FR-010**: El diagnóstico DEBE detectar plugins de terceros con forma v1 en la carpeta de plugins de OpenCode cuando OpenCode es 2.x y avisar, sin modificarlos ni atribuirlos a gomemory.
- **FR-011**: La degradación silenciosa ante la ausencia de `mem` DEBE conservarse en ambas versiones.
- **FR-012**: Las pruebas de contrato existentes de OpenCode (ganchos, nombres de tools, compactación) DEBEN cubrir ambas formas (v1 y v2) del plugin.
- **FR-013**: La documentación de instalación DEBE indicar las versiones de OpenCode soportadas y cómo actualizar tras migrar de 1.18 a 2.x.

### Key Entities

- **Variante del plugin**: forma del archivo instalado (solo v1, dual v1+v2). Atributos: versiones de OpenCode compatibles, identificador, capacidades soportadas.
- **Capacidad de integración**: cada fila de la tabla de contexto. Atributos: punto de enganche en v1, equivalente en v2 (o "no disponible"), estado en la matriz de canales.
- **Versión de OpenCode detectada**: versión instalada en la máquina; determina la variante y la forma de configuración.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 0 errores de carga de plugin atribuibles a gomemory en OpenCode 2.x tras reinstalar.
- **SC-002**: 100 % de las capacidades de la tabla de contexto funcionan en OpenCode 1.18.29+ sin cambios observables respecto a la versión anterior de gomemory.
- **SC-003**: En OpenCode 2.x, al menos 4 de las 5 capacidades funcionan con paridad y la restante (si la hubiera) aparece como "no disponible en v2" en el diagnóstico; ninguna se reporta como sana si no funciona.
- **SC-004**: Una persona que actualiza OpenCode de 1.18 a 2.x con gomemory ya instalado no necesita pasos manuales, o recibe del diagnóstico un único comando de reparación.
- **SC-005**: Reinstalar gomemory dos veces seguidas no altera la configuración de OpenCode (idempotencia) en ninguna de las dos versiones.

## Assumptions

- La definición dual (v2 + `server()` v1) en un solo archivo es la vía elegida. *Verificado en research R-3*: la cargan 2.0.16 (`setup`) y 1.17.0, 1.18.20 y 1.18.32 (`server`), así que no hace falta una variante solo v1 y FR-003 queda cubierto con el mismo archivo.
- La ruta de usuario `~/.config/opencode/plugins/` sigue siendo descubierta por OpenCode 2.x. *Verificado en research R-2* con el log de carga de 2.0.16.
- El paquete de tipos de v2 (`@opencode/plugin`) no es una dependencia de ejecución: como hoy, el plugin no debe requerir instalar módulos.
- En v2 sigue siendo posible ejecutar el binario `mem` desde el plugin (hoy se usa el shell `$` que v1 inyecta); si v2 no lo inyecta, se usa el mecanismo de subprocesos del propio runtime.
- La compactación sí tiene equivalente en v2: `session.hook("compaction")` y el evento `session.compaction.ended` (*research R-5*). FR-005 queda para las lecturas de mensajes best-effort (R-7).
- v2 cambió el formato (`mcp.servers`, `permissions` como lista de reglas), pero traduce solo el formato v1 al leerlo (*research R-8*); el instalador no cambia.
- Corregir `cbm-augment.ts` queda fuera de alcance (es de `codebase-memory-mcp`); gomemory solo lo diagnostica (FR-010). Conviene abrir un issue en ese proyecto.
- Según las reglas de trabajo del proyecto, la verificación final se hace contra OpenCode real en ejecución (1.18.32 y 2.x), no solo con pruebas unitarias.
