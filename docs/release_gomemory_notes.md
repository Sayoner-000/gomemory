# Release de gomemory — paso a paso manual

> Instrucciones para compilar, empaquetar y publicar un release en GitHub
> desde cero. Para cuando no puedes (o no quieres) usar GoReleaser.

## Vista rápida

```bash
# 1. Comprueba que estás en master con todo commiteado
git checkout master && git status

# 2. Compila para todas las plataformas (CGO_ENABLED=0)
./scripts/release-build.sh      # crea /tmp/gomemory-release/

# 3. Pushea el tag a GitHub
git push origin master
git push origin vX.Y.Z

# 4. Crea el Release en GitHub web, sube los 6 assets
#    (los archivos están en /tmp/gomemory-release/)
```

---

## Requisitos

- **Go 1.25+** — `go version`
- **Git** — `git --version`
- Acceso de escritura al repo `Sayoner-000/gomemory` en GitHub
- `zip` — `which zip` (para el asset de Windows)

---

## 1. Prepara el tag

Asegúrate de que todo esté commiteado en `master`:

```bash
git checkout master
git status                # debe decir "nothing to commit, working tree clean"
```

Crea el tag semántico:

```bash
git tag v1.8.0            # reemplaza con la versión que toque
git push origin master
git push origin v1.8.0
```

> El tag debe existir en GitHub antes de crear el Release,
> porque el Release se asocia a un tag existente.

---

## 2. Compila los binarios cross-platform

El driver SQLite es `modernc.org/sqlite` (Go puro, sin CGO),
así que se puede compilar a cualquier plataforma sin toolchain de C.

```bash
# Limpiar y verificar
go mod tidy
go vet ./...

# Compilar para cada plataforma
mkdir -p /tmp/gomemory-release

GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/gomemory-release/mem_linux_amd64    ./infrastructure/
GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/gomemory-release/mem_linux_arm64    ./infrastructure/
GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/gomemory-release/mem_darwin_amd64   ./infrastructure/
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/gomemory-release/mem_darwin_arm64   ./infrastructure/
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/gomemory-release/mem_windows_amd64.exe ./infrastructure/
```

---

## 3. Empaqueta los assets

Cada archivo comprimido debe contener solo el binario (con nombre `mem`
o `mem.exe`) más la documentación estándar.

```bash
cd /tmp/gomemory-release

# Documentos que van dentro de cada paquete
DOCS="README.md INSTALLATION.md"
[ -f "/home/admindocker/data/go_memory/LICENSE" ] && DOCS="$DOCS LICENSE"

# Preparar carpeta temporal con docs
mkdir -p dist
for f in $DOCS; do cp "/home/admindocker/data/go_memory/$f" dist/; done

# linux/amd64
cp mem_linux_amd64 dist/mem
tar czf mem_linux_amd64.tar.gz -C dist mem $DOCS
rm dist/mem

# linux/arm64
cp mem_linux_arm64 dist/mem
tar czf mem_linux_arm64.tar.gz -C dist mem $DOCS
rm dist/mem

# darwin/amd64
cp mem_darwin_amd64 dist/mem
tar czf mem_darwin_amd64.tar.gz -C dist mem $DOCS
rm dist/mem

# darwin/arm64
cp mem_darwin_arm64 dist/mem
tar czf mem_darwin_arm64.tar.gz -C dist mem $DOCS
rm dist/mem

# windows/amd64
cp mem_windows_amd64.exe dist/mem.exe
cd dist && zip -q /tmp/gomemory-release/mem_windows_amd64.zip mem.exe $DOCS
cd /tmp/gomemory-release
rm dist/mem.exe
```

---

## 4. Genera checksums.txt

```bash
cd /tmp/gomemory-release
sha256sum mem_*.tar.gz mem_*.zip > checksums.txt
cat checksums.txt
```

---

## 5. Assets generados

| Archivo | Descripción |
|---|---|
| `mem_linux_amd64.tar.gz` | Linux x86_64 |
| `mem_linux_arm64.tar.gz` | Linux ARM64 |
| `mem_darwin_amd64.tar.gz` | macOS Intel |
| `mem_darwin_arm64.tar.gz` | macOS Apple Silicon |
| `mem_windows_amd64.zip` | Windows x86_64 |
| `checksums.txt` | SHA256 de todos los assets |

---

## 6. Crea el Release en GitHub (web)

1. Ve a https://github.com/Sayoner-000/gomemory/releases
2. Click **"Create a new release"** (o **"Draft a new release"**)
3. **Tag:** `v1.8.0` (el que pusheaste antes)
4. **Release title:** `v1.8.0`
5. **Description:** escribe las notas del release
   (puedes usar el formato tradicional: What's New, Fixes, Breaking Changes)
6. **Attach binaries:** arrastra los 6 archivos de `/tmp/gomemory-release/`
7. **Publish release**

---

## 7. Verifica la instalación

Desde cualquier máquina:

```bash
curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.sh | bash
```

Debe descargar y extraer el asset correcto para tu SO/arquitectura
y dejar el binario `mem` en el PATH.

---

## Notas

- **Los assets en `/tmp/gomemory-release/` se pierden al reiniciar.**
  Si no publicas el release de inmediato, copia los archivos a un lugar seguro.
- El instalador `install.sh` busca `mem_${os}_${arch}.tar.gz`.
  Si cambias el naming, actualiza también el script.
- Si usas GoReleaser (`.goreleaser.yml`), todo esto se automatiza con
  `goreleaser release --clean`. Pero requiere el token de GitHub configurado.
