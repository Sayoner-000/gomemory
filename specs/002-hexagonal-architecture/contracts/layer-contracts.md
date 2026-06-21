# Contratos entre Capas Hexagonales

## Contrato 1: Dominio → Aplicación

**Origen**: `domain/`
**Destino**: `application/ports/`
**Regla**: `domain/` NO importa nada del proyecto. `application/ports/` importa `domain/` para tipos.
**Verificación**: `go vet` + `golangci-lint` — cualquier import de `adapters/` o `infrastructure/` dentro de `domain/` es ERROR.

```go
// domain/memory.go — tipos puros
package domain
// NO imports de otros paquetes del proyecto

// application/ports/memory_repository.go — interfaz
package ports
import "mem/domain"  // ✓ Único import permitido en application/
```

---

## Contrato 2: Puertos → Adaptadores Secundarios

**Origen**: `application/ports/` (interfaces)
**Destino**: `adapters/secondary/persistence/` (implementaciones)
**Regla**: Los adaptadores secundarios implementan las interfaces de `ports/`. El composition root inyecta la implementación concreta.

```go
// adapters/secondary/persistence/memory_repo.go
package persistence
import (
    "mem/domain"
    "mem/application/ports"
)

// MemoryRepository implementa ports.MemoryRepository
type MemoryRepository struct { db *sql.DB }
func (r *MemoryRepository) Save(m *domain.Memory) error { ... }
```

**Verificación**: `adapters/secondary/` NO debe importar `adapters/primary/` ni `infrastructure/`.

---

## Contrato 3: Composition Root → Todo

**Origen**: `infrastructure/`
**Destino**: Todos los adaptadores y casos de uso
**Regla**: `infrastructure/main.go` es el ÚNICO lugar donde se construyen e inyectan dependencias. Ningún adaptador primario construye sus propias dependencias.

```go
// infrastructure/main.go — composition root
func main() {
    db := persistence.OpenDB(root)
    memRepo := persistence.NewMemoryRepository(db)
    sessionRepo := persistence.NewSessionRepository(db)
    
    // Inyectar en adaptadores primarios
    cli.Run(memRepo, sessionRepo, ...)
}
```

**Verificación**: Buscar `store.Open()`, `store.FindRoot()`, `sql.Open()` fuera de `infrastructure/` → ERROR.

---

## Contrato 4: CLI como Adaptador (no como dispatcher)

**Origen**: `adapters/primary/cli/`
**Regla**: Los comandos CLI reciben dependencias por constructor/inyección. No llaman a `store.Open()` ni a ningún adaptador secundario directamente.

**Antes (violación)**:
```go
func cmdSave(args []string) {
    root, _ := store.FindRoot()
    db, _ := store.Open(root)   // ← violación: acopla CLI a persistencia
    defer db.Close()
    store.SaveMemory(db, ...)   // ← violación
}
```

**Después (correcto)**:
```go
// adapters/primary/cli/cmd_save.go
func CmdSave(args []string, repo ports.MemoryRepository) {
    repo.Save(memory)           // ← correcto: usa interfaz
}
```

---

## Contrato 5: Tests y Migration de Imports

**Origen**: `tests/`
**Regla**: Los tests existentes actualizan SOLAMENTE sus import paths. No se modifica lógica.

**Antes**: `import "mem/store"` → **Después**: `import "mem/adapters/secondary/persistence"`

**Excepción**: El Principio III (tests intocables) se flexibiliza ÚNICAMENTE para actualizar import paths durante la migración. Cualquier otro cambio requiere autorización explícita.

---

## Contrato 6: Embed y PluginFS

**Origen**: `infrastructure/main.go`
**Destino**: `adapters/primary/setup/`
**Regla**: El `//go:embed plugin` se define en `infrastructure/main.go`. El `EmbedFS` se pasa al adaptador de setup via constructor, no via variable global ni `init()`.

```go
// infrastructure/main.go
//go:embed plugin
var pluginFS embed.FS

func main() {
    setupSvc := setup.NewSetupService(pluginFS)
    // ...
}
```

```go
// adapters/primary/setup/setup.go
type SetupService struct {
    pluginFS fs.FS
}

func NewSetupService(pluginFS fs.FS) *SetupService {
    return &SetupService{pluginFS: pluginFS}
}
```
