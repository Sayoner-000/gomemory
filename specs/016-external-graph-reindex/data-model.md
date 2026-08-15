# Data Model: Reindexado dual de grafos de código + edición de huella de contexto en TUI

Esta funcionalidad no agrega tablas SQLite ni entidades de dominio persistentes nuevas. Extiende
un puerto de aplicación y reutiliza el modelo de settings ya existente. Se documentan aquí como
"entidades" en el sentido de datos que fluyen entre capas, no de nuevas filas de base de datos.

## CodeGraphIndexer (puerto de aplicación)

Capacidad opcional que un proveedor de grafo de código externo puede implementar, separada de
`CodeGraphProvider` (que mantiene su contrato de no-bloqueo intacto).

| Campo/Método | Tipo | Descripción |
|---|---|---|
| `Name()` | `string` | Identificador legible del proveedor (para mensajes al usuario) |
| `IndexRepository(ctx, mode)` | `(nodes int, edges int, err error)` | Dispara el reindexado bloqueante; `mode` siempre `"full"` en esta funcionalidad |

**Relaciones**: implementado por `codebasememory.Provider` (adaptador secundario existente).
Consumido por `cmd_index.go` (CLI) y `tui.go` (TUI), ambos vía el tipo de puerto, nunca el tipo
concreto.

**Reglas de validación / invariantes**:
- Sin binario resuelto (`binPath == ""`) → `err` es `ports.ErrIndexerNotInstalled` (sentinel,
  verificable con `errors.Is`), `nodes`/`edges` en cero.
- Respuesta del proceso externo no parseable como el fixture esperado → error envuelto
  (`fmt.Errorf("index_repository: ...")`), no el sentinel de "no instalado".

## ErrIndexerNotInstalled (sentinel de error)

Vive en `application/ports`, no en el adaptador. Único propósito: permitir a los llamadores
distinguir "el proveedor no está instalado" (mensaje informativo, no es un fallo) de cualquier
otro error real de indexado (advertencia).

## RespuestaIndexRepository (forma de datos externa, no persistida)

Estructura de la respuesta JSON que devuelve el proceso externo `codebase-memory-mcp cli
index_repository`, verificada en vivo:

| Campo | Tipo | Uso en esta funcionalidad |
|---|---|---|
| `project` | string | No usado por el parser de esta funcionalidad |
| `status` | string | No usado por el parser de esta funcionalidad |
| `excluded.dirs` / `excluded.count` | array/int | No usado por el parser de esta funcionalidad |
| `nodes` | int | Extraído y reportado al usuario |
| `edges` | int | Extraído y reportado al usuario |
| `adr_present` | bool | No usado por el parser de esta funcionalidad |

Solo `nodes`/`edges` se extraen (`parseIndexRepositoryResponse`); el resto de campos se ignora a
propósito — no hay necesidad actual de propagarlos más allá del mensaje de éxito.

## Ajustes de huella de contexto (modelo ya existente, sin cambios de esquema)

Ya definidos en `adapters/secondary/persistence/settings.go`; esta funcionalidad solo agrega una
vía de edición nueva (TUI), no cambia el modelo:

| Ajuste | Campo | Semántica |
|---|---|---|
| Presupuesto | `Budget` | `0` → `DefaultBudget` (24000); negativo → sin límite |
| Umbral de compactación | `CompactThreshold` | `0` → `DefaultCompactThreshold` (48000); negativo → desactivado |
| Ventana de deduplicación | `DedupWindowDays` | `0` → `DefaultDedupWindowDays` (7); negativo → desactivada |

La normalización (0 → default, negativo → opt-out) ocurre en `ReadSettings`, no en el punto de
escritura — la TUI persiste el entero tal cual lo ingresó el usuario, sin reinterpretarlo.

## Estados de la pantalla de edición (TUI)

Modelo de estados de `screenEditSetting`, sin persistencia propia — vive en memoria del proceso
de la TUI mientras esa pantalla está activa:

```text
screenConfig
   │  (usuario selecciona fila de un ajuste)
   ▼
screenEditSetting (editSettingField = Budget|CompactThreshold|DedupDays)
   │
   ├── Esc ──────────────────────────────► screenConfig (sin guardar)
   │
   ├── Enter, entrada no numérica/vacía/decimal ──► permanece en screenEditSetting (editSettingErr)
   │
   └── Enter, entero válido ──► persiste vía settingsRepo.Write ──► screenConfig (statusMsg de éxito)
```

## Estados del reindexado externo (TUI)

```text
configView (fila "Reindexar grafo externo")
   │  (usuario presiona Enter)
   ▼
¿m.codeProvider implementa CodeGraphIndexer?
   │
   ├── No ──► statusMsg = "no disponible" (sin disparar tea.Cmd)
   │
   └── Sí ──► ¿ya hay un reindexado en curso?
              │
              ├── Sí ──► statusMsg = "ya en curso" (sin disparar un segundo tea.Cmd)
              │
              └── No ──► statusMsg = "Indexando..." + dispara reindexExternalGraphCmd()
                              │
                              ▼ (async, la TUI sigue respondiendo)
                         externalReindexDoneMsg{nodes, edges, err}
                              │
                              ├── err == nil ──► statusMsg = "N nodos, M aristas"
                              ├── errors.Is(err, ErrIndexerNotInstalled) ──► statusMsg = "no disponible"
                              └── otro err ──► statusMsg = advertencia con el error
```
