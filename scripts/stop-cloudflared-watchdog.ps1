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
$logDir = Join-Path $workspace 'logs'
$pidFile = Join-Path $logDir 'token-router-watchdog.pid'
$stopped = $false

if (Test-Path -LiteralPath $pidFile) {
    $existingPidText = (Get-Content -LiteralPath $pidFile -Raw).Trim()
    $existingPid = 0
    if ([int]::TryParse($existingPidText, [ref]$existingPid)) {
        $existing = Get-Process -Id $existingPid -ErrorAction SilentlyContinue
        if ($null -ne $existing) {
            Stop-Process -Id $existingPid -Force
            $stopped = $true
            Write-Host "Stopped token-router watchdog PID $existingPid."
        }
    }
    Remove-Item -LiteralPath $pidFile -Force -ErrorAction SilentlyContinue
}

Get-CimInstance Win32_Process -Filter "Name='powershell.exe' OR Name='pwsh.exe'" |
    Where-Object {
        $_.CommandLine -and
        (
            $_.CommandLine -match 'token-router-watchdog\.ps1' -or
            $_.CommandLine -match 'cloudflared-watchdog\.ps1'
        ) -and
        $_.ProcessId -ne $PID
    } |
    ForEach-Object {
        Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
        $stopped = $true
        Write-Host "Stopped leftover watchdog host PID $($_.ProcessId)."
    }

$legacyPidFile = Join-Path $logDir 'cloudflared-watchdog.pid'
if (Test-Path -LiteralPath $legacyPidFile) {
    Remove-Item -LiteralPath $legacyPidFile -Force -ErrorAction SilentlyContinue
}

if (-not $stopped) {
    Write-Host 'token-router watchdog is not running.'
}
