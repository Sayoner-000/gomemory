# Data Model — Feature 021

**Fecha**: 2026-08-23 · **Spec**: [spec.md](./spec.md) · **Research**: [research.md](./research.md)

Esta feature **no crea tablas ni columnas nuevas**. Reutiliza `memories.topic_key`,
que ya existe desde la feature 008 con su índice parcial. Lo que sigue documenta las
entidades tal como quedan y las reglas que las gobiernan.

---

## 1. Claves de tópico canónicas

Nuevas constantes en `domain/seed.go` — fuente única, para que el instalador, el
constructor de contexto y el comando de constitución nunca puedan discrepar sobre
qué fila es cuál.

```go
const (
    TopicWorkRules    = "gomemory:work-rules"
    TopicConstitution = "gomemory:constitution"
)
```

**Por qué el prefijo `gomemory:`**: distingue una fila sembrada por el producto de
una agrupación que la persona haya creado con su propio `topic_key` al guardar. El
espacio de nombres es una convención, no una restricción del esquema.

**Estabilidad**: estas dos cadenas son contrato. Cambiarlas en una versión futura
convertiría las semillas existentes en huérfanas y provocaría una segunda siembra
duplicada. Si alguna vez hay que versionarlas, la migración debe ser explícita, no
un cambio de constante.

---

## 2. Memoria semilla — reglas de trabajo

| Campo | Valor | Nota |
|---|---|---|
| `project` | clave del proyecto destino | `ProjectRepo.Key(target)`, no el cwd |
| `session_id` | **vacío** | Una semilla no pertenece a ninguna sesión de trabajo. La inercia de la sinapsis ya no depende de esto: la garantiza la vía inerte (§3bis) |
| `type` | `preference` | `domain.Preference` |
| `title` | `Reglas de trabajo del proyecto` | |
| `content` | `templates/agent-preamble.md` íntegro | 58 líneas, ya embebido |
| `filepath` | vacío | Una semilla no describe un archivo concreto del proyecto |
| `topic_key` | `gomemory:work-rules` | |

**Ciclo de vida**:

```
(no existe) --SeedDefaults--> [sembrada] --edición del usuario--> [propia del usuario]
                                   |                                      |
                                   +--- SeedDefaults posterior: NO TOCA ---+
                                   |
                                   +--- forget_memory --> (no existe): la siguiente
                                        siembra la recrea desde la plantilla
```

**Regla dura**: `SeedDefaults` solo transiciona desde *(no existe)*. Nunca escribe
sobre una fila con esa clave, cualquiera sea su contenido (FR-003, R5).

---

## 3. Memoria semilla — constitución

Idéntica en estructura, distinta en clasificación y en cómo se consume:

| Campo | Valor |
|---|---|
| `type` | `architecture` |
| `title` | `Constitución del proyecto (spec-kit)` |
| `content` | `templates/speckit-constitution-gen.md` íntegro (635 líneas) |
| `topic_key` | `gomemory:constitution` |

**Diferencia de consumo**: la constitución NO goza de emisión íntegra en el contexto.
Aparece en `## Decisiones de Arquitectura` recortada a 200 caracteres con su puntero,
como cualquier otra memoria. Es intencional: 635 líneas en cada arranque de sesión
contradicen el objetivo de la feature 020. Se consulta bajo demanda (FR-024).

---

## 3bis. Vía de inserción inerte

Las semillas **no** se insertan por la vía normal. `InsertMemory` dispara cuatro
efectos laterales pensados para memorias nacidas del trabajo real; sobre una semilla
creada por la herramienta, dos son inocuos solo por accidente y uno hace daño
observable (research R4).

```
insertMemory(db, m, opts)          núcleo compartido: INSERT + índice FTS
├─ InsertMemory(db, m)             opts{} — camino actual, sin cambios
└─ InsertSeedMemory(db, m)         opts{inerte:true} — NUEVO
```

| Efecto de `InsertMemory` | Vía normal | Vía inerte |
|---|---|---|
| `RedactPrivate` / `RedactSecrets` | Se aplica | **Se aplica igual** — la depuración de secretos no se desactiva nunca; se protege con un test guardián de igualdad (FR-032) |
| `annotateImpact` | Se aplica | Se aplica igual — inocuo sin `Filepath` |
| Dedup por identidad / `topic_key` | Se aplica | Se aplica igual — es la red que hace idempotente el `INSERT` |
| `formSynapse` | Se aplica | **Omitido** (FR-033) |
| `exportToADR` | Se aplica | **Omitido** (FR-034) |

**Por qué la redacción de secretos NO se omite**: es una garantía de seguridad, no un
canal lateral. Desactivarla para las semillas abriría un agujero por comodidad. La
brecha real no era que se aplicara, sino que pudiera mutilar el texto **sin que nadie
se enterara**; eso se cierra con la comprobación de igualdad, no apagando la defensa.

**Invariante de la vía inerte**: sembrar no produce escritura alguna fuera de las
tablas `memories` y `memory_search` del propio proyecto. Ni relaciones, ni registros
de sincronización, ni archivos, ni llamadas a proveedores externos.

---

## 3ter. Documento fijado: la semilla vista por quien la administra

Una semilla y un documento fijado son **la misma fila** con dos lecturas. Como
semilla, la herramienta la crea si falta. Como documento fijado, la persona la
exporta, la edita y la reimporta. El catálogo `domain.PinnedDocs` (ver
[contracts/pinned-docs.md](./contracts/pinned-docs.md)) es lo que da a esa fila un
alias legible y un contenido por defecto al que volver.

