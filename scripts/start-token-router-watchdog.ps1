[CmdletBinding()]
param(
    [string]$Workspace = '',
    [string]$TunnelConfig = ''
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
$watchdogScript = Join-Path $PSScriptRoot 'token-router-watchdog.ps1'
$pidFile = Join-Path $logDir 'token-router-watchdog.pid'
$stdoutLog = Join-Path $logDir 'token-router-watchdog-stdout.log'
$stderrLog = Join-Path $logDir 'token-router-watchdog-stderr.log'

if (-not (Test-Path -LiteralPath $watchdogScript)) {
    throw "Watchdog script not found: $watchdogScript"
}

$null = New-Item -ItemType Directory -Path $logDir -Force

if (Test-Path -LiteralPath $pidFile) {
    $existingPidText = (Get-Content -LiteralPath $pidFile -Raw).Trim()
    $existingPid = 0
    if ([int]::TryParse($existingPidText, [ref]$existingPid)) {
        $existing = Get-Process -Id $existingPid -ErrorAction SilentlyContinue
        if ($null -ne $existing) {
            Write-Host "token-router watchdog already running (PID $existingPid)."
            exit 0
        }
    }
}

$legacyPidFile = Join-Path $logDir 'cloudflared-watchdog.pid'
if (Test-Path -LiteralPath $legacyPidFile) {
    $legacyPidText = (Get-Content -LiteralPath $legacyPidFile -Raw).Trim()
    $legacyPid = 0
    if ([int]::TryParse($legacyPidText, [ref]$legacyPid)) {
        Stop-Process -Id $legacyPid -Force -ErrorAction SilentlyContinue
    }
    Remove-Item -LiteralPath $legacyPidFile -Force -ErrorAction SilentlyContinue
}
Get-CimInstance Win32_Process -Filter "Name='powershell.exe' OR Name='pwsh.exe'" |
    Where-Object {
        $_.CommandLine -and
        $_.CommandLine -match 'cloudflared-watchdog\.ps1' -and
        $_.ProcessId -ne $PID
    } |
    ForEach-Object {
        Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
    }

$argumentList = @(
    '-NoProfile',
    '-ExecutionPolicy', 'Bypass',
    '-File', $watchdogScript,
    '-Workspace', $workspace
)
if (-not [string]::IsNullOrWhiteSpace($TunnelConfig)) {
    $argumentList += @('-TunnelConfig', $TunnelConfig)
}

$process = Start-Process -FilePath 'powershell.exe' -ArgumentList $argumentList -WorkingDirectory $workspace -RedirectStandardOutput $stdoutLog -RedirectStandardError $stderrLog -WindowStyle Hidden -PassThru

Write-Host "token-router watchdog started. PID=$($process.Id)"
Write-Host "Log: $logDir\token-router-watchdog.log"
