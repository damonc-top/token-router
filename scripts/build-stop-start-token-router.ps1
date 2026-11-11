[CmdletBinding()]
param(
    [string]$Workspace = '',
    [switch]$UseEnvironmentProxy
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# Resolve the workspace inside the body: Windows PowerShell 5.1 leaves
# $PSScriptRoot empty while evaluating parameter defaults under -File, so a
# param default cannot see it. $MyInvocation covers the remaining invocations
# that leave it empty, such as piping the script into the interpreter.
if ([string]::IsNullOrWhiteSpace($Workspace)) {
    $scriptDir = $PSScriptRoot
    if ([string]::IsNullOrWhiteSpace($scriptDir)) {
        $command = $MyInvocation.MyCommand
        if ($null -ne $command -and $command.CommandType -eq 'ExternalScript') {
            $scriptDir = Split-Path -Parent $command.Path
        }
    }
    if ([string]::IsNullOrWhiteSpace($scriptDir)) {
        throw 'Unable to resolve the script directory; pass -Workspace explicitly.'
    }
    $Workspace = Split-Path -Parent $scriptDir
}

function Invoke-ExternalCommand {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [string[]]$Arguments = @(),
        [Parameter(Mandatory = $true)][string]$Description,
        [string]$WorkingDirectory
    )

    if ($WorkingDirectory) {
        Push-Location -LiteralPath $WorkingDirectory
    }
    try {
        & $FilePath @Arguments
        if ($LASTEXITCODE -ne 0) {
            throw "$Description failed with exit code $LASTEXITCODE."
        }
    } finally {
        if ($WorkingDirectory) {
            Pop-Location
        }
    }
}

function Get-ManagedProcesses {
    param([Parameter(Mandatory = $true)][string]$FilePath)

    $resolvedPath = [System.IO.Path]::GetFullPath($FilePath)
    $name = [System.IO.Path]::GetFileName($resolvedPath).Replace("'", "''")
    return @(
        Get-CimInstance -ClassName Win32_Process -Filter "Name='$name'" |
            Where-Object {
                if (-not [string]::IsNullOrWhiteSpace($_.ExecutablePath)) {
                    return [string]::Equals(
                        [System.IO.Path]::GetFullPath($_.ExecutablePath),
                        $resolvedPath,
                        [System.StringComparison]::OrdinalIgnoreCase
                    )
                }
                if (-not [string]::IsNullOrWhiteSpace($_.CommandLine)) {
                    return $_.CommandLine.IndexOf($resolvedPath, [System.StringComparison]::OrdinalIgnoreCase) -ge 0
                }
                return $true
            }
    )
}

function Stop-ManagedProcess {
    param([Parameter(Mandatory = $true)][string]$FilePath)

    $processes = @(Get-ManagedProcesses -FilePath $FilePath)
    foreach ($process in $processes) {
        $runningProcess = Get-Process -Id ([int]$process.ProcessId) -ErrorAction SilentlyContinue
        if ($null -ne $runningProcess) {
            $runningProcess.Kill($true)
        }
    }

    $deadline = (Get-Date).AddSeconds(30)
    $remainingProcesses = @(Get-ManagedProcesses -FilePath $FilePath)
    while ($remainingProcesses.Count -gt 0 -and (Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 250
        $remainingProcesses = @(Get-ManagedProcesses -FilePath $FilePath)
    }
    if ($remainingProcesses.Count -gt 0) {
        throw "Timed out stopping $FilePath."
    }
}

function Start-ManagedProcess {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [string[]]$Arguments = @(),
        [Parameter(Mandatory = $true)][string]$WorkingDirectory,
        [Parameter(Mandatory = $true)][string]$StandardOutputPath,
        [Parameter(Mandatory = $true)][string]$StandardErrorPath,
        [hashtable]$Environment = @{}
    )

    $parameters = @{
        FilePath               = $FilePath
        ArgumentList           = $Arguments
        WorkingDirectory       = $WorkingDirectory
        RedirectStandardOutput = $StandardOutputPath
        RedirectStandardError  = $StandardErrorPath
        WindowStyle            = 'Hidden'
        PassThru               = $true
    }
    if ($Environment.Count -gt 0) {
        $parameters.Environment = $Environment
    }
    return Start-Process @parameters
}

