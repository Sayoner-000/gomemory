#!/usr/bin/env pwsh
# update-gomemory-context.ps1
#
# Incorpora el resumen de historial del proyecto que gomemory ya construye
# (`mem context`) al contexto de la especificación. Nunca falla el flujo de
# spec-kit: siempre termina con código de salida 0, con o sin salida.
#
# Ver contracts/update-gomemory-context-script.md
# (specs/011-gomemory-spec-context) para el contrato completo.
#
# Usage: update-gomemory-context.ps1 (sin argumentos)

[CmdletBinding()]
param()

$ErrorActionPreference = 'Continue'

$ProjectRoot = (Get-Location).Path
$SettingsFile = Join-Path $ProjectRoot '.memory/settings.json'

# 1. Localizar el binario mem: ./mem(.exe) primero, luego mem en PATH.
$MemBin = $null
foreach ($candidate in @('mem.exe', 'mem')) {
    $local = Join-Path $ProjectRoot $candidate
    if (Test-Path -Path $local -PathType Leaf) {
        $MemBin = $local
        break
    }
}
if (-not $MemBin) {
    $onPath = Get-Command 'mem' -ErrorAction SilentlyContinue
    if ($onPath) {
        $MemBin = $onPath.Source
    }
}

if (-not $MemBin) {
    exit 0
}

# 2. Interruptor (feature 011, historia 4): si speckit_context_disabled=true
#    en settings.json, la integración está apagada. Lectura directa del JSON
#    (sin invocar `mem settings`), para no depender de la CLI/TUI.
if (Test-Path -Path $SettingsFile -PathType Leaf) {
    try {
        $raw = Get-Content -Path $SettingsFile -Raw -ErrorAction Stop
        if ($raw -match '"speckit_context_disabled"\s*:\s*true') {
            exit 0
        }
    } catch {
        # settings.json ilegible: tratar como "no desactivado" y seguir,
        # mismo criterio de degradación transparente que el resto del script.
    }
}

# 3. Obtener el resumen. Si falla, salir en silencio (degradación transparente).
try {
    $output = & $MemBin context 2>$null
    if ($LASTEXITCODE -ne 0) {
        exit 0
    }
} catch {
    exit 0
}

# 4. Emitir el resumen tal cual: ya viene acotado por el presupuesto de
#    caracteres configurado en gomemory (Budget), sin recorte adicional aquí.
if ($output) {
    $output | Out-String | Write-Output
}
exit 0
