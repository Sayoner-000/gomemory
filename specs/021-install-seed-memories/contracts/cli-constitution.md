# Contrato — `mem constitution`

**Componente**: `adapters/primary/cli/cmd_constitution.go` (nuevo)

---

## Sintaxis

```
mem constitution [--sync]
```

| Flag | Efecto |
|---|---|
| (ninguno) | Escribe el texto íntegro de la constitución vigente en stdout |
| `--sync` | Además, escribe `.specify/memory/constitution.md` si el proyecto usa spec-kit |

## Resolución del contenido

```
1. ByTopicKey(project, "gomemory:constitution")
   ├─ encontrada ──> imprimir su Content (versión vigente, posiblemente editada)
   └─ ausente
      ├─ plantilla embebida disponible ──> imprimirla + aviso a stderr
      └─ plantilla no disponible ────────> error, exit != 0
```

**Regla**: stdout lleva SOLO el documento. Cualquier aviso va a stderr, para que
`mem constitution > archivo.md` produzca un documento limpio.

## Contrato de salida

| Caso | stdout | stderr | Exit |
|---|---|---|---|
| Semilla presente | Documento íntegro | — | 0 |
| Semilla editada por la persona | El texto editado, no la plantilla | — | 0 |
| Semilla ausente, plantilla disponible | Plantilla íntegra | `⚠️ No hay constitución en la memoria de este proyecto; se muestra la plantilla de referencia.` | 0 |
| Ni semilla ni plantilla | — | Mensaje de error | ≠ 0 |
| `--sync` con `.specify/` presente | Documento íntegro | `✅ .specify/memory/constitution.md actualizado` | 0 |
| `--sync` sin `.specify/` | Documento íntegro | `ℹ️ Este proyecto no usa spec-kit; no se sincronizó ningún archivo.` | 0 |

**`--sync` nunca crea `.specify/`**: misma comprobación que
`InstallSpeckitExtension` (`setup/speckit_extension.go:38`) — si el directorio no
existe, no se toca nada.

## Registro

- `dispatcher.go`: `case "constitution": CmdConstitution(deps, rest)`
- `cli.go`: línea de ayuda `mem constitution [--sync]   Mostrar la constitución vigente del proyecto`
