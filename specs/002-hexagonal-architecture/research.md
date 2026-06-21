# Research: Reorganización a Arquitectura Hexagonal

## 1. Manejo de `internal/` Package Visibility

**Contexto**: `internal/server/` y `internal/setup/` actualmente se benefician de la restricción de visibilidad de Go: solo el módulo `mem` puede importarlos. Al moverlos a `adapters/primary/mcp/` y `adapters/primary/setup/`, pierden esta protección.

**Decisión**: Migrar fuera de `internal/`. La protección de visibilidad se reemplaza por convención de capas hexagonales:
- Ningún código fuera del módulo `mem` debería importar estos paquetes
- La regla se refuerza con `golangci-lint` (dependencia circular detector) y code review
- No se anidan bajo un nuevo `internal/` para mantener la estructura plana de 4 capas

**Alternativas consideradas**:
- Mantener `internal/` como contenedor de toda la lógica y solo exponer `adapters/` → añade profundidad innecesaria
- Usar `go vet` custom para verificar imports → sobreingeniería para este proyecto

**Riesgo**: Bajo. El proyecto es monomódulo y monousuario.

---

## 2. Embed Directive (`//go:embed plugin`)

**Contexto**: `main.go` usa `//go:embed plugin` para incrustar los archivos de plugin TypeScript en el binario. El `pluginFS` embed.FS se asigna a `setup.PluginFS` en `init()`. Al mover `main.go` a `infrastructure/main.go` y `internal/setup/` a `adapters/primary/setup/`, el path del embed cambia.

**Decision**: 
- El `//go:embed plugin` actual referencia la carpeta `plugin/` en la raíz del proyecto
- El directorio `plugin/` permanece en la raíz (es contenido estático, no código Go)
- `infrastructure/main.go` puede usar `//go:embed ../plugin` o mejor: mover a raíz y mantener package main

**Alternativa preferida**: Mantener `main.go` en la raíz como composition root liviano, o mover `plugin/` dentro de `infrastructure/` y ajustar el path del embed. La opción más limpia es mover `plugin/` a `infrastructure/plugin/` y cambiar el embed a `//go:embed plugin`.

**Riesgo**: Medio. Si `main.go` se mueve a `infrastructure/`, el path relativo del embed cambia. Hay que verificar que `//go:embed` soporta paths con `..` (sí, desde Go 1.16, pero solo dentro del módulo).

**Decisión final**: Mover `plugin/` a `infrastructure/plugin/` para mantener el embed dentro del mismo paquete `infrastructure`. El `//go:embed plugin` funciona con path relativo al directorio del archivo fuente.

---

## 3. Ubicación del Paquete `context/`

**Contexto**: `context/builder.go` es un servicio que orquesta `store.*` + `types.*` para leer memorias, sesiones y escribirlas en `.memory/context.md`. Depende de `store` directamente.

**Decisión**: Migrar a `application/usecases/build_context.go` como un caso de uso:
- Define una interfaz `ContextBuilder` en `application/ports/context_builder.go`
- La implementación concreta se inyecta desde el composition root
- El adaptador CLI (`cmd_context.go`) solo conoce la interfaz, no la implementación

**Alternativas consideradas**:
- Mover a `adapters/primary/context/` (como adaptador, no como lógica de negocio) → incorrecto porque la lógica de orquestación pertenece a la capa de aplicación
- Mantener como paquete separado `context/` fuera de las capas → viola el Principio I

**Riesgo**: Bajo. Es una extracción de interfaz directa.

---

## 4. Dependencia Circular Potencial

**Contexto**: Actualmente `store` importa `types`, varios paquetes importan `store`. Al extraer interfaces a `application/ports/`, las implementaciones en `adapters/secondary/persistence/` importarán `application/ports/` y `domain/`. La capa `application/` solo importa `domain/`. No hay riesgo de ciclo siempre que las interfaces se definan en `application/ports/` y las implementaciones estén en `adapters/`.

**Decisión**: No requiere acción. El grafo de dependencias es acíclico por construcción:
```
domain/ ← application/ ← adapters/primary/ + adapters/secondary/ + infrastructure/
```

**Riesgo**: Ninguno.

---

## 5. Tests: Actualización de Imports

**Contexto**: `tests/contract/memory_protocol_test.go` y `tests/integration/plugin_integration_test.go` importan `mem/store`. Al mover `store/` a `adapters/secondary/persistence/`, estos imports se rompen.

**Decisión**: Actualizar los imports en tests existentes al nuevo path. Esto es un cambio puramente mecánico (reemplazar `mem/store` por `mem/adapters/secondary/persistence`). No se modifica lógica de tests.

**Excepción constitucional**: El Principio III dice "tests existentes son intocables". Esta migración requiere una excepción controlada: actualizar imports no es modificar lógica, es mantener la referencia correcta al código migrado. Se documenta aquí como excepción aprobada.

**Riesgo**: Bajo. `git mv` preserva el contenido del archivo; solo se actualiza el import path.

---

## 6. Comandos CLI en Package Separat

**Contexto**: Los 16 archivos `cmd_*.go` actualmente comparten `package main` y acceden a variables/funciones entre sí y con `main.go`. Para migrarlos a `adapters/primary/cli/`, necesitan un package name diferente (ej. `package cli`).

**Decisión**: Migrar todos los `cmd_*.go` a `adapters/primary/cli/` con `package cli`. Las funciones exportadas se convierten en `CmdInit`, `CmdSave`, etc. El composition root en `infrastructure/main.go` llama a `cli.CmdInit()`, `cli.CmdSave()`, etc.

**Dependencias compartidas**: 
- `fail()` y `fatalf()` en `main.go` se mueven a `adapters/primary/cli/cli.go` como helpers del paquete
- `usage()` se mueve a `adapters/primary/cli/usage.go`
- `launchTUI()` se convierte en `cli.LaunchTUI()` o se integra en el dispatcher

**Riesgo**: Medio. Hay que revisar cada archivo `cmd_*.go` para detectar llamadas entre comandos (si existen) y asegurar que todas las funciones referenciadas sean exportadas o estén en el mismo paquete.

---

## 7. MCP Server: Setup y PluginFS

**Contexto**: `setup.PluginFS` se asigna en `init()` de `main.go` via `setuppluginFS`. Este embed contiene los plugins TypeScript que el setup escribe al instalar.

**Decisión**: 
- `//go:embed plugin` se mueve a `infrastructure/main.go` (o al archivo que sea composition root)
- El `PluginFS` se inyecta vía constructor de `SetupService` en el composition root, no via `init()` ni variable global de paquete

**Alternativa**: Usar singleton con asignación en `init()` → se mantiene como está pero se mueve el `init()` al nuevo main. Se prefiere inyección explícita para cumplir con hexagonal.

**Riesgo**: Bajo. Solo cambia el mecanismo de entrega del embed.

---

## Resumen de Decisiones

| Tema | Decisión |
|------|----------|
| `internal/` packages | Migrar fuera, confiar en convención + linter |
| `//go:embed plugin` | Mover `plugin/` a `infrastructure/plugin/` |
| `context/builder.go` | Caso de uso en `application/usecases/` |
| Ciclos de dependencia | No hay riesgo, grafo acíclico |
| Tests imports | Excepción controlada: actualizar solo import paths |
| `cmd_*.go` package | `package cli` en `adapters/primary/cli/` |
| `PluginFS` | Inyección por constructor, no global |