function Wait-ForHttpStatus {
    param(
        [Parameter(Mandatory = $true)][string]$Uri,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $lastError = "no response"
    while ((Get-Date) -lt $deadline) {
        try {
            $response = Invoke-WebRequest -Uri $Uri -NoProxy -UseBasicParsing -TimeoutSec 3
            if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 300) {
                return
            }
            $lastError = "received HTTP $($response.StatusCode)"
        } catch {
            $lastError = $_.Exception.Message
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for ${Uri}: $lastError"
}

function Wait-ForTunnelConnection {
    param(
        [Parameter(Mandatory = $true)][int[]]$MetricsPorts,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds
    )

    # cloudflared picks the first free metrics port from its configured
    # address; a socket still lingering from the stopped instance bumps the
    # bind to the next port, so probe a range instead of one fixed port.
    $uris = @($MetricsPorts | ForEach-Object { "http://127.0.0.1:$_/metrics" })
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $lastError = "no metrics response"
    while ((Get-Date) -lt $deadline) {
        foreach ($uri in $uris) {
            try {
                $response = Invoke-WebRequest -Uri $uri -NoProxy -UseBasicParsing -TimeoutSec 3
                $connectionMatches = [regex]::Matches(
                    $response.Content,
                    '(?m)^cloudflared_tunnel_ha_connections(?:\{[^}]*\})?\s+([0-9]+(?:\.[0-9]+)?)'
                )
                foreach ($connectionMatch in $connectionMatches) {
                    if ([double]$connectionMatch.Groups[1].Value -gt 0) {
                        return
                    }
                }
                $lastError = "cloudflared_tunnel_ha_connections is zero at $uri"
            } catch {
                $lastError = $_.Exception.Message
            }
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for an active Cloudflare Tunnel connection; tried metrics at $($uris -join ', '): $lastError"
}

function Resolve-BunPath {
    $command = Get-Command bun -CommandType Application -ErrorAction SilentlyContinue
    if ($null -ne $command) {
        return $command.Source
    }

    $candidates = @(
        (Join-Path $env:USERPROFILE '.bun\bin\bun.exe'),
        (Join-Path $env:LOCALAPPDATA 'bun\bin\bun.exe')
    )
    foreach ($candidate in $candidates) {
        if (Test-Path -LiteralPath $candidate) {
            return (Resolve-Path -LiteralPath $candidate).Path
        }
    }
    throw "Bun was not found. Install Bun or add it to PATH."
}

$workspace = (Resolve-Path -LiteralPath $Workspace).Path
$webDir = Join-Path $workspace 'web'
$logDir = Join-Path $workspace 'logs'
$backupDir = Join-Path $logDir 'deploy-backups'
$apiPath = Join-Path $workspace 'token-router.exe'
$cloudflaredPath = Join-Path $workspace 'bin\cloudflared.exe'
$watchdogStart = Join-Path $workspace 'scripts\start-token-router-watchdog.ps1'
$watchdogStop = Join-Path $workspace 'scripts\stop-token-router-watchdog.ps1'
$watchdogScript = Join-Path $workspace 'scripts\token-router-watchdog.ps1'
$powerShellPath = (Get-Process -Id $PID).Path
$tunnelConfig = Join-Path $env:USERPROFILE '.cloudflared\config.yml'
$metricsPorts = 20241..20245
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$stagedApiPath = Join-Path $workspace "token-router-$stamp.next.exe"
$backupApiPath = Join-Path $backupDir "token-router-$stamp.exe"
$failedApiPath = Join-Path $backupDir "token-router-$stamp.failed.exe"

foreach ($path in @($webDir, $cloudflaredPath, $tunnelConfig, $watchdogStart, $watchdogStop, $watchdogScript)) {
    if (-not (Test-Path -LiteralPath $path)) {
        throw "Required path does not exist: $path"
    }
}

$goPath = (Get-Command go -CommandType Application -ErrorAction Stop).Source
$bunPath = Resolve-BunPath
$null = New-Item -ItemType Directory -Path $logDir, $backupDir -Force

$hadVersion = Test-Path Env:VITE_REACT_APP_VERSION
$previousVersion = $env:VITE_REACT_APP_VERSION
$hadDisableEslint = Test-Path Env:DISABLE_ESLINT_PLUGIN
$previousDisableEslint = $env:DISABLE_ESLINT_PLUGIN
$versionPath = Join-Path $workspace 'VERSION'
$version = 'v0.0.0'
if (Test-Path -LiteralPath $versionPath) {
    $versionContents = Get-Content -LiteralPath $versionPath -Raw
    if (-not [string]::IsNullOrWhiteSpace($versionContents)) {
        $version = $versionContents.Trim()
    }
}
$env:VITE_REACT_APP_VERSION = $version
$env:DISABLE_ESLINT_PLUGIN = 'true'
try {
    Invoke-ExternalCommand -FilePath $bunPath -Arguments @('install', '--frozen-lockfile') -Description 'Frontend dependency validation' -WorkingDirectory $webDir
    Invoke-ExternalCommand -FilePath $bunPath -Arguments @('run', 'build') -Description 'Frontend build' -WorkingDirectory $webDir
} finally {
    if ($hadVersion) {
        $env:VITE_REACT_APP_VERSION = $previousVersion
    } else {
        Remove-Item Env:VITE_REACT_APP_VERSION -ErrorAction SilentlyContinue
    }
    if ($hadDisableEslint) {
        $env:DISABLE_ESLINT_PLUGIN = $previousDisableEslint
    } else {
        Remove-Item Env:DISABLE_ESLINT_PLUGIN -ErrorAction SilentlyContinue
    }
}

Invoke-ExternalCommand -FilePath $goPath -Arguments @(
    'build',
    '-ldflags', "-X github.com/QuantumNous/new-api/common.Version=$version",
    '-o', $stagedApiPath,
    '.'
) -Description 'Backend build' -WorkingDirectory $workspace
if (-not (Test-Path -LiteralPath $stagedApiPath)) {
    throw "Backend build did not produce $stagedApiPath."
}

Invoke-ExternalCommand -FilePath $cloudflaredPath -Arguments @('--config', $tunnelConfig, 'tunnel', 'ingress', 'validate') -Description 'Cloudflare Tunnel ingress validation' -WorkingDirectory $workspace

$processEnvironment = @{}
if (-not $UseEnvironmentProxy) {
    $processEnvironment = @{
        HTTP_PROXY  = ''
        HTTPS_PROXY = ''
        ALL_PROXY   = ''
        NO_PROXY    = '127.0.0.1,localhost,::1,argotunnel.com,.argotunnel.com,cloudflare.com,.cloudflare.com,cloudflarestatus.com,.cloudflarestatus.com,api.seawork.ai,seawork.ai,.seawork.ai'
    }
}

$watchdogTask = Get-ScheduledTask -TaskName 'TokenRouterWatchdog' -ErrorAction SilentlyContinue
if ($null -ne $watchdogTask) {
    $matchingActions = @($watchdogTask.Actions | Where-Object {
        $_.WorkingDirectory -eq $workspace -and
        $_.Arguments.IndexOf($watchdogScript, [StringComparison]::OrdinalIgnoreCase) -ge 0
    })
    if ($matchingActions.Count -eq 0) {
        $watchdogTask = $null
    }
}
$watchdogTaskDisabled = $false
$deploymentCompleted = $false
try {
    if ($null -ne $watchdogTask -and $watchdogTask.State -ne 'Disabled') {
        Disable-ScheduledTask -InputObject $watchdogTask | Out-Null
        $watchdogTaskDisabled = $true
        Stop-ScheduledTask -InputObject $watchdogTask
    }
    Invoke-ExternalCommand -FilePath $powerShellPath `
        -Arguments @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $watchdogStop, '-Workspace', $workspace) `
        -Description 'Watchdog stop' -WorkingDirectory $workspace
    Stop-ManagedProcess -FilePath $cloudflaredPath
    Stop-ManagedProcess -FilePath $apiPath

    try {
        if (Test-Path -LiteralPath $apiPath) {
            Move-Item -LiteralPath $apiPath -Destination $backupApiPath -ErrorAction Stop
        }
        Move-Item -LiteralPath $stagedApiPath -Destination $apiPath -ErrorAction Stop
    } catch {
        if (-not (Test-Path -LiteralPath $apiPath) -and (Test-Path -LiteralPath $backupApiPath)) {
            Move-Item -LiteralPath $backupApiPath -Destination $apiPath -ErrorAction Stop
        }
        throw
    }

    $apiStdout = Join-Path $logDir "service-$stamp-stdout.log"
    $apiStderr = Join-Path $logDir "service-$stamp-stderr.log"
    try {
        $apiProcess = Start-ManagedProcess `
            -FilePath $apiPath `
            -Arguments @('--log-dir', $logDir) `
            -WorkingDirectory $workspace `
            -StandardOutputPath $apiStdout `
            -StandardErrorPath $apiStderr `
            -Environment $processEnvironment
        Wait-ForHttpStatus -Uri 'http://127.0.0.1:3000/api/status' -TimeoutSeconds 60
    } catch {
        $startupError = $_
        Stop-ManagedProcess -FilePath $apiPath
        if (Test-Path -LiteralPath $apiPath) {
            Move-Item -LiteralPath $apiPath -Destination $failedApiPath -ErrorAction Stop
        }
        if (Test-Path -LiteralPath $backupApiPath) {
            Move-Item -LiteralPath $backupApiPath -Destination $apiPath -ErrorAction Stop
            $rollbackStdout = Join-Path $logDir "service-$stamp-rollback-stdout.log"
            $rollbackStderr = Join-Path $logDir "service-$stamp-rollback-stderr.log"
            $null = Start-ManagedProcess `
                -FilePath $apiPath `
                -Arguments @('--log-dir', $logDir) `
                -WorkingDirectory $workspace `
                -StandardOutputPath $rollbackStdout `
                -StandardErrorPath $rollbackStderr `
                -Environment $processEnvironment
            Wait-ForHttpStatus -Uri 'http://127.0.0.1:3000/api/status' -TimeoutSeconds 60
        }
        throw $startupError
    }

    $tunnelStdout = Join-Path $logDir "cloudflared-$stamp-stdout.log"
    $tunnelStderr = Join-Path $logDir "cloudflared-$stamp-stderr.log"
    $tunnelConfigContents = Get-Content -LiteralPath $tunnelConfig -Raw
    $tunnelMatch = [regex]::Match($tunnelConfigContents, '(?m)^\s*tunnel:\s*(\S+)\s*$')
    if (-not $tunnelMatch.Success) {
        throw "Unable to read the tunnel ID from $tunnelConfig."
    }
    $tunnelProcess = Start-ManagedProcess `
        -FilePath $cloudflaredPath `
        -Arguments @('--config', $tunnelConfig, '--protocol', 'http2', 'tunnel', 'run', $tunnelMatch.Groups[1].Value) `
        -WorkingDirectory $workspace `
        -StandardOutputPath $tunnelStdout `
        -StandardErrorPath $tunnelStderr `
        -Environment $processEnvironment
    Wait-ForTunnelConnection -MetricsPorts $metricsPorts -TimeoutSeconds 60
    $deploymentCompleted = $true
} finally {
    try {
        if ($watchdogTaskDisabled) {
            Enable-ScheduledTask -InputObject $watchdogTask | Out-Null
        }
        Invoke-ExternalCommand -FilePath $powerShellPath `
            -Arguments @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $watchdogStart, '-Workspace', $workspace, '-TunnelConfig', $tunnelConfig) `
            -Description 'Watchdog start' -WorkingDirectory $workspace
    } catch {
        if ($deploymentCompleted) {
            throw
        }
        Write-Warning "Watchdog recovery failed: $($_.Exception.Message)"
    }
}

Write-Host "Deployment complete. token-router PID=$($apiProcess.Id), cloudflared PID=$($tunnelProcess.Id); watchdog ready."
