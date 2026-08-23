# Contrato — Siembra de memorias por defecto

**Componente**: `application/usecases/seed_defaults.go` (nuevo)
**Consumidores**: `CmdInstall`, `CmdMCP`

---

## Puerto requerido

```go
// application/ports/memory_topic.go (nuevo)

// MemoryTopicQuerier resuelve una memoria por su clave de tópico, sin depender
// de cuán reciente sea. Puerto mínimo y aparte de MemoryLister a propósito: el
// constructor de contexto solo necesita esta capacidad, no la escritura.
type MemoryTopicQuerier interface {
    // ByTopicKey devuelve la memoria con esa clave en el proyecto, o
    // (nil, nil) si no existe. Un error solo señala un fallo real de lectura,
    // nunca "no encontrado" — constitución, principio V, manejo de errores.
    ByTopicKey(project, topicKey string) (*domain.Memory, error)
}
```

```go
// MemorySeeder inserta una memoria por la vía INERTE: sin sinapsis automática y
// sin publicación a sistemas externos de documentación. Puerto aparte de la
// escritura normal a propósito — el llamador declara que está sembrando, no
// guardando el fruto de una sesión de trabajo.
type MemorySeeder interface {
    InsertSeed(m *domain.Memory) (int64, error)
}
```

`persistence.MemoryRepository` implementa ambos; el composition root los inyecta.

## Firma del caso de uso

```go
// Seed describe una memoria semilla a sembrar.
type Seed struct {
    TopicKey string
    Type     domain.MemoryType
    Title    string
    Content  string
}

// SeedDefaults siembra las memorias que faltan y devuelve las claves de tópico
// realmente creadas (vacío = no había nada que sembrar).
func SeedDefaults(
    seeder ports.MemorySeeder,
    topics ports.MemoryTopicQuerier,
    project string,
    seeds []Seed,
) (created []string, err error)
```

## Contrato de comportamiento

| # | Precondición | Postcondición |
|---|---|---|
| C1 | `Content` en blanco tras `TrimSpace` | La semilla se omite; no cuenta como creada; sin error |
| C2 | Ya existe memoria con ese `TopicKey` | No se escribe nada; no cuenta como creada; sin error |
| C3 | No existe | Se inserta **por la vía inerte** con `SessionID` y `Filepath` vacíos; su clave se devuelve en `created` |
| C4 | `topics` es `nil` | Retorna `(nil, nil)` sin escribir — degradación limpia, no pánico |
| C5 | La consulta por clave falla | Esa semilla se omite y el error se propaga en `err`; las demás se intentan igual |
| C6 | Llamada repetida sobre el mismo proyecto | `created` vacío a partir de la segunda vez (idempotencia, constitución principio V.7) |

### Invariantes

1. **Nunca un `UPDATE`.** La única escritura posible es un `INSERT` sobre una clave
   inexistente (C2). Una semilla existente es propiedad de la persona.
2. **Siembra inerte.** La inserción no crea relaciones automáticas (FR-033) ni
   publica a sistemas externos de documentación, ni siquiera con la sincronización
   activada (FR-034). Escribe en `memories` y `memory_search`, y en nada más.
3. **Texto idéntico al origen.** Lo persistido coincide carácter por carácter con el
   `Content` recibido (FR-032). La depuración de secretos **sigue activa** — no se
   desactiva por comodidad —; lo que garantiza la igualdad es que las plantillas no
   contienen nada que matchee, y un test guardián lo verifica en cada ejecución en
   vez de confiar en que siga siendo cierto.

### Contrato de las brechas cerradas (research R4)

| # | Verificación exigida al test | Falla si |
|---|---|---|
| G1 | Tras sembrar, `Content` almacenado == plantilla de origen | Una edición futura de la plantilla introduce una cadena parecida a un secreto |
| G2 | Tras sembrar, 0 filas nuevas en `memory_relations` | Alguien asocia la siembra a la sesión activa y reactiva `formSynapse` |
| G3 | Con `adr_sync_enabled = true` y proveedor simulado, 0 intentos de export | Alguien enruta la siembra por `InsertMemory` en vez de `InsertSeed` |

## Semillas por defecto

Las provee el llamador, no el caso de uso — así los tests inyectan las suyas sin
depender de `TemplatesFS`:

| Clave | Tipo | Título | Contenido |
|---|---|---|---|
| `gomemory:work-rules` | `preference` | `Reglas de trabajo del proyecto` | `templates/agent-preamble.md` |
| `gomemory:constitution` | `architecture` | `Constitución del proyecto (spec-kit)` | `templates/speckit-constitution-gen.md` |

## Puntos de invocación

| Sitio | Momento | Ante error |
|---|---|---|
| `CmdInstall` | Tras verificar/inicializar la memoria, antes de la limpieza | Avisa con `⚠️` y continúa |
| `CmdMCP` | Tras el auto-arranque de sesión, antes de `server.Run` | `log.Printf` y continúa; nunca fatal |

**Prohibido**: que un fallo de siembra interrumpa la instalación o impida arrancar el
servidor MCP. Es una capa oportunista, igual que el auto-arranque de sesión.
