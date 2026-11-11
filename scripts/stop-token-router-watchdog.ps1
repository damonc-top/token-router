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
$watchdogScript = Join-Path $workspace 'scripts\token-router-watchdog.ps1'
$pidFile = Join-Path $logDir 'token-router-watchdog.pid'
$stopped = $false

function Test-WatchdogProcess {
    param($Process)

    if ($null -eq $Process -or $Process.Name -notin @('powershell.exe', 'pwsh.exe') -or
        [string]::IsNullOrWhiteSpace($Process.CommandLine)) {
        return $false
    }
    $tokens = @([regex]::Matches($Process.CommandLine, '"(?:\\.|[^"\\])*"|\S+') | ForEach-Object { $_.Value.Trim('"') })
    for ($i = 1; $i -lt $tokens.Count; $i++) {
        $argument = $tokens[$i]
        if ('-Command'.StartsWith($argument, [StringComparison]::OrdinalIgnoreCase) -or
            '-CommandWithArgs'.StartsWith($argument, [StringComparison]::OrdinalIgnoreCase) -or
            '-EncodedCommand'.StartsWith($argument, [StringComparison]::OrdinalIgnoreCase)) {
            return $false
        }
        if ($argument -ieq '-File' -or $argument -ieq '-f') {
            if ($i + 1 -ge $tokens.Count -or -not [IO.Path]::IsPathRooted($tokens[$i + 1])) {
                return $false
            }
            if (-not [string]::Equals([IO.Path]::GetFullPath($tokens[$i + 1]), $watchdogScript, [StringComparison]::OrdinalIgnoreCase)) {
                return $false
            }
            for ($j = $i + 2; $j -lt $tokens.Count; $j++) {
                if ($tokens[$j] -ieq '-Workspace') {
                    return $j + 1 -lt $tokens.Count -and
                        [string]::Equals([IO.Path]::GetFullPath($tokens[$j + 1]), $workspace, [StringComparison]::OrdinalIgnoreCase)
                }
            }
            return $true
        }
    }
    return $false
}

if (Test-Path -LiteralPath $pidFile) {
    $existingPidText = Get-Content -LiteralPath $pidFile -Raw -ErrorAction SilentlyContinue
    $existingPid = 0
    if ([int]::TryParse($existingPidText, [ref]$existingPid)) {
        $existing = Get-CimInstance Win32_Process -Filter "ProcessId=$existingPid" -OperationTimeoutSec 5 -ErrorAction Stop
        if (Test-WatchdogProcess $existing) {
            Stop-Process -Id $existingPid -Force
            $stopped = $true
            Write-Host "Stopped token-router watchdog PID $existingPid."
        }
    }
    Remove-Item -LiteralPath $pidFile -Force -ErrorAction SilentlyContinue
}

Get-CimInstance Win32_Process -Filter "Name='powershell.exe' OR Name='pwsh.exe'" |
    Where-Object {
        $_.ProcessId -ne $PID -and (Test-WatchdogProcess $_)
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
