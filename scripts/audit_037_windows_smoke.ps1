# Runner de validación de la feature 037 en Windows: integridad de
# scripts/install.ps1 (H-08) contra un release de fixture, sin red pública.
#
# Uso (PowerShell 5.1 o 7, en Windows):
#   powershell -NoProfile -File scripts/audit_037_windows_smoke.ps1 -MemBin <mem.exe> -WorkDir <dir> -Case all
#
# Códigos de salida (mismos que audit_037_smoke.py):
#   0  todos los casos se ejecutaron y aprobaron
#   1  al menos un caso falló
#   2  preparación inválida, runtime incompatible o cero casos ejecutados
#
# La red se intercepta redefiniendo Invoke-WebRequest/Invoke-RestMethod en
# esta sesión: las funciones tienen precedencia sobre los cmdlets y el
# instalador las hereda al ejecutarse con &. No hay interruptor en
# producción para saltar la verificación. LOCALAPPDATA apunta a un directorio
# temporal y el PATH de usuario, que el instalador modifica tras instalar, se
# restaura al terminar cada caso.

param(
  [Parameter(Mandatory = $true)][string]$MemBin,
  [Parameter(Mandatory = $true)][string]$WorkDir,
  [ValidateSet('all', 'installation')][string]$Case = 'all'
)

$ErrorActionPreference = 'Stop'

if ($PSVersionTable.PSVersion.Major -lt 5) {
  Write-Error "RUNTIME_UNSUPPORTED: PowerShell $($PSVersionTable.PSVersion) < 5.1"
  exit 2
}
if (-not $IsWindows -and $PSVersionTable.PSEdition -eq 'Core') {
  Write-Error "RUNTIME_UNSUPPORTED: este runner valida el instalador de Windows y debe ejecutarse en Windows"
  exit 2
}
if (-not (Test-Path -LiteralPath $MemBin -PathType Leaf)) {
  Write-Error "preparación inválida: -MemBin no existe: $MemBin"
  exit 2
}

Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

$Installer = Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) 'install.ps1'
# Un instalador que no parsea lanza en todos los casos: los negativos, que
# esperan una excepción, aprobarían sin probar nada. Se exige antes de empezar.
$parseErrors = $null
[void][System.Management.Automation.Language.Parser]::ParseFile($Installer, [ref]$null, [ref]$parseErrors)
if ($parseErrors.Count) {
  Write-Error "preparación inválida: install.ps1 no parsea en PowerShell $($PSVersionTable.PSVersion): $($parseErrors[0].Message) (línea $($parseErrors[0].Extent.StartLineNumber))"
  exit 2
}
$Asset = 'mem_windows_amd64.zip'
$RealBytes = [System.IO.File]::ReadAllBytes((Resolve-Path -LiteralPath $MemBin))
$PreviousBytes = [System.Text.Encoding]::ASCII.GetBytes("MZ-binario-anterior")
$Results = New-Object System.Collections.Generic.List[object]

# Estado del release de fixture que leen las funciones de red simuladas.
$script:Fixture = @{}
$script:FailUrl = ''

function Invoke-RestMethod {
  param([string]$Uri, [switch]$UseBasicParsing)
  if ($script:FailUrl -and $Uri -like "*$($script:FailUrl)*") { throw "fallo de red simulado" }
  if (-not $script:Fixture.ContainsKey($Uri)) { throw "404 $Uri" }
  return ([System.Text.Encoding]::UTF8.GetString($script:Fixture[$Uri]) | ConvertFrom-Json)
}

function Invoke-WebRequest {
  param([string]$Uri, [string]$OutFile, [switch]$UseBasicParsing)
  if ($script:FailUrl -and $Uri -like "*$($script:FailUrl)*") { throw "fallo de red simulado" }
  if (-not $script:Fixture.ContainsKey($Uri)) { throw "404 $Uri" }
  [System.IO.File]::WriteAllBytes($OutFile, $script:Fixture[$Uri])
}

function New-Zip([object[]]$Members) {
  $ms = New-Object System.IO.MemoryStream
  $zip = New-Object System.IO.Compression.ZipArchive($ms, [System.IO.Compression.ZipArchiveMode]::Create, $true)
  foreach ($m in $Members) {
    $entry = $zip.CreateEntry($m.Name)
    $s = $entry.Open()
    try { if ($m.Bytes.Length -gt 0) { $s.Write($m.Bytes, 0, $m.Bytes.Length) } } finally { $s.Dispose() }
  }
  $zip.Dispose()
  return $ms.ToArray()
}

function Get-Sha256Hex([byte[]]$Bytes) {
  $sha = [System.Security.Cryptography.SHA256]::Create()
  try { return (($sha.ComputeHash($Bytes) | ForEach-Object { $_.ToString('x2') }) -join '') } finally { $sha.Dispose() }
}

