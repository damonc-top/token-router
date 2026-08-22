[CmdletBinding()]
param(
    [string]$Workspace = ''
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($Workspace)) {
    $scriptDir = $PSScriptRoot
    if ([string]::IsNullOrWhiteSpace($scriptDir) -and $MyInvocation.MyCommand.Path) {
        $scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
    }
    if ([string]::IsNullOrWhiteSpace($scriptDir)) {
        throw 'Unable to resolve script directory; pass -Workspace explicitly.'
    }
    $Workspace = Split-Path -Parent $scriptDir
}

$workspace = (Resolve-Path -LiteralPath $Workspace).Path
$launcher = Join-Path $workspace 'scripts\start-token-router-watchdog.ps1'
if (-not (Test-Path -LiteralPath $launcher)) {
    throw "Watchdog launcher not found: $launcher"
}

$startupDir = [Environment]::GetFolderPath('Startup')
if ([string]::IsNullOrWhiteSpace($startupDir) -or -not (Test-Path -LiteralPath $startupDir)) {
    throw "Startup folder not found: $startupDir"
}

$cmdPath = Join-Path $startupDir 'TokenRouterWatchdog.cmd'
$cmd = @(
    '@echo off',
    "powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$launcher`" -Workspace `"$workspace`""
) -join "`r`n"
Set-Content -LiteralPath $cmdPath -Value $cmd -Encoding ascii
Write-Host "Startup launcher installed: $cmdPath"
