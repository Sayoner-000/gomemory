# Instalador universal de gomemory para Windows (PowerShell).
#
# Uso:
#   irm https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.ps1 | iex
#
# Desinstalar:
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.ps1))) -Uninstall
#
# Integridad (spec 037, H-08): la versión se resuelve una sola vez a un tag
# estable; el zip y checksums.txt salen de ese mismo release y el SHA-256 se
# verifica ANTES de extraer. Solo se extrae mem.exe, que debe ser un
# ejecutable PE no vacío. Cualquier fallo deja intacto el binario instalado.
# checksums.txt viene del mismo origen: verifica la integridad publicada, no
# es una firma independiente. Los errores usan throw (no exit): con
# `irm | iex` un exit cerraría la consola de quien instala.

param(
  [switch]$Uninstall,
  [string]$Version = $env:GOMEMORY_VERSION
)

$ErrorActionPreference = "Stop"
$Repo = "Sayoner-000/gomemory"
$BinName = "mem.exe"
$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\gomemory"

# Logo derivado de assets/gomemory-terminal.txt; funciona sin el binario.
function Write-Brand($Operation) {
  if ([Console]::IsOutputRedirected -or $env:CI) { return }
  $displayVersion = if ($Version) { $Version } else { 'latest' }
  $width = 80
  try { $width = [Console]::WindowWidth } catch { }
  if ($env:NO_COLOR -or $env:TERM -eq 'dumb' -or $width -lt 78) {
    Write-Host "`ngoMemory · $displayVersion · $Operation`n"
    return
  }
  $logo = @'
             ⣴⣶  ⢀⣀
        ⢰⣷   ⣿⣿  ⣼⣿⠇
     ⣀   ⠿⠇  ⣉⡉  ⠿⠟ ⢀⣼⣷⡄
    ⠘⢿⣦    ⣴⣿⣿⣿⣆    ⠻⡿⠋
      ⠁  ⢀⣾⣿⣿⣿⣿⣿⣾⣆    ⢠⣴⣿⣧
  ⠈⣐⡦  ⢀⢢⣿⣿⣿⣿⣿⣿⣿⣿⣿⣷⣄  ⠈⠟⠋
    ⠁ ⢠⣯⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣆   ⣤⣴⣶
      ⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣆  ⠿⠟⠛
     ⢸⣿⣿⣿⣿⠻⠙⣿⣿⣿⠱⡻⣿⣿⣿⣿⣿⡄ ⣠⣀⣀
     ⢸⣿⣿⣿⣿⣦⣼⠿⣿⣿⣦⣴⣿⣿⣿⣿⣿⡧⠐⠿⣿⣿
   ⠈⠆⢸⣿⣿⣿⣿⣿⣿⣿⣯⣿⣿⣿⣿⣿⣿⣿⣿⡇⣀⡀
     ⠈⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠸⣿⣿⡦
     ⠈⠛⢿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣥  ⠙⠁
        ⠙⠿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠁⠙⠃
'@
  Write-Host ""
  $theme = $env:GOMEMORY_THEME
  if ($theme -notin @('dark', 'light')) {
    $theme = if (($env:COLORFGBG -split ';')[-1] -in @('7', '15')) { 'light' } else { 'dark' }
  }
  $escape = [char]27
  $brandColor = if ($theme -eq 'light') { '57;38;227' } else { '6;193;238' }
  Write-Host "$escape[38;2;${brandColor}m$logo$escape[0m"
  Write-Host "`n  goMemory · $displayVersion  ›  $Operation`n"
}

function Write-Info($m) { if ($env:NO_COLOR) { Write-Host "› $m" } else { Write-Host "› $m" -ForegroundColor Blue } }
function Write-Ok($m)   { if ($env:NO_COLOR) { Write-Host "✓ $m" } else { Write-Host "✓ $m" -ForegroundColor Green } }

function Add-ToUserPath($dir) {
  $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
  if ($userPath -notlike "*$dir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$dir", "User")
    Write-Info "Se agregó $dir al PATH de usuario. Reinicia la terminal para aplicarlo."
  }
}

function Invoke-Uninstall {
  if (Test-Path (Join-Path $InstallDir $BinName)) {
    Remove-Item (Join-Path $InstallDir $BinName) -Force
    Write-Ok "Eliminado $InstallDir\$BinName"
  } else {
    Write-Info "No se encontró el binario instalado."
  }
  Write-Info "Nota: la config y la memoria por-proyecto se quitan con 'mem uninstall <proyecto>'."
  exit 0
}

if ($Uninstall) {
  Write-Brand "Desinstalar"
  Invoke-Uninstall
}
Write-Brand "Instalar"

# Resolve-GomemoryTag convierte la selección en UN tag estable concreto.
# `latest` se consulta una sola vez; el zip y checksums.txt se piden después a
# ese tag exacto.
function Resolve-GomemoryTag([string]$Requested) {
  $tag = $Requested
  if (-not $tag -or $tag -eq 'latest') {
    try {
      $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
    } catch {
      throw "No se pudo consultar la última versión estable de $Repo."
    }
    $tag = [string]$release.tag_name
  }
  if (-not $tag.StartsWith('v')) { $tag = "v$tag" }
  if ($tag -notmatch '^v\d+\.\d+\.\d+$') {
    throw "Versión no soportada: '$tag'. Usa un release estable vX.Y.Z."
  }
  return $tag
}

