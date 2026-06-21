# Data Model: Arquitectura Hexagonal

## Entidades de Dominio (`domain/`)

### Memory
```go
package domain

type MemoryType string
const (
    MemoryTypeLearning     MemoryType = "learning"
    MemoryTypeDecision     MemoryType = "decision"
    MemoryTypeArchitecture MemoryType = "architecture"
    MemoryTypeBugfix       MemoryType = "bugfix"
    MemoryTypePattern      MemoryType = "pattern"
    MemoryTypeDiscovery    MemoryType = "discovery"
)

type Memory struct {
    ID        string
    Project   string
    SessionID string
    Type      MemoryType
    Title     string
    Content   string
    Filepath  string
    CreatedAt time.Time
    UpdatedAt time.Time
}

func ValidMemoryType(t MemoryType) bool { ... }
```

### Session
```go
type Session struct {
    ID        string
    Project   string
    Summary   string
    CreatedAt time.Time
    EndedAt   *time.Time
}

func NewSessionID() string { ... }   // crypto/rand UUID
```

### Relation
```go
type RelationType string
const (
    RelationRelated      RelationType = "related"
    RelationCompatible   RelationType = "compatible"
    RelationScoped       RelationType = "scoped"
    RelationConflicts    RelationType = "conflicts_with"
    RelationSupersedes   RelationType = "supersedes"
    RelationNotConflict  RelationType = "not_conflict"
)

type Relation struct {
    ID         string
    Project    string
    MemoryIDA  string
    MemoryIDB  string
    Relation   RelationType
    Confidence float64
    Reasoning  string
    CreatedAt  time.Time
}
```

### Domain Errors
```go
var (
    ErrNotFound      = errors.New("not found")
    ErrValidation    = errors.New("validation error")
    ErrAlreadyExists = errors.New("already exists")
)
```

### Proyecto
```go
type Project struct {
    Root string
    Name string
}
```

---

## Puertos de Aplicación (`application/ports/`)

### MemoryRepository
```go
type MemoryRepository interface {
    Save(m *domain.Memory) error
    FindByID(id string) (*domain.Memory, error)
    List(project string, limit int) ([]domain.Memory, error)
    Search(project, query string, limit int) ([]domain.Memory, error)
    Update(m *domain.Memory) error
    Delete(id string) error
}
```

### SessionRepository
```go
type SessionRepository interface {
    Create(s *domain.Session) error
    FindByID(id string) (*domain.Session, error)
    Update(s *domain.Session) error
    Active(project string) (*domain.Session, error)
    Recent(project string, limit int) ([]domain.Session, error)
    Close(id, summary string) error
}
```

### RelationRepository
```go
type RelationRepository interface {
    Save(r *domain.Relation) error
    List(project string, limit int) ([]domain.Relation, error)
}
```

### SettingsRepository
```go
type SettingsRepository interface {
    Get(project string) (map[string]string, error)
    Set(project, key, value string) error
}
```

### ContextBuilder
```go
type ContextBuilder interface {
    Build(project string) (string, error)
    Write(project, root string) (string, error)
}
```

### ProjectRepository
```go
type ProjectRepository interface {
    FindRoot() (string, error)
    Current() (*domain.Project, error)
    MemDir(root string) string
}
```

---

## Relaciones entre Capas

```
domain/ (tipos puros, sin imports del proyecto)
  │
  ▼ importa
application/ports/ (interfaces que usan tipos de domain/)
  │
  ├──► adapters/primary/cli/    — implementa llamadas a puertos
  ├──► adapters/primary/tui/    — implementa llamadas a puertos
  ├──► adapters/primary/mcp/    — implementa llamadas a puertos
  │
  └──► adapters/secondary/persistence/ — implementa repositorios (SQLite)
```

## Reglas de Validación

- `Memory.Title` no puede estar vacío
- `Memory.Type` debe ser uno de los tipos válidos (validado por `ValidMemoryType()`)
- `Memory.Content` no puede estar vacío
- `Session.ID` debe ser único por proyecto
- `Relation.Confidence` debe estar entre 0.0 y 1.0
- `Relation.MemoryIDA` y `MemoryIDB` deben referenciar memorias existentes
