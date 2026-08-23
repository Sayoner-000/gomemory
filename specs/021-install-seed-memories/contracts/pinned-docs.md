# Contrato — Documentos fijados: catálogo, CLI y TUI

**Componentes**: `domain/seed.go` (catálogo), `adapters/primary/cli/cmd_docs.go` (nuevo),
`adapters/primary/tui/tui_docs.go` (nuevo)

> **Principio rector**: las plantillas que se envían son un **default**, no doctrina.
> Todo lo que sigue existe para que el contenido de las reglas y la constitución sea
> del equipo, y la memoria un contenedor neutral.

---

## 1. Catálogo (una tabla, cero casos especiales)

```go
// domain/seed.go

// PinnedDoc describe un documento fijado: una memoria semilla vista desde la
// perspectiva de quien la administra. El catálogo es table-driven a propósito —
// añadir un documento nuevo es una entrada más, nunca un comando ni una pantalla
// nueva (FR-035, FR-015 del criterio de éxito SC-015).
type PinnedDoc struct {
    Alias    string     // lo que teclea la persona: "rules", "constitution"
    TopicKey string     // identidad en la memoria
    Type     MemoryType
    Title    string     // título de la memoria
    Label    string     // rótulo en la TUI: "Reglas IA", "Constitución"
    Template string     // nombre de la plantilla embebida, para restaurar
}

var PinnedDocs = []PinnedDoc{
    {Alias: "rules", TopicKey: TopicWorkRules, Type: Preference,
     Title: "Reglas de trabajo del proyecto", Label: "Reglas IA",
     Template: "agent-preamble.md"},
    {Alias: "constitution", TopicKey: TopicConstitution, Type: Architecture,
     Title: "Constitución del proyecto (spec-kit)", Label: "Constitución",
     Template: "speckit-constitution-gen.md"},
}
```

**Regla de crecimiento**: un tercer documento fijado se añade con una entrada aquí.
La CLI y la TUI recorren el catálogo; no lo enumeran a mano. Si alguna vez hace falta
tocar `cmd_docs.go` o `tui_docs.go` para añadir un documento, el diseño se rompió.

---

## 2. CLI — `mem docs`

```
mem docs [list]                      Listar documentos fijados y su estado
mem docs show   <alias>              Escribir el contenido vigente en stdout
mem docs export <alias> [-o ruta]    Igual que show; con -o escribe un archivo
mem docs export --all -o <dir>       Exportar todo el catálogo a un directorio
mem docs import <alias> <ruta>       Reemplazar el contenido desde un archivo
mem docs import --topic <clave> <ruta>   Importar a cualquier clave, dentro o fuera del catálogo
mem docs reset  <alias>              Restaurar el contenido por defecto
```

**Alias ergonómicos** (misma maquinaria, nombre corto para lo frecuente):

| Atajo | Equivale a |
|---|---|
| `mem constitution` | `mem docs show constitution` |
| `mem constitution --sync` | `mem docs show constitution` + escribir `.specify/memory/constitution.md` |
| `mem rules` | `mem docs show rules` |

### `mem docs list`

```
ALIAS          DOCUMENTO       ESTADO           LÍNEAS  ÚLTIMA MODIFICACIÓN
rules          Reglas IA       personalizado        71  2026-08-23 09:14
constitution   Constitución    por defecto         635  2026-08-23 08:02
```

`ESTADO` se deriva comparando el contenido guardado con la plantilla embebida:
idéntico → `por defecto`; distinto → `personalizado`; ausente → `sin sembrar`
(FR-044).

### Contrato de salida

| Operación | stdout | stderr | Exit |
|---|---|---|---|
| `show`/`export` con documento presente | Contenido vigente íntegro | — | 0 |
| `show`/`export` con documento ausente | Plantilla por defecto | `⚠️ No hay <alias> en la memoria; se muestra el contenido por defecto.` | 0 |
| `export -o ruta` | — | `✅ <alias> → ruta (N líneas)` | 0 |
| `export -o` sin permisos | — | Motivo del fallo | ≠ 0 |
| `import` correcto | — | `✅ <alias> actualizado desde ruta (N líneas)` | 0 |
| `import` de archivo vacío o ilegible | — | Motivo; **documento anterior intacto** | ≠ 0 |
| `import` de documento inexistente | — | `✅ <alias> creado desde ruta` | 0 |
| `import` con contenido idéntico | — | `ℹ️ <alias> sin cambios` | 0 |
| `reset` | — | `✅ <alias> restaurado al contenido por defecto` | 0 |
| alias desconocido | — | Motivo + lista de alias válidos | ≠ 0 |

