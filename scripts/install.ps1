# Instalador universal de gomemory para Windows (PowerShell).
#
# Uso:
#   irm https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.ps1 | iex
#
# Desinstalar:
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.ps1))) -Uninstall

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
    ⠘⢿⣦    ⣴⣿⣿⣿⣆    ⠻⡿⠋          ███  ██  █   █ ████ █   █  ██  ███  █  █
      ⠁  ⢀⣾⣿⣿⣿⣿⣿⣾⣆    ⢠⣴⣿⣧      █    █  █ ██ ██ █    ██ ██ █  █ █  █ █  █
  ⠈⣐⡦  ⢀⢢⣿⣿⣿⣿⣿⣿⣿⣿⣿⣷⣄  ⠈⠟⠋       █ ██ █  █ █ █ █ ███  █ █ █ █  █ ███   ██
    ⠁ ⢠⣯⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣆   ⣤⣴⣶     █  █ █  █ █   █ █    █   █ █  █ █ █   █
      ⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣆  ⠿⠟⠛      ███  ██  █   █ ████ █   █  ██  █  █  █
     ⢸⣿⣿⣿⣿⠻⠙⣿⣿⣿⠱⡻⣿⣿⣿⣿⣿⡄ ⣠⣀⣀
     ⢸⣿⣿⣿⣿⣦⣼⠿⣿⣿⣦⣴⣿⣿⣿⣿⣿⡧⠐⠿⣿⣿
   ⠈⠆⢸⣿⣿⣿⣿⣿⣿⣿⣯⣿⣿⣿⣿⣿⣿⣿⣿⡇⣀⡀       Memoria persistente
     ⠈⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠸⣿⣿⡦      para agentes de código
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
  Write-Host "`n  $displayVersion  ›  $Operation`n"
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

if (-not $Version) { $Version = "latest" }

$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "amd64" }
$asset = "mem_windows_$arch.zip"

if ($Version -eq "latest") {
  $url = "https://github.com/$Repo/releases/latest/download/$asset"
} else {
  $url = "https://github.com/$Repo/releases/download/$Version/$asset"
}

Write-Info "Instalando gomemory (windows/$arch, $Version)"
Write-Info "Descargando $url"

$tmp = New-Item -ItemType Directory -Path (Join-Path $env:TEMP ([System.Guid]::NewGuid()))
try {
  $zip = Join-Path $tmp $asset
  Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing
  Expand-Archive -Path $zip -DestinationPath $tmp -Force

  New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
  Copy-Item (Join-Path $tmp $BinName) (Join-Path $InstallDir $BinName) -Force

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
