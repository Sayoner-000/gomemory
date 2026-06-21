# Guía de Validación: Migración Hexagonal

## Prerrequisitos

- Go 1.25+
- `golangci-lint` instalado
- Repositorio clonado y en estado limpio (`git status` sin cambios)

## Pasos de Validación

### 1. Estado Inicial (antes de migrar)

```bash
cd /Users/josegomezj/home/rcw/go_memory

# Verificar que todo compila y tests pasan ANTES
go build ./...
go test ./...

# Registrar el estado actual
git log --oneline -5
git diff --stat HEAD
```

**Esperado**: Build exitoso, todos los tests verdes.

---

### 2. Migración por Lotes

Cada lote se migra con `git mv` y se verifica individualmente:

#### Lote 1: Domain (más puro, sin dependencias)

```bash
mkdir -p domain
git mv types/types.go domain/       # types.go → domain/memory.go + session.go + relation.go
# Dividir types.go en domain/memory.go, domain/session.go, domain/relation.go
go build ./...
go test ./...
```

**Esperado**: Build exitoso.

#### Lote 2: Application (ports + use cases)

```bash
mkdir -p application/ports application/usecases
# Crear interfaces en application/ports/
# Mover context/builder.go a application/usecases/
go build ./...
```

**Esperado**: Build exitoso.

#### Lote 3: Adapters Secondary (persistence)

```bash
mkdir -p adapters/secondary/persistence
git mv store/ adapters/secondary/persistence/
# Actualizar package name de store a persistence
go build ./...
```

**Esperado**: Build exitoso, imports rotos resueltos.

#### Lote 4: Adapters Primary (CLI, TUI, MCP, Setup)

```bash
mkdir -p adapters/primary/cli adapters/primary/tui adapters/primary/mcp adapters/primary/setup
git mv cmd_*.go adapters/primary/cli/
git mv tui/tui.go adapters/primary/tui/
git mv internal/server/ adapters/primary/mcp/
git mv internal/setup/ adapters/primary/setup/
go build ./...
```

**Esperado**: Build exitoso.

#### Lote 5: Infrastructure (composition root)

```bash
mkdir -p infrastructure
# Mover plugin/ a infrastructure/plugin/
git mv plugin infrastructure/plugin
# Crear infrastructure/main.go (composition root liviano)
# main.go original se elimina (git rm)
go build ./...
```

**Esperado**: Build exitoso.

---

### 3. Verificación Post-Migración

```bash
# Compilación completa
go build ./...

# Todos los tests
go test ./...

# Linter
golangci-lint run ./...

# Verificar estructura hexagonal
ls -d domain/ application/ adapters/ infrastructure/
```

**Esperado**:
- `go build ./...` → éxito
- `go test ./...` → todos verdes
- `golangci-lint run ./...` → sin errores
- Los 4 directorios hexagonales existen

---

### 4. Verificación Funcional

```bash
# Compilar binario
go build -o mem .

# Probar comandos clave
./mem help
./mem init
./mem save -t "test" -y decision "migración hexagonal completada"
./mem list
./mem search "hexagonal"
./mem context
./mem session start
./mem session end -s "verificación post-migración"
```

**Esperado**: Todos los comandos funcionan igual que antes.

---

### 5. Verificación de Dependencias

```bash
# Verificar que domain/ no importa nada del proyecto
grep -r "mem/" domain/*.go | grep -v "_test.go" | grep "import" || echo "✓ domain/ no tiene imports del proyecto"

# Verificar que application/ solo importa domain/
grep -r "mem/" application/*.go | grep -v "_test.go" | grep -v "mem/domain" | grep "import" && echo "✗ application/ importa algo que no es domain/"

# Verificar tests actualizados
grep "mem/store" tests/ -r && echo "✗ tests aún referencian store/ viejo" || echo "✓ tests actualizados"
```

---

### 6. Verificación de Historial Git

```bash
# Verificar que git mv preservó historial
git log --follow --oneline adapters/secondary/persistence/db.go | head -3
git log --follow --oneline adapters/primary/cli/cmd_init.go | head -3
```

**Esperado**: `git log --follow` muestra commits anteriores a la migración.

---

## Criterios de Aceptación

| # | Criterio | Comando de Verificación |
|---|----------|------------------------|
| SC-001 | 14 comandos CLI funcionan | `./mem help` lista todos |
| SC-002 | Build exitoso | `go build ./...` |
| SC-003 | Tests pasan | `go test ./...` |
| SC-004 | 4 capas existen | `ls -d domain/ application/ adapters/ infrastructure/` |
| SC-005 | Sin .go en raíz | `ls *.go 2>/dev/null \|\| echo "✓"` |
| SC-006 | domain/ sin imports del proyecto | Verificación paso 5 |
| SC-007 | Sin código duplicado | `git diff --stat` solo muestra movimientos |
| SC-008 | Historial preservado | `git log --follow` muestra commits previos |