**Regla de stdout**: en `show`/`export` sin `-o`, stdout lleva **solo** el documento.
Todo aviso va a stderr, para que `mem docs show rules > reglas.md` produzca un archivo
limpio y `mem docs show rules | diff - reglas.md` funcione.

---

## 3. Semántica de importación

```
import(alias|clave, ruta)
├─ archivo ilegible ──────────────> error, nada cambia
├─ contenido vacío tras depurar ──> error, nada cambia          (FR-040)
├─ idéntico al vigente ───────────> no-op informado             (idempotencia)
├─ documento ausente ─────────────> INSERT por vía inerte       (FR-039)
└─ documento presente ────────────> UPDATE por vía inerte       (FR-038)
```

### Import vs. siembra: por qué uno actualiza y el otro no

Hay una tensión aparente con el invariante de la siembra («`SeedDefaults` nunca
ejecuta un `UPDATE`»). No es contradicción, son dos operaciones distintas:

| | Siembra | Importación |
|---|---|---|
| Quién la origina | La herramienta, sola | La persona, explícitamente |
| Sobre documento existente | **No toca nada** | **Reemplaza** |
| Intención | Dar un punto de partida si no hay ninguno | Poner el contenido del equipo |

La regla de fondo es una sola: **el contenido es de la persona**. La siembra no lo
pisa porque nadie se lo pidió; la importación lo reemplaza porque es exactamente lo
que se pidió.

### Garantías comunes con la siembra

- Vía **inerte**: sin `formSynapse`, sin `exportToADR`, ni siquiera con la
  sincronización externa activada (FR-045).
- La depuración de secretos **sigue activa**: si alguien importa un archivo con un
  token pegado por error, no se persiste.
- La `topic_key` se conserva siempre: un documento importado sigue siendo el mismo
  documento fijado y mantiene su trato en el contexto (FR-038).

---

## 4. TUI

### Filas nuevas en el menú de configuración

Se generan **recorriendo el catálogo**, y se añaden al FINAL del menú, respetando la
convención declarada en `tui.go` (`configRowReindexGraph`: «las filas nuevas se
agregan siempre al final, nunca insertadas en medio, para no invalidar
`configRowAtomicPlan`/`configOptions` ya referenciadas por nombre en los tests»).

```go
const configRowDocsBase = configRowPlanGuard + 1
const configOptions     = configRowDocsBase + len(domain.PinnedDocs)
```

Rótulo por fila, con el estado a la vista:

```
  Actualizar Reglas IA: personalizado
  Actualizar Constitución: por defecto
```

### `screenDocs` — pantalla de un documento fijado

Reutiliza el patrón ya probado de `screenImport` (entrada de ruta con
`textinput`, `enter` confirma, `esc` vuelve a `screenConfig`).

```
Reglas IA                                        [personalizado · 71 líneas]
Clave: gomemory:work-rules · modificado 2026-08-23 09:14

  v  Ver contenido
  e  Exportar a archivo
  i  Importar desde archivo
  r  Restaurar contenido por defecto

  esc volver
```

| Tecla | Acción | Resultado |
|---|---|---|
| `v` | Ver | Muestra el contenido con desplazamiento, sin salir de la TUI |
| `e` | Exportar | Pide ruta; confirma con la ruta escrita y el número de líneas |
| `i` | Importar | Pide ruta; aplica la misma semántica de §3; el error se muestra en pantalla sin perder el documento |
| `r` | Restaurar | **Pide confirmación** antes de descartar el contenido personalizado |

**Paridad exigida (FR-041)**: las cuatro operaciones de la CLI existen en la TUI. Un
test de contrato recorre el catálogo y comprueba que ambas superficies ofrecen el
mismo conjunto, para que no se separen con el tiempo.

---

## 5. Relación con `mem export` / `mem import`

Coexisten y no se solapan (FR-046):

| | `mem export` / `mem import` | `mem docs export` / `mem docs import` |
|---|---|---|
| Alcance | Toda la memoria del proyecto + relaciones | Un documento fijado |
| Formato | Volcado estructurado para intercambio | Texto plano editable a mano |
| Uso típico | Mover la memoria entre máquinas o proyectos | Poner las reglas del equipo |
| Sobre lo existente | Añade con dedup | Reemplaza ese documento |

Ninguno reemplaza al otro y ambos siguen disponibles sin cambios.
