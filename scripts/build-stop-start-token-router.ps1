[CmdletBinding()]
param(
    [string]$Workspace = (Split-Path -Parent $PSScriptRoot),
    [switch]$UseEnvironmentProxy
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

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
                -not [string]::IsNullOrWhiteSpace($_.ExecutablePath) -and
                    [string]::Equals(
                        [System.IO.Path]::GetFullPath($_.ExecutablePath),
                        $resolvedPath,
                        [System.StringComparison]::OrdinalIgnoreCase
                    )
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
        [Parameter(Mandatory = $true)][string]$MetricsUri,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $lastError = "no metrics response"
    while ((Get-Date) -lt $deadline) {
        try {
            $response = Invoke-WebRequest -Uri $MetricsUri -NoProxy -UseBasicParsing -TimeoutSec 3
            $connectionMatches = [regex]::Matches(
                $response.Content,
                '(?m)^cloudflared_tunnel_ha_connections(?:\{[^}]*\})?\s+([0-9]+(?:\.[0-9]+)?)'
            )
            foreach ($connectionMatch in $connectionMatches) {
                if ([double]$connectionMatch.Groups[1].Value -gt 0) {
                    return
                }
            }
            $lastError = "cloudflared_tunnel_ha_connections is zero"
        } catch {
            $lastError = $_.Exception.Message
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for an active Cloudflare Tunnel connection at ${MetricsUri}: $lastError"
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
$apiPath = Join-Path $workspace 'new-api.exe'
$cloudflaredPath = Join-Path $workspace 'bin\cloudflared.exe'
$tunnelConfig = Join-Path $env:USERPROFILE '.cloudflared\config.yml'
$metricsUri = 'http://127.0.0.1:20241/metrics'
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$stagedApiPath = Join-Path $workspace "new-api-$stamp.next.exe"
$backupApiPath = Join-Path $backupDir "new-api-$stamp.exe"
$failedApiPath = Join-Path $backupDir "new-api-$stamp.failed.exe"

foreach ($path in @($webDir, $cloudflaredPath, $tunnelConfig)) {
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
if (Test-Path -LiteralPath (Join-Path $workspace 'VERSION')) {
    $version = Get-Content -LiteralPath (Join-Path $workspace 'VERSION') -Raw
    if ($null -ne $version) {
        $env:VITE_REACT_APP_VERSION = $version.Trim()
    }
}
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

Invoke-ExternalCommand -FilePath $goPath -Arguments @('build', '-o', $stagedApiPath, '.') -Description 'Backend build' -WorkingDirectory $workspace
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
        NO_PROXY    = '127.0.0.1,localhost,::1'
    }
}

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
    -Arguments @('--config', $tunnelConfig, 'tunnel', 'run', $tunnelMatch.Groups[1].Value) `
    -WorkingDirectory $workspace `
    -StandardOutputPath $tunnelStdout `
    -StandardErrorPath $tunnelStderr `
    -Environment $processEnvironment
Wait-ForTunnelConnection -MetricsUri $metricsUri -TimeoutSeconds 60

Write-Host "Deployment complete. new-api PID=$($apiProcess.Id), cloudflared PID=$($tunnelProcess.Id)."