### Estado observable

| Estado | Cómo se determina | Qué ve la persona |
|---|---|---|
| `sin sembrar` | No hay memoria con esa clave | La herramienta ofrece el contenido por defecto |
| `por defecto` | Contenido idéntico a la plantilla embebida | Nadie lo ha tocado |
| `personalizado` | Contenido distinto de la plantilla | Es del equipo, con su fecha de modificación |

El estado se **deriva**, no se almacena: comparar contra la plantilla embebida evita
una columna nueva y un dato que podría quedar desincronizado.

### Ciclo de vida completo

```
(no existe)
   │ siembra automática (nunca pisa)
   ▼
[por defecto] ──importar──> [personalizado] ──importar──> [personalizado]
   ▲                              │                             │
   └────────── restaurar ─────────┘                             │
   ▲                                                            │
   └──────────────── restaurar ─────────────────────────────────┘

  install / update / arranque MCP: NO transicionan desde ningún estado poblado
```

**La regla única**: el contenido es de la persona. La siembra solo actúa sobre el
vacío; la importación y la restauración son las únicas vías que reemplazan, y ambas
las pide la persona explícitamente.

### Escrituras permitidas sobre un documento fijado

| Operación | ¿Escribe? | Vía |
|---|---|---|
| Siembra | Solo si no existe | Inerte |
| Importación | Siempre (crea o reemplaza) | Inerte |
| Restauración | Siempre (reemplaza con la plantilla) | Inerte |
| `install` / `update` / arranque MCP | Nunca sobre uno existente | — |

Las tres escrituras usan la **vía inerte** de §3bis: ni relaciones automáticas ni
publicación externa, con la depuración de secretos siempre activa.

---

## 4. Regla de emisión en el contexto

Estados posibles de la sección de reglas dentro de `Build()`:

| Condición | Salida |
|---|---|
| `Topics == nil` (wiring incompleto, tests) | Sección ausente, sin error |
| Sin memoria con `TopicWorkRules` | Sección ausente, sin error |
| Semilla presente, `IndexMode == false` | `## Reglas de trabajo (memoria fijada)` + contenido **íntegro** |
| Semilla presente, `IndexMode == true` | Sección presente, cuerpo colapsado a `→ get_memory <id>` |

**Posición**: inmediatamente después de `# Memoria del Proyecto`, antes de conflictos
y sinapsis.

**Exclusión de la sección de preferencias**: el bucle de `## Preferencias del Usuario`
salta la memoria cuyo `TopicKey == domain.TopicWorkRules`. Esto exige que
`ListMemories` traiga la columna (R2); sin ese cambio la comparación siempre daría
falso y las reglas aparecerían dos veces.

**Contabilidad de presupuesto**: el contenido íntegro **no** incrementa
`discardedChars`; la invariante `rawChars >= finalChars` se sigue cumpliendo por
construcción (R3).

---

## 5. Artefactos legados — clasificación y trato

| Artefacto | Trato | Respaldo | Motivo |
|---|---|---|---|
| `AGENTS.md` | Eliminar | **Sí** | Puede contener texto propio de la persona |
| `CLAUDE.md` | Eliminar | **Sí** | Ídem |
| `CLAUDE.txt` | Eliminar | **Sí** | Ídem |
| `speckit-constitution-gen.md` | Eliminar | No | Copia literal de la plantilla embebida; recuperable con `mem constitution` |
| `.windsurf/mcp_config.json` | Desregistrar | No | Config generada; ver regla de abajo |
| `.cline/mcp_settings.json` | Desregistrar | No | Ídem |
| `.cursorrules`, `.windsurfrules` | **No tocar** | — | El instalador nunca los crea desde cero; son de la persona |
| `.cursor/mcp.json` | **No tocar** | — | `setupCursor` sigue activo en `install` |

**Regla de desregistro (FR-020)**, aplicada a cada archivo de configuración:

```
leer JSON
├─ ilegible o inválido ──────────────> no tocar, informar
├─ sin clave "gomemory" en mcpServers > no tocar, silencio
└─ con clave "gomemory"
   ├─ quedan otros servidores ───────> quitar solo "gomemory", reescribir
   └─ no queda ninguno ──────────────> borrar archivo y su directorio
```

**Regla de respaldo (FR-017/FR-018)**:

```
para cada archivo de instrucciones existente:
  copiar a .memory/backups/agent-files/<nombre>
  ├─ copia OK ──────> borrar original, informar ruta del respaldo
  └─ copia falla ───> NO borrar, informar el error, continuar con el resto
```

El respaldo sobrescribe una copia previa del mismo nombre: la instalación es
idempotente y el archivo original ya no existirá en la segunda pasada, así que el
caso solo se da si la persona restaura el archivo a mano entre instalaciones.

---

## 6. Entidad de salida — respaldo de archivos de agente

| Atributo | Valor |
|---|---|
| Ubicación | `<proyecto>/.memory/backups/agent-files/` |
| Nombre | El mismo del archivo original |
| Visibilidad | Ignorado por git (`.gitignore` línea 2: `.memory/`) |
| Limpieza | Ninguna automática — es responsabilidad de la persona |
| Creación | Solo cuando hay al menos un archivo que respaldar |
