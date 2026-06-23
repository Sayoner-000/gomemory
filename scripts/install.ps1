# Instalador universal de gomemory para Windows (PowerShell).
#
# Uso:
#   irm https://raw.githubusercontent.com/Sayoner-000/gomemory/main/scripts/install.ps1 | iex
#
# Desinstalar:
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/Sayoner-000/gomemory/main/scripts/install.ps1))) -Uninstall

param(
  [switch]$Uninstall,
  [string]$Version = $env:GOMEMORY_VERSION
)

$ErrorActionPreference = "Stop"
$Repo = "Sayoner-000/gomemory"
$BinName = "mem.exe"
$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\gomemory"

function Write-Info($m) { Write-Host "› $m" -ForegroundColor Blue }
function Write-Ok($m)   { Write-Host "✓ $m" -ForegroundColor Green }

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

if ($Uninstall) { Invoke-Uninstall }

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
}
finally {
  Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
