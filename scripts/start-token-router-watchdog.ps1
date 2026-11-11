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

function Get-RunningWatchdog {
    if (-not (Test-Path -LiteralPath $pidFile)) {
        return $null
    }
    $existingPid = 0
    $pidText = Get-Content -LiteralPath $pidFile -Raw -ErrorAction SilentlyContinue
    if (-not [int]::TryParse($pidText, [ref]$existingPid)) {
        return $null
    }
    if ($null -eq (Get-Process -Id $existingPid -ErrorAction SilentlyContinue)) {
        return $null
    }
    $candidate = Get-CimInstance Win32_Process -Filter "ProcessId=$existingPid" -OperationTimeoutSec 5 -ErrorAction Stop
    if (Test-WatchdogProcess $candidate) {
        return $candidate
    }
    return $null
}

if (-not (Test-Path -LiteralPath $watchdogScript)) {
    throw "Watchdog script not found: $watchdogScript"
}

$null = New-Item -ItemType Directory -Path $logDir -Force

$existing = Get-RunningWatchdog
if ($null -ne $existing) {
    Write-Host "token-router watchdog already running (PID $($existing.ProcessId))."
    exit 0
}

if (Test-Path -LiteralPath $pidFile) {
    Remove-Item -LiteralPath $pidFile -Force
}
$existing = Get-CimInstance Win32_Process -Filter "Name='powershell.exe' OR Name='pwsh.exe'" |
    Where-Object { Test-WatchdogProcess $_ } |
    Select-Object -First 1
if ($null -ne $existing) {
    Set-Content -LiteralPath $pidFile -Value $existing.ProcessId -Encoding ascii
    Write-Host "token-router watchdog already running (PID $($existing.ProcessId))."
    exit 0
}

$task = $null
if (Get-Command Get-ScheduledTask -ErrorAction SilentlyContinue) {
    $installedTask = Get-ScheduledTask -TaskName 'TokenRouterWatchdog' -ErrorAction SilentlyContinue
    if ($null -ne $installedTask -and $installedTask.State -ne 'Disabled' -and @($installedTask.Actions).Count -eq 1) {
        $action = @($installedTask.Actions)[0]
        if (Test-WatchdogProcess ([pscustomobject]@{
            Name = [IO.Path]::GetFileName($action.Execute)
            CommandLine = '"' + $action.Execute + '" ' + $action.Arguments
        })) {
            $task = $installedTask
        }
    }
}

$startedAt = Get-Date
$process = $null
if ($null -ne $task) {
    Start-ScheduledTask -InputObject $task
} else {
    $powerShellPath = if ($PSVersionTable.PSVersion.Major -ge 7) {
        Join-Path $PSHOME 'pwsh.exe'
    } else {
        (Get-Command pwsh.exe, powershell.exe -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1).Source
    }
    $arguments = '-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "{0}" -Workspace "{1}" -HealthyIntervalSeconds 10 -SkipSleepManagement' -f $watchdogScript, $workspace
    if (-not [string]::IsNullOrWhiteSpace($TunnelConfig)) {
        $configPath = (Resolve-Path -LiteralPath $TunnelConfig).Path
        $arguments += ' -TunnelConfig "{0}"' -f $configPath
    }
    $process = Start-Process -FilePath $powerShellPath -ArgumentList $arguments -WorkingDirectory $workspace -RedirectStandardOutput $stdoutLog -RedirectStandardError $stderrLog -WindowStyle Hidden -PassThru
}

$deadline = (Get-Date).AddSeconds(60)
while ((Get-Date) -lt $deadline) {
    $running = Get-RunningWatchdog
    if ($null -ne $running) {
        Start-Sleep -Milliseconds 500
        $confirmed = Get-RunningWatchdog
        if ($null -ne $confirmed -and $confirmed.ProcessId -eq $running.ProcessId) {
            Write-Host "token-router watchdog started. PID=$($confirmed.ProcessId)"
            Write-Host "Log: $logDir\token-router-watchdog.log"
            exit 0
        }
    }
    if ($null -ne $process -and $process.HasExited) {
        break
    }
    Start-Sleep -Milliseconds 250
}
$details = @()
foreach ($log in @($stderrLog, (Join-Path $logDir 'token-router-watchdog.log'))) {
    if ((Test-Path -LiteralPath $log) -and (Get-Item -LiteralPath $log).LastWriteTime -ge $startedAt) {
        $details += Get-Content -LiteralPath $log -Tail 10
    }
}
if ($null -ne $task) {
    $info = Get-ScheduledTaskInfo -InputObject $task
    $details += "Scheduled task result: $($info.LastTaskResult)"
}
throw "token-router watchdog did not become ready. $($details -join [Environment]::NewLine)"