# Get-ExpectedSha256 exige una única línea cuyo nombre (con asterisco binario
# opcional) coincida EXACTO con el asset y un digest de 64 hex. Una entrada
# duplicada se rechaza aunque ambos digests coincidan.
function Get-ExpectedSha256([string]$SumsPath, [string]$Name) {
  $found = @(Get-Content -LiteralPath $SumsPath | ForEach-Object {
    $parts = @($_.Trim() -split '\s+')
    if ($parts.Count -eq 2 -and $parts[1].TrimStart('*') -ceq $Name) { $parts[0] }
  })
  if ($found.Count -ne 1) {
    throw "checksums.txt no tiene exactamente una entrada para $Name ($($found.Count))."
  }
  if ($found[0] -notmatch '^[0-9a-fA-F]{64}$') {
    throw "checksums.txt tiene un digest inválido para $Name."
  }
  return $found[0].ToLowerInvariant()
}

# Expand-GomemoryBinary extrae solo la entrada raíz mem.exe del zip (sin rutas
# ni traversal) y valida que sea un PE no vacío, sin ejecutarlo.
function Expand-GomemoryBinary([string]$ZipPath, [string]$Destination) {
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  $archive = [System.IO.Compression.ZipFile]::OpenRead($ZipPath)
  try {
    $entries = @($archive.Entries | Where-Object { $_.FullName -ceq $BinName })
    if ($entries.Count -ne 1) { throw "El archivo no contiene exactamente un $BinName en la raíz." }
    if ($entries[0].Length -le 0) { throw "El $BinName del archivo está vacío." }
    $in = $entries[0].Open()
    try {
      $out = [System.IO.File]::Create($Destination)
      try { $in.CopyTo($out) } finally { $out.Dispose() }
    } finally { $in.Dispose() }
  } finally { $archive.Dispose() }
  $header = [byte[]]::new(2)
  $stream = [System.IO.File]::OpenRead($Destination)
  try { $read = $stream.Read($header, 0, 2) } finally { $stream.Dispose() }
  if ($read -ne 2 -or $header[0] -ne 0x4D -or $header[1] -ne 0x5A) {
    throw "El binario descargado no es un ejecutable de Windows."
  }
}

# Publish-GomemoryBinary prepara el nuevo binario junto al destino y lo
# publica con una sola operación de reemplazo. Si el destino está en uso o el
# directorio no admite escritura, falla sin tocar el binario anterior.
function Publish-GomemoryBinary([string]$Source, [string]$Dir) {
  New-Item -ItemType Directory -Force -Path $Dir | Out-Null
  $dest = Join-Path $Dir $BinName
  $staged = Join-Path $Dir (".$BinName." + [System.Guid]::NewGuid().ToString('N') + ".tmp")
  try {
    Copy-Item -LiteralPath $Source -Destination $staged
    if (Test-Path -LiteralPath $dest) {
      [System.IO.File]::Replace($staged, $dest, [NullString]::Value)
    } else {
      [System.IO.File]::Move($staged, $dest)
    }
  } catch {
    Remove-Item -LiteralPath $staged -Force -ErrorAction SilentlyContinue
    throw "No se pudo instalar en ${Dir}: el binario anterior no se modificó."
  }
}

$arch = "amd64"
$asset = "mem_windows_$arch.zip"
$tag = Resolve-GomemoryTag $Version
$base = "https://github.com/$Repo/releases/download/$tag"

Write-Info "Instalando gomemory (windows/$arch, $tag)"
Write-Info "Descargando $base/$asset"

$tmp = New-Item -ItemType Directory -Path (Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid()))
try {
  $zip = Join-Path $tmp $asset
  $sums = Join-Path $tmp "checksums.txt"
  try { Invoke-WebRequest -Uri "$base/$asset" -OutFile $zip -UseBasicParsing }
  catch { throw "No se pudo descargar $asset de $tag." }
  try { Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile $sums -UseBasicParsing }
  catch { throw "No se pudo descargar checksums.txt de ${tag}: no se instala sin verificar." }

  $want = Get-ExpectedSha256 $sums $asset
  $got = (Get-FileHash -Algorithm SHA256 -LiteralPath $zip).Hash.ToLowerInvariant()
  if ($got -ne $want) { throw "El SHA-256 de $asset no coincide con checksums.txt. No se instaló nada." }
  Write-Ok "Integridad verificada (SHA-256)"

  $extracted = Join-Path $tmp $BinName
  Expand-GomemoryBinary $zip $extracted
  Publish-GomemoryBinary $extracted $InstallDir

  Write-Ok "gomemory instalado en $InstallDir\$BinName"
  Add-ToUserPath $InstallDir

  Write-Host ""
  Write-Ok "Listo. Próximos pasos:"
  Write-Host "  mem --help"
  Write-Host "  cd tu-proyecto; mem install ."

  # Feature 034 (FR-026): con sesión interactiva y dentro de un repositorio
  # git, ofrece la instalación guiada en el directorio actual.
  $isRepo = $false
  if (Get-Command git -ErrorAction SilentlyContinue) {
    git rev-parse --show-toplevel 2>$null | Out-Null
    $isRepo = ($LASTEXITCODE -eq 0)
  }
  if ([Environment]::UserInteractive -and -not [Console]::IsInputRedirected -and $isRepo) {
    $answer = Read-Host "¿Configurar gomemory en $((Get-Location).Path) ahora? [S/n]"
    if ($answer -notmatch '^(n|no)$') {
      & (Join-Path $InstallDir $BinName) install .
    } else {
      Write-Info "Puedes hacerlo luego con: mem install ."
    }
  }
}
finally {
  Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