function Invoke-Scenario {
  param(
    [string]$Name,
    [byte[]]$Archive,
    [string]$Checksums = $null,
    [switch]$OmitChecksums,
    [string]$Tag = 'v9.9.9',
    [string]$Version = 'v9.9.9',
    [string]$Latest = $null,
    [string]$FailUrl = '',
    [switch]$LockDestination,
    [switch]$ExpectOk
  )
  $root = Join-Path $WorkDir $Name
  if (Test-Path $root) { Remove-Item $root -Recurse -Force }
  $localAppData = Join-Path $root 'localappdata'
  $installDir = Join-Path $localAppData 'Programs\gomemory'
  New-Item -ItemType Directory -Force -Path $installDir | Out-Null
  $target = Join-Path $installDir 'mem.exe'
  [System.IO.File]::WriteAllBytes($target, $PreviousBytes)

  $base = "https://github.com/Sayoner-000/gomemory/releases/download/$Tag"
  $script:Fixture = @{}
  if ($Archive) { $script:Fixture["$base/$Asset"] = $Archive }
  if (-not $OmitChecksums) {
    if ($null -eq $Checksums) { $Checksums = "$(Get-Sha256Hex $Archive)  $Asset`n" }
    $script:Fixture["$base/checksums.txt"] = [System.Text.Encoding]::UTF8.GetBytes($Checksums)
  }
  if ($Latest) {
    $script:Fixture['https://api.github.com/repos/Sayoner-000/gomemory/releases/latest'] = [System.Text.Encoding]::UTF8.GetBytes((@{ tag_name = $Latest } | ConvertTo-Json))
  }
  $script:FailUrl = $FailUrl

  $savedLocal = $env:LOCALAPPDATA
  $savedUserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
  $lock = $null
  $threw = $false
  $thrown = ''
  try {
    $env:LOCALAPPDATA = $localAppData
    if ($LockDestination) { $lock = [System.IO.File]::Open($target, 'Open', 'Read', 'None') }
    # Fuera de cualquier repositorio git: así el instalador no ofrece la
    # instalación guiada interactiva al terminar.
    Push-Location -LiteralPath $root
    try { & $Installer -Version $Version *> $null } catch { $threw = $true; $thrown = $_.Exception.Message } finally { Pop-Location }
  } finally {
    if ($lock) { $lock.Dispose() }
    $env:LOCALAPPDATA = $savedLocal
    [Environment]::SetEnvironmentVariable('Path', $savedUserPath, 'User')
  }

  $installed = [System.IO.File]::ReadAllBytes($target)
  $leftovers = @(Get-ChildItem -LiteralPath $installDir -Force | Where-Object { $_.Name -ne 'mem.exe' })
  $isNew = [System.Linq.Enumerable]::SequenceEqual($installed, $RealBytes)
  $isPrevious = [System.Linq.Enumerable]::SequenceEqual($installed, $PreviousBytes)
  if ($ExpectOk) {
    $ok = (-not $threw) -and $isNew -and $leftovers.Count -eq 0
  } else {
    $ok = $threw -and $isPrevious -and $leftovers.Count -eq 0
  }
  $detail = "lanzó=$threw nuevo=$isNew previo=$isPrevious residuos=$($leftovers.Count) excepción=$thrown"
  $Results.Add([pscustomobject]@{ case_id = "install.$Name"; ok = $ok; detail = $(if ($ok) { '' } else { $detail }) })
}

New-Item -ItemType Directory -Force -Path $WorkDir | Out-Null
$good = New-Zip @(@{ Name = 'mem.exe'; Bytes = $RealBytes })
$digest = Get-Sha256Hex $good

Invoke-Scenario -Name 'explicit_ok' -Archive $good -ExpectOk
Invoke-Scenario -Name 'latest_resolved_once' -Archive $good -Version 'latest' -Latest 'v9.9.9' -ExpectOk
Invoke-Scenario -Name 'hash_mismatch' -Archive $good -Checksums "$('0' * 64)  $Asset`n"
Invoke-Scenario -Name 'checksums_absent' -Archive $good -OmitChecksums
Invoke-Scenario -Name 'checksums_invalid' -Archive $good -Checksums "zz$($digest.Substring(2))  $Asset`n"
Invoke-Scenario -Name 'entry_missing' -Archive $good -Checksums "$digest  otro.zip`n"
Invoke-Scenario -Name 'entry_substring_only' -Archive $good -Checksums "$digest  $Asset.sig`n"
Invoke-Scenario -Name 'entry_duplicated' -Archive $good -Checksums "$digest  $Asset`n$digest *$Asset`n"
Invoke-Scenario -Name 'network_failure' -Archive $good -FailUrl $Asset
Invoke-Scenario -Name 'empty_binary' -Archive (New-Zip @(@{ Name = 'mem.exe'; Bytes = [byte[]]@() }))
Invoke-Scenario -Name 'nested_member' -Archive (New-Zip @(@{ Name = 'sub/mem.exe'; Bytes = $RealBytes }))
Invoke-Scenario -Name 'traversal_member' -Archive (New-Zip @(@{ Name = '../mem.exe'; Bytes = $RealBytes }))
Invoke-Scenario -Name 'not_pe_binary' -Archive (New-Zip @(@{ Name = 'mem.exe'; Bytes = [System.Text.Encoding]::ASCII.GetBytes('no soy un exe') }))
Invoke-Scenario -Name 'prerelease_tag' -Archive $good -Tag 'v9.9.9-rc1' -Version 'v9.9.9-rc1'
Invoke-Scenario -Name 'destination_locked' -Archive $good -LockDestination

$evidence = Join-Path $WorkDir 'result.json'
$failed = @($Results | Where-Object { -not $_.ok })
$payload = [pscustomobject]@{
  cases_run = $Results.Count
  status = $(if ($Results.Count -eq 0) { 'preparation_failed' } elseif ($failed.Count) { 'failed' } else { 'passed' })
  powershell = "$($PSVersionTable.PSVersion) $($PSVersionTable.PSEdition)"
  os = [System.Environment]::OSVersion.VersionString
  results = $Results
}
$payload | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $evidence -Encoding UTF8

foreach ($r in $Results) {
  $mark = if ($r.ok) { 'ok  ' } else { 'FAIL' }
  Write-Host "$mark $($r.case_id) $($r.detail)"
}
Write-Host "$($Results.Count - $failed.Count)/$($Results.Count) casos aprobados"
if ($Results.Count -eq 0) { exit 2 }
if ($failed.Count) { exit 1 }
exit 0
