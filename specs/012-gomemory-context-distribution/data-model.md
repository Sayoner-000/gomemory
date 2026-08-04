# Data Model: Distribuir el brazo extensor gomemory-context vía `mem install`, transversal a agentes

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

No hay entidades de dominio ni persistencia en base de datos. Esta feature
solo mueve archivos de plantilla desde el binario hacia el filesystem del
proyecto destino. Se documentan aquí las "entidades" de archivo para
trazabilidad con `spec.md`.

## Plantillas embebidas del brazo extensor

Árbol embebido en el binario (`infrastructure/templates/gomemory-context/`,
vía `go:embed all:templates`, mismo mecanismo que
`speckit-constitution-gen.md`). Tres subárboles, cada uno con su propio
destino en el proyecto instalado:

| Subárbol embebido | Destino en el proyecto | Condición de copia |
|---|---|---|
| `gomemory-context/extension/**` | `.specify/extensions/gomemory-context/**` | `.specify/` presente |
| `gomemory-context/claude/speckit-gomemory-context-update/SKILL.md` | `.claude/skills/speckit-gomemory-context-update/SKILL.md` | `.specify/` presente |
| `gomemory-context/opencode/speckit.gomemory-context.update.md` | `.opencode/commands/speckit.gomemory-context.update.md` | `.specify/` presente |

Los tres comparten la misma condición (FR-001/FR-002/FR-003) y el mismo
criterio de escritura (FR-005/FR-006): se reescribe únicamente si el
contenido destino difiere del embebido.

## Regla de escritura (compartida)

Por archivo individual dentro de los tres subárboles:

1. Si el archivo destino no existe → se crea.
2. Si existe y su contenido es byte-idéntico al embebido → no se toca
   (evita ruido/timestamps falsos en corridas repetidas — FR-006).
3. Si existe y difiere → se sobrescribe con el contenido embebido actual
   (FR-005) — mismo criterio ya verificado en `copyFileOrDir` para el
   plugin de OpenCode.
4. Excepción de permisos: `scripts/bash/update-gomemory-context.sh` recibe
   además `chmod 0755` tras escribirse (ver research.md #4); el resto de
   archivos quedan en `0644`.

## Precondición de instalación

`InstallSpeckitExtension(root string) error`:

- **Entrada**: `root`, la raíz del proyecto destino de `mem install`.
- **Precondición evaluada**: `os.Stat(filepath.Join(root, ".specify"))` es
  un directorio.
  - **Falso** → la función retorna sin crear ni tocar ningún archivo
    (FR-004). No es un error: es el camino esperado para la mayoría de los
    proyectos gomemory hoy.
  - **Verdadero** → se copian los tres subárboles de la tabla anterior con
    la regla de escritura de arriba.
- **Postcondición** (cuando la precondición es verdadera): los tres
  destinos existen y coinciden byte a byte con la versión embebida en el
  binario que ejecutó `mem install`.
