# Publicar una versión de gomemory

Los releases se publican automáticamente con GoReleaser. El workflow
`.github/workflows/release.yml` se ejecuta al enviar un tag `v*`; no es
necesario compilar, comprimir ni subir archivos a GitHub de forma manual.

## Requisitos

- Go 1.27 o posterior para validar el código localmente.
- Permiso para enviar commits y tags al repositorio.
- Una rama `main` limpia y actualizada.
- `gh` es opcional para consultar el workflow y el release desde la terminal.

## 1. Preparar el cambio

Actualiza `CHANGELOG.md`, confirma la versión que se publicará y ejecuta las
validaciones del repositorio:

```bash
git status --short
go test ./...
go vet ./...
git diff --check
```

El release debe apuntar a un commit ya integrado en `main`.

### Mantener las guías HTML vigentes

El repositorio publica una sola edición de cada guía: `Guia-SDD_HCD_GOMEMORY-vX.Y.html`
y `Guia-Tecnica_GOMEMORY-vX.Y.html` en `docs/`. Para cada release, actualiza ambas
guías desde la edición vigente, revisa los capítulos y recuentos afectados por el
cambio, y elimina las guías de versión anterior. Actualiza sus enlaces en
`README.md` y `docs/ARQUITECTURA.md`. No mantengas un archivo histórico de ediciones.

## 2. Crear y enviar el tag

Usa una versión semántica con prefijo `v`:

```bash
version=vX.Y.Z
git tag -a "$version" -m "Release $version"
git push origin main
git push origin "$version"
```

El tag activa el workflow de release. GoReleaser toma la versión del propio tag
y la inyecta en el binario mediante `-ldflags`.

## 3. Artefactos publicados

La configuración `.goreleaser.yml` genera binarios para:

| Sistema | Arquitectura | Formato |
|---|---|---|
| Linux | amd64, arm64 | `tar.gz` |
| macOS | amd64, arm64 | `tar.gz` |
| Windows | amd64 | `zip` |

Cada archivo incluye el binario, `README.md`, `LICENSE` e `INSTALLATION.md`.
El release también publica `checksums.txt` con las sumas SHA-256.

## 4. Verificar el release

Comprueba que el workflow terminó correctamente y que GitHub muestra los cinco
paquetes y el archivo de checksums:

```bash
gh run list --workflow release.yml --limit 1
gh release view "$version"
```

Después valida el instalador fijando la versión recién publicada:

```bash
curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/main/scripts/install.sh | \
  GOMEMORY_VERSION="$version" bash
mem --version
```

En PowerShell:

```powershell
$base = "https://github.com/Sayoner-000/gomemory/releases/download/vX.Y.Z"
Invoke-WebRequest "$base/mem_windows_amd64.zip" -OutFile "$env:TEMP\mem.zip"
Expand-Archive "$env:TEMP\mem.zip" -DestinationPath "$env:TEMP\mem-vX.Y.Z" -Force
& "$env:TEMP\mem-vX.Y.Z\mem.exe" --version
```

El job de Windows de `test.yml` ejecuta esta misma descarga contra el último
release y comprueba además el SHA-256.

Si el workflow falla, corrige la causa y publica un tag nuevo. No reemplaces un
tag ya distribuido ni subas artefactos construidos con pasos distintos a los de
`.goreleaser.yml`.
