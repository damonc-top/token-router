[CmdletBinding()]
param(
    [string]$Workspace = '',
    [string]$TunnelConfig = ''
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($Workspace)) {
    throw 'Unable to resolve script directory; pass -Workspace explicitly.'
}

$workspace = (Resolve-Path -LiteralPath $Workspace).Path
$watchdogScript = Join-Path $workspace 'scripts\token-router-watchdog.ps1'
if (-not (Test-Path -LiteralPath $watchdogScript)) {
    throw "Watchdog script not found: $watchdogScript"
}

$arguments = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $watchdogScript)
if (-not [string]::IsNullOrWhiteSpace($TunnelConfig)) {
    $arguments += @('-TunnelConfig', $TunnelConfig)
}

& 'powershell.exe' @arguments
