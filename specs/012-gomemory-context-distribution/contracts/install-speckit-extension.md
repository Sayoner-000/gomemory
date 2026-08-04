# Contrato: `setup.InstallSpeckitExtension`

Función nueva en `adapters/primary/setup/speckit_extension.go`, en el
mismo paquete que `InstallOpenCode`/`InstallClaudeCode`.

## Firma

```go
// InstallSpeckitExtension copia el brazo extensor gomemory-context (spec
// 011) al proyecto destino, si y solo si ya tiene spec-kit inicializado
// (.specify/ presente). No-op silencioso en caso contrario — nunca es un
// error que un proyecto no use spec-kit. templatesFS es el mismo fs.FS que
// ya usa embeddedTemplate() en cmd_install.go (cli.TemplatesFS), con rutas
// bajo el prefijo "templates/" (así lo embebe "go:embed all:templates" —
// ver embeddedTemplate(), que lee "templates/"+name). Si templatesFS es
// nil (p. ej. en tests que no lo inyectan), retorna nil sin hacer nada —
// mismo criterio de degradación con gracia que embeddedTemplate().
func InstallSpeckitExtension(root string, templatesFS fs.FS) error
```

Rutas fuente esperadas dentro de `templatesFS` (todas bajo el prefijo
`templates/`, igual que `embeddedTemplate()`):

- `templates/gomemory-context/extension/**` (árbol completo)
- `templates/gomemory-context/claude/speckit-gomemory-context-update/SKILL.md`
- `templates/gomemory-context/opencode/speckit.gomemory-context.update.md`

## Implementación: 3 llamadas a `InstallPlugin`, sin función nueva de copia

`InstallPlugin(fsys fs.FS, pluginDir, targetDir string, ctx *PluginContext)
(int, error)` (ya existe en `setup.go`, usada hoy por
`installOpenCodePlugin`) copia recursivamente todo lo que hay bajo
`pluginDir` a `targetDir`, con la semántica de escritura exacta que pide
esta spec (ver research.md #3). El layout de las plantillas embebidas se
diseñó a propósito para que el `pluginDir` de cada llamada sea justo el
subárbol que corresponde a cada destino — no hace falta un helper de copia
de archivo suelto:

```go
func InstallSpeckitExtension(root string, templatesFS fs.FS) error {
    if templatesFS == nil {
        return nil
    }
    if fi, err := fs.Stat(templatesFS, filepath.Join(root, ".specify")); ... // ver nota abajo
    // (chequeo real: os.Stat sobre el filesystem del SO, no sobre templatesFS)
    if info, err := osStat(filepath.Join(root, ".specify")); err != nil || !info.IsDir() {
        return nil
    }
    base := "templates/gomemory-context"
    if _, err := InstallPlugin(templatesFS, base+"/extension", filepath.Join(root, ".specify", "extensions", "gomemory-context"), nil); err != nil {
        return err
    }
    if _, err := InstallPlugin(templatesFS, base+"/claude", filepath.Join(root, ".claude", "skills"), nil); err != nil {
        return err
    }
    if _, err := InstallPlugin(templatesFS, base+"/opencode", filepath.Join(root, ".opencode", "commands"), nil); err != nil {
        return err
    }
    return chmodBashScript(filepath.Join(root, ".specify", "extensions", "gomemory-context", "scripts", "bash", "update-gomemory-context.sh"))
}
```

(Pseudocódigo ilustrativo — `/speckit-tasks`/`/speckit-implement` deciden
la forma final exacta; lo que fija el contrato es: 3 llamadas a
`InstallPlugin` con los `pluginDir`/`targetDir` de arriba, más el chmod
explícito del script bash.)

## Precondiciones / Postcondiciones

Ver `data-model.md` — mismo contrato, expresado aquí como firma Go.

## Entrada embebida

No hace falta ninguna directiva `go:embed` nueva ni tocar
`infrastructure/main.go`: `gomemory-context/` es un subdirectorio más bajo
`infrastructure/templates/`, ya cubierto por la directiva `all:templates`
existente. El llamador (`cmd_install.go`, paquete `cli`) ya tiene acceso a
ese árbol vía su propia variable `TemplatesFS` y la pasa directo como
argumento — así `adapters/primary/setup` no necesita importar el paquete
`cli` (evita un ciclo de imports, dado que `cli` ya importa `setup` para
`InstallOpenCode`/`InstallClaudeCode`).

## Comportamiento esperado (tabla de verdad)

| `.specify/` en `root` | Resultado |
|---|---|
| No existe | Retorna `nil` inmediatamente. Cero archivos creados/tocados. |
| Existe, extensión nunca instalada | Crea los 3 destinos completos (ver `data-model.md`). |
| Existe, extensión instalada e idéntica a la embebida | No reescribe ningún archivo. |
| Existe, extensión instalada pero con contenido de una versión anterior | Sobrescribe solo los archivos que difieren. |
| Existe, extensión instalada y editada a mano por la persona | Se sobrescribe igual (ver research.md #3 — comportamiento heredado y ya aceptado del framework, no un caso especial). |

## Manejo de errores

- Errores de I/O al copiar (permisos, disco lleno) se retornan (`error`),
  igual que el resto de pasos de `cmd_install.go` — el llamador decide si
  imprime un `⚠️` y continúa (mismo patrón que la copia de la
  constitución) o aborta. Dado que es un paso no crítico (degradación
  aceptable: el resto de `mem install` debe completarse igual), el
  llamador en `cmd_install.go` imprime la advertencia y continúa, no aborta
  el instalador completo.
