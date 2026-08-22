[CmdletBinding()]
param(
    [string]$Workspace = '',
    [string]$TunnelConfig = '',
    [string]$TaskName = 'TokenRouterWatchdog',
    [switch]$LeaveLegacyTasksEnabled
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Test-IsAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

if (-not (Test-IsAdministrator)) {
    throw 'install-token-router-watchdog-task.ps1 must run elevated (Administrator).'
}

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
$watchdogScript = Join-Path $workspace 'scripts\token-router-watchdog.ps1'
if (-not (Test-Path -LiteralPath $watchdogScript)) {
    throw "Watchdog script not found: $watchdogScript"
}

if ([string]::IsNullOrWhiteSpace($TunnelConfig)) {
    if (-not [string]::IsNullOrWhiteSpace($env:CLOUDFLARED_CONFIG) -and (Test-Path -LiteralPath $env:CLOUDFLARED_CONFIG)) {
        $TunnelConfig = (Resolve-Path -LiteralPath $env:CLOUDFLARED_CONFIG).Path
    } elseif (Test-Path -LiteralPath (Join-Path $env:USERPROFILE '.cloudflared\config.yml')) {
        $TunnelConfig = (Resolve-Path -LiteralPath (Join-Path $env:USERPROFILE '.cloudflared\config.yml')).Path
    } elseif (Test-Path -LiteralPath 'C:\Users\afeng\.cloudflared\config.yml') {
        $TunnelConfig = 'C:\Users\afeng\.cloudflared\config.yml'
    } else {
        throw 'Tunnel config not found. Pass -TunnelConfig explicitly.'
    }
} else {
    $TunnelConfig = (Resolve-Path -LiteralPath $TunnelConfig).Path
}

# Long-running task process (not a detaching launcher), so RestartCount works.
$arguments = "-NoProfile -ExecutionPolicy Bypass -File `"$watchdogScript`" -Workspace `"$workspace`" -TunnelConfig `"$TunnelConfig`""
$action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument $arguments -WorkingDirectory $workspace
$trigger = New-ScheduledTaskTrigger -AtStartup
$trigger.Delay = 'PT15S'
$principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew -WakeToRun -DisallowDemandStart:$false
$settings.DisallowStartIfOnBatteries = $false
$settings.StopIfGoingOnBatteries = $false
$settings.IdleSettings.StopOnIdleEnd = $false
$settings.IdleSettings.RestartOnIdle = $false

Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
Write-Host "Scheduled task installed: $TaskName (long-running watchdog)"

if (-not $LeaveLegacyTasksEnabled) {
    foreach ($legacyName in @('TokenRouterService', 'TokenRouterTunnelService')) {
        $legacy = Get-ScheduledTask -TaskName $legacyName -ErrorAction SilentlyContinue
        if ($null -ne $legacy) {
            Disable-ScheduledTask -TaskName $legacyName | Out-Null
            Write-Host "Disabled legacy task: $legacyName"
        }
    }
}

& (Join-Path $workspace 'scripts\stop-token-router-watchdog.ps1') -Workspace $workspace
Start-Sleep -Seconds 1
Start-ScheduledTask -TaskName $TaskName
Start-Sleep -Seconds 3
$info = Get-ScheduledTaskInfo -TaskName $TaskName
$task = Get-ScheduledTask -TaskName $TaskName
Write-Host "Task state=$($task.State) lastResult=$($info.LastTaskResult) lastRun=$($info.LastRunTime)"
Write-Host "Watchdog log: $workspace\logs\token-router-watchdog.log"
