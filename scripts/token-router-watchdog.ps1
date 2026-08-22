[CmdletBinding()]
param(
    [string]$Workspace = '',
    [string]$TunnelConfig = '',
    [string]$ApiStatusUri = 'http://127.0.0.1:3000/api/status',
    [string]$MetricsUri = 'http://127.0.0.1:20241/metrics',
    [int]$HealthyIntervalSeconds = 60,
    [int]$UnhealthyIntervalSeconds = 15,
    [int]$FailureThreshold = 4,
    [int]$CheckTimeoutSeconds = 5,
    [int]$StartupWaitSeconds = 60,
    [int]$RestartWaitSeconds = 60,
    [int]$RestartCooldownSeconds = 120,
    [switch]$Once
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
$apiPath = Join-Path $workspace 'new-api.exe'
$cloudflaredPath = Join-Path $workspace 'bin\cloudflared.exe'
$pidFile = Join-Path $logDir 'token-router-watchdog.pid'
$watchdogLog = Join-Path $logDir 'token-router-watchdog.log'

function Resolve-TunnelConfigPath {
    param([string]$ConfiguredPath)

    $candidates = @()
    if (-not [string]::IsNullOrWhiteSpace($ConfiguredPath)) {
        $candidates += $ConfiguredPath
    }
    if (-not [string]::IsNullOrWhiteSpace($env:CLOUDFLARED_CONFIG)) {
        $candidates += $env:CLOUDFLARED_CONFIG
    }
    if (-not [string]::IsNullOrWhiteSpace($env:USERPROFILE)) {
        $candidates += (Join-Path $env:USERPROFILE '.cloudflared\config.yml')
    }
    $candidates += 'C:\Users\afeng\.cloudflared\config.yml'

    foreach ($candidate in $candidates) {
        if (-not [string]::IsNullOrWhiteSpace($candidate) -and (Test-Path -LiteralPath $candidate)) {
            return (Resolve-Path -LiteralPath $candidate).Path
        }
    }

    throw 'cloudflared config not found. Pass -TunnelConfig or set CLOUDFLARED_CONFIG.'
}

function Write-WatchdogLog {
    param(
        [Parameter(Mandatory = $true)][string]$Message,
        [ValidateSet('INFO', 'WARN', 'ERROR')][string]$Level = 'INFO'
    )

    $line = '{0} [{1}] {2}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $Level, $Message
    Add-Content -LiteralPath $watchdogLog -Value $line -Encoding UTF8
    Write-Host $line
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

    # Windows PowerShell 5.1 (used by scheduled tasks via powershell.exe) has no
    # Start-Process -Environment. Temporarily clear/set process env instead.
    $previous = @{}
    foreach ($key in @($Environment.Keys)) {
        $previous[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
        [Environment]::SetEnvironmentVariable($key, $Environment[$key], 'Process')
    }
    try {
        if ($Environment.Count -gt 0 -and $PSVersionTable.PSVersion.Major -ge 6) {
            $parameters.Environment = $Environment
        }
        return Start-Process @parameters
    } finally {
        foreach ($key in @($previous.Keys)) {
            [Environment]::SetEnvironmentVariable($key, $previous[$key], 'Process')
        }
    }
}

function Invoke-DirectWebRequest {
    param(
        [Parameter(Mandatory = $true)][string]$Uri,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds
    )

    $request = [System.Net.HttpWebRequest]::Create($Uri)
    $request.Method = 'GET'
    $request.Proxy = $null
    $request.Timeout = [Math]::Max(1, $TimeoutSeconds) * 1000
    $request.ReadWriteTimeout = [Math]::Max(1, $TimeoutSeconds) * 1000
    $request.KeepAlive = $false

    $response = $null
    $stream = $null
    $reader = $null
    try {
        $response = [System.Net.HttpWebResponse]$request.GetResponse()
        $stream = $response.GetResponseStream()
        $reader = New-Object System.IO.StreamReader($stream)
        $body = $reader.ReadToEnd()
        return [pscustomobject]@{
            StatusCode = [int]$response.StatusCode
            Content    = $body
        }
    } finally {
        if ($null -ne $reader) { $reader.Dispose() }
        if ($null -ne $stream) { $stream.Dispose() }
        if ($null -ne $response) { $response.Dispose() }
    }
}

function Get-DirectNoProxyEnvironment {
    return @{
        HTTP_PROXY  = ''
        HTTPS_PROXY = ''
        ALL_PROXY   = ''
        NO_PROXY    = '127.0.0.1,localhost,::1,argotunnel.com,.argotunnel.com,cloudflare.com,.cloudflare.com,cloudflarestatus.com,.cloudflarestatus.com,api.seawork.ai,seawork.ai,.seawork.ai'
    }
}

function Initialize-SleepPrevention {
    if (-not ('TokenRouter.SleepPreventer' -as [type])) {
        Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
namespace TokenRouter {
    public static class SleepPreventer {
        public const uint ES_SYSTEM_REQUIRED = 0x00000001;
        public const uint ES_AWAYMODE_REQUIRED = 0x00000040;
        public const uint ES_CONTINUOUS = 0x80000000;

        [DllImport("kernel32.dll")]
        public static extern uint SetThreadExecutionState(uint esFlags);
    }
}
'@
    }
}

function Set-SleepPrevention {
    param([Parameter(Mandatory = $true)][bool]$Prevent)

    Initialize-SleepPrevention
    if ($Prevent) {
        $flags = [TokenRouter.SleepPreventer]::ES_CONTINUOUS -bor [TokenRouter.SleepPreventer]::ES_SYSTEM_REQUIRED
        $result = [TokenRouter.SleepPreventer]::SetThreadExecutionState($flags)
        if ($result -eq 0) {
            $flags = $flags -bor [TokenRouter.SleepPreventer]::ES_AWAYMODE_REQUIRED
            $result = [TokenRouter.SleepPreventer]::SetThreadExecutionState($flags)
        }
        return $result -ne 0
    }

    $result = [TokenRouter.SleepPreventer]::SetThreadExecutionState([TokenRouter.SleepPreventer]::ES_CONTINUOUS)
    return $result -ne 0
}

function Enable-AlwaysOnPowerPolicy {
    $commands = @(
        @('/change', 'standby-timeout-ac', '0'),
        @('/change', 'standby-timeout-dc', '0'),
        @('/change', 'hibernate-timeout-ac', '0'),
        @('/change', 'hibernate-timeout-dc', '0'),
        @('/SETACVALUEINDEX', 'SCHEME_CURRENT', 'SUB_SLEEP', 'STANDBYIDLE', '0'),
        @('/SETDCVALUEINDEX', 'SCHEME_CURRENT', 'SUB_SLEEP', 'STANDBYIDLE', '0'),
        @('/SETACVALUEINDEX', 'SCHEME_CURRENT', 'SUB_SLEEP', 'HYBRIDSLEEP', '0'),
        @('/SETDCVALUEINDEX', 'SCHEME_CURRENT', 'SUB_SLEEP', 'HYBRIDSLEEP', '0'),
        @('/SETACVALUEINDEX', 'SCHEME_CURRENT', 'SUB_SLEEP', 'HIBERNATEIDLE', '0'),
        @('/SETDCVALUEINDEX', 'SCHEME_CURRENT', 'SUB_SLEEP', 'HIBERNATEIDLE', '0'),
        @('/SETACVALUEINDEX', 'SCHEME_CURRENT', 'SUB_BUTTONS', 'LIDACTION', '0'),
        @('/SETDCVALUEINDEX', 'SCHEME_CURRENT', 'SUB_BUTTONS', 'LIDACTION', '0'),
        @('/SETACTIVE', 'SCHEME_CURRENT')
    )

    foreach ($arguments in $commands) {
        $output = & powercfg.exe @arguments 2>&1
        if ($LASTEXITCODE -ne 0) {
            Write-WatchdogLog -Level 'WARN' -Message ("powercfg $($arguments -join ' ') failed: $output")
        }
    }
}

function Get-TunnelId {
    param([Parameter(Mandatory = $true)][string]$ConfigPath)

    $configContents = Get-Content -LiteralPath $ConfigPath -Raw
    $tunnelMatch = [regex]::Match($configContents, '(?m)^\s*tunnel:\s*(\S+)\s*$')
    if (-not $tunnelMatch.Success) {
        throw "Unable to read tunnel ID from $ConfigPath."
    }
    return $tunnelMatch.Groups[1].Value
}

function Test-NewApiHealth {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string]$StatusUri,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds
    )

    $processes = @(Get-ManagedProcesses -FilePath $FilePath)
    if ($processes.Count -eq 0) {
        return [pscustomobject]@{
            Healthy = $false
            Reason  = 'new-api process not running'
            Detail  = 'process_missing'
        }
    }

    try {
        $response = Invoke-DirectWebRequest -Uri $StatusUri -TimeoutSeconds $TimeoutSeconds
    } catch {
        return [pscustomobject]@{
            Healthy = $false
            Reason  = "api status unreachable: $($_.Exception.Message)"
            Detail  = 'status_unreachable'
        }
    }

    if ($response.StatusCode -lt 200 -or $response.StatusCode -ge 300) {
        return [pscustomobject]@{
            Healthy = $false
            Reason  = "api status HTTP $($response.StatusCode)"
            Detail  = 'status_http_error'
        }
    }

    return [pscustomobject]@{
        Healthy = $true
        Reason  = "ok status=$($response.StatusCode) pid=$($processes[0].ProcessId)"
        Detail  = 'ok'
    }
}

function Test-CloudflaredHealth {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string]$MetricsUri,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds
    )

    $processes = @(Get-ManagedProcesses -FilePath $FilePath)
    if ($processes.Count -eq 0) {
        return [pscustomobject]@{
            Healthy = $false
            Reason  = 'cloudflared process not running'
            Detail  = 'process_missing'
        }
    }

    try {
        $response = Invoke-DirectWebRequest -Uri $MetricsUri -TimeoutSeconds $TimeoutSeconds
    } catch {
        return [pscustomobject]@{
            Healthy = $false
            Reason  = "metrics unreachable: $($_.Exception.Message)"
            Detail  = 'metrics_unreachable'
        }
    }

    if ($response.StatusCode -lt 200 -or $response.StatusCode -ge 300) {
        return [pscustomobject]@{
            Healthy = $false
            Reason  = "metrics HTTP $($response.StatusCode)"
            Detail  = 'metrics_http_error'
        }
    }

    $connectionMatches = [regex]::Matches(
        $response.Content,
        '(?m)^cloudflared_tunnel_ha_connections(?:\{[^}]*\})?\s+([0-9]+(?:\.[0-9]+)?)'
    )
    if ($connectionMatches.Count -eq 0) {
        return [pscustomobject]@{
            Healthy = $false
            Reason  = 'metrics missing cloudflared_tunnel_ha_connections'
            Detail  = 'metrics_missing_metric'
        }
    }

    $maxConnections = 0.0
    foreach ($connectionMatch in $connectionMatches) {
        $value = [double]$connectionMatch.Groups[1].Value
        if ($value -gt $maxConnections) {
            $maxConnections = $value
        }
    }

    if ($maxConnections -le 0) {
        return [pscustomobject]@{
            Healthy = $false
            Reason  = 'cloudflared_tunnel_ha_connections is zero'
            Detail  = 'no_tunnel_connection'
        }
    }

    return [pscustomobject]@{
        Healthy = $true
        Reason  = "ok connections=$maxConnections pid=$($processes[0].ProcessId)"
        Detail  = 'ok'
    }
}

function Wait-ForCondition {
    param(
        [Parameter(Mandatory = $true)][scriptblock]$Checker,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds,
        [Parameter(Mandatory = $true)][string]$Description
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $lastReason = 'no check yet'
    while ((Get-Date) -lt $deadline) {
        $result = & $Checker
        if ($result.Healthy) {
            return $result
        }
        $lastReason = $result.Reason
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for ${Description}: $lastReason"
}

function Start-NewApiProcess {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string]$WorkingDirectory,
        [Parameter(Mandatory = $true)][string]$LogDirectory
    )

    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
    $stdoutPath = Join-Path $LogDirectory "new-api-watchdog-$stamp-stdout.log"
    $stderrPath = Join-Path $LogDirectory "new-api-watchdog-$stamp-stderr.log"
    return Start-ManagedProcess `
        -FilePath $FilePath `
        -Arguments @('--log-dir', $LogDirectory) `
        -WorkingDirectory $WorkingDirectory `
        -StandardOutputPath $stdoutPath `
        -StandardErrorPath $stderrPath `
        -Environment (Get-DirectNoProxyEnvironment)
}

function Start-CloudflaredProcess {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string]$ConfigPath,
        [Parameter(Mandatory = $true)][string]$WorkingDirectory,
        [Parameter(Mandatory = $true)][string]$LogDirectory
    )

    $tunnelId = Get-TunnelId -ConfigPath $ConfigPath
    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
    $stdoutPath = Join-Path $LogDirectory "cloudflared-watchdog-$stamp-stdout.log"
    $stderrPath = Join-Path $LogDirectory "cloudflared-watchdog-$stamp-stderr.log"
    return Start-ManagedProcess `
        -FilePath $FilePath `
        -Arguments @('--config', $ConfigPath, '--protocol', 'http2', 'tunnel', 'run', $tunnelId) `
        -WorkingDirectory $WorkingDirectory `
        -StandardOutputPath $stdoutPath `
        -StandardErrorPath $stderrPath `
        -Environment (Get-DirectNoProxyEnvironment)
}

function Restart-NewApi {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string]$WorkingDirectory,
        [Parameter(Mandatory = $true)][string]$LogDirectory,
        [Parameter(Mandatory = $true)][string]$StatusUri,
        [Parameter(Mandatory = $true)][int]$WaitSeconds
    )

    Write-WatchdogLog -Level 'WARN' -Message 'Restarting new-api'
    Stop-ManagedProcess -FilePath $FilePath
    $process = Start-NewApiProcess -FilePath $FilePath -WorkingDirectory $WorkingDirectory -LogDirectory $LogDirectory
    $health = Wait-ForCondition -TimeoutSeconds $WaitSeconds -Description 'new-api readiness' -Checker {
        Test-NewApiHealth -FilePath $FilePath -StatusUri $StatusUri -TimeoutSeconds 3
    }
    Write-WatchdogLog -Message "new-api restart complete. PID=$($process.Id); $($health.Reason)"
    return $process
}

function Restart-Cloudflared {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string]$ConfigPath,
        [Parameter(Mandatory = $true)][string]$WorkingDirectory,
        [Parameter(Mandatory = $true)][string]$LogDirectory,
        [Parameter(Mandatory = $true)][string]$MetricsUri,
        [Parameter(Mandatory = $true)][int]$WaitSeconds
    )

    $tunnelId = Get-TunnelId -ConfigPath $ConfigPath
    Write-WatchdogLog -Level 'WARN' -Message "Restarting cloudflared tunnel=$tunnelId"
    Stop-ManagedProcess -FilePath $FilePath
    $process = Start-CloudflaredProcess `
        -FilePath $FilePath `
        -ConfigPath $ConfigPath `
        -WorkingDirectory $WorkingDirectory `
        -LogDirectory $LogDirectory
    $health = Wait-ForCondition -TimeoutSeconds $WaitSeconds -Description 'cloudflared readiness' -Checker {
        Test-CloudflaredHealth -FilePath $FilePath -MetricsUri $MetricsUri -TimeoutSeconds 3
    }
    Write-WatchdogLog -Message "cloudflared restart complete. PID=$($process.Id); $($health.Reason)"
    return $process
}

function Test-ExistingWatchdog {
    param([Parameter(Mandatory = $true)][string]$PidFilePath)

    if (-not (Test-Path -LiteralPath $PidFilePath)) {
        return $false
    }

    $existingPidText = (Get-Content -LiteralPath $PidFilePath -Raw).Trim()
    $existingPid = 0
    if (-not [int]::TryParse($existingPidText, [ref]$existingPid)) {
        return $false
    }

    $existing = Get-Process -Id $existingPid -ErrorAction SilentlyContinue
    if ($null -eq $existing) {
        return $false
    }

    return $existing.Id -ne $PID
}

function Invoke-ServiceWatchCycle {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][scriptblock]$HealthChecker,
        [Parameter(Mandatory = $true)][scriptblock]$Restarter,
        [Parameter(Mandatory = $true)][hashtable]$State
    )

    $health = & $HealthChecker
    if ($health.Healthy) {
        if ($State.Failures -gt 0) {
            Write-WatchdogLog -Message "$Name recovered after $($State.Failures) failure(s). $($health.Reason)"
        } else {
            Write-WatchdogLog -Message "$Name $($health.Reason)"
        }
        $State.Failures = 0
        return $true
    }

    if ($health.Detail -eq 'process_missing') {
        Write-WatchdogLog -Level 'WARN' -Message "$Name missing: $($health.Reason)"
        $secondsSinceRestart = 0
        if ($null -ne $State.LastRestartAt) {
            $secondsSinceRestart = [int][Math]::Min([Math]::Max(((Get-Date) - $State.LastRestartAt).TotalSeconds, 0), [int]::MaxValue)
        }
        if ($null -ne $State.LastRestartAt -and $secondsSinceRestart -lt $RestartCooldownSeconds) {
            Write-WatchdogLog -Level 'WARN' -Message ("$Name start skipped due to cooldown ({0}s remaining)." -f ($RestartCooldownSeconds - $secondsSinceRestart))
            $State.Failures++
            return $false
        }

        try {
            $null = & $Restarter
            $State.LastRestartAt = Get-Date
            $State.Failures = 0
            return $true
        } catch {
            Write-WatchdogLog -Level 'ERROR' -Message "$Name start failed: $($_.Exception.Message)"
            $State.LastRestartAt = Get-Date
            $State.Failures = 0
            return $false
        }
    }

    $State.Failures++
    Write-WatchdogLog -Level 'WARN' -Message ("$Name failure {0}/{1}: {2}" -f $State.Failures, $FailureThreshold, $health.Reason)

    if ($State.Failures -lt $FailureThreshold) {
        return $false
    }

    $secondsSinceRestart = 0
    if ($null -ne $State.LastRestartAt) {
        $secondsSinceRestart = [int][Math]::Min([Math]::Max(((Get-Date) - $State.LastRestartAt).TotalSeconds, 0), [int]::MaxValue)
    }
    if ($null -ne $State.LastRestartAt -and $secondsSinceRestart -lt $RestartCooldownSeconds) {
        Write-WatchdogLog -Level 'WARN' -Message ("$Name restart skipped due to cooldown ({0}s remaining)." -f ($RestartCooldownSeconds - $secondsSinceRestart))
        return $false
    }

    try {
        $null = & $Restarter
        $State.LastRestartAt = Get-Date
        $State.Failures = 0
        return $true
    } catch {
        Write-WatchdogLog -Level 'ERROR' -Message "$Name restart failed: $($_.Exception.Message)"
        $State.LastRestartAt = Get-Date
        $State.Failures = 0
        return $false
    }
}

$null = New-Item -ItemType Directory -Path $logDir -Force
$tunnelConfigPath = Resolve-TunnelConfigPath -ConfiguredPath $TunnelConfig

if (-not $Once) {
    if (Test-ExistingWatchdog -PidFilePath $pidFile) {
        $existingPid = (Get-Content -LiteralPath $pidFile -Raw).Trim()
        throw "Another token-router watchdog is already running (PID $existingPid)."
    }
    Set-Content -LiteralPath $pidFile -Value $PID -Encoding ascii
}

try {
    foreach ($path in @($apiPath, $cloudflaredPath, $tunnelConfigPath)) {
        if (-not (Test-Path -LiteralPath $path)) {
            throw "Required path does not exist: $path"
        }
    }

    Write-WatchdogLog -Message ("Watchdog started. healthy={0}s unhealthy={1}s threshold={2} api={3} metrics={4}" -f `
            $HealthyIntervalSeconds, $UnhealthyIntervalSeconds, $FailureThreshold, $ApiStatusUri, $MetricsUri)

    if (-not $Once) {
        $apiHealth = Test-NewApiHealth -FilePath $apiPath -StatusUri $ApiStatusUri -TimeoutSeconds $CheckTimeoutSeconds
        if (-not $apiHealth.Healthy) {
            Write-WatchdogLog -Level 'WARN' -Message "Boot ensure new-api: $($apiHealth.Reason)"
            if ($apiHealth.Detail -eq 'process_missing') {
                $null = Start-NewApiProcess -FilePath $apiPath -WorkingDirectory $workspace -LogDirectory $logDir
            } else {
                $null = Restart-NewApi -FilePath $apiPath -WorkingDirectory $workspace -LogDirectory $logDir -StatusUri $ApiStatusUri -WaitSeconds $StartupWaitSeconds
            }
            $null = Wait-ForCondition -TimeoutSeconds $StartupWaitSeconds -Description 'new-api readiness' -Checker {
                Test-NewApiHealth -FilePath $apiPath -StatusUri $ApiStatusUri -TimeoutSeconds 3
            }
            Write-WatchdogLog -Message 'Boot ensure new-api ready.'
        } else {
            Write-WatchdogLog -Message "Boot new-api already healthy. $($apiHealth.Reason)"
        }

        $tunnelHealth = Test-CloudflaredHealth -FilePath $cloudflaredPath -MetricsUri $MetricsUri -TimeoutSeconds $CheckTimeoutSeconds
        if (-not $tunnelHealth.Healthy) {
            Write-WatchdogLog -Level 'WARN' -Message "Boot ensure cloudflared: $($tunnelHealth.Reason)"
            if ($tunnelHealth.Detail -eq 'process_missing') {
                $null = Start-CloudflaredProcess -FilePath $cloudflaredPath -ConfigPath $tunnelConfigPath -WorkingDirectory $workspace -LogDirectory $logDir
            } else {
                $null = Restart-Cloudflared -FilePath $cloudflaredPath -ConfigPath $tunnelConfigPath -WorkingDirectory $workspace -LogDirectory $logDir -MetricsUri $MetricsUri -WaitSeconds $StartupWaitSeconds
            }
            $null = Wait-ForCondition -TimeoutSeconds $StartupWaitSeconds -Description 'cloudflared readiness' -Checker {
                Test-CloudflaredHealth -FilePath $cloudflaredPath -MetricsUri $MetricsUri -TimeoutSeconds 3
            }
            Write-WatchdogLog -Message 'Boot ensure cloudflared ready.'
        } else {
            Write-WatchdogLog -Message "Boot cloudflared already healthy. $($tunnelHealth.Reason)"
        }
    }

    $apiState = @{
        Failures      = 0
        LastRestartAt = $null
    }
    $tunnelState = @{
        Failures      = 0
        LastRestartAt = $null
    }
    $sleepPreventionHeld = $false
    $powerPolicyApplied = $false

    while ($true) {
        $apiOk = Invoke-ServiceWatchCycle -Name 'new-api' -State $apiState `
            -HealthChecker {
                Test-NewApiHealth -FilePath $apiPath -StatusUri $ApiStatusUri -TimeoutSeconds $CheckTimeoutSeconds
            } `
            -Restarter {
                Restart-NewApi -FilePath $apiPath -WorkingDirectory $workspace -LogDirectory $logDir -StatusUri $ApiStatusUri -WaitSeconds $RestartWaitSeconds
            }

        $tunnelOk = Invoke-ServiceWatchCycle -Name 'cloudflared' -State $tunnelState `
            -HealthChecker {
                Test-CloudflaredHealth -FilePath $cloudflaredPath -MetricsUri $MetricsUri -TimeoutSeconds $CheckTimeoutSeconds
            } `
            -Restarter {
                Restart-Cloudflared -FilePath $cloudflaredPath -ConfigPath $tunnelConfigPath -WorkingDirectory $workspace -LogDirectory $logDir -MetricsUri $MetricsUri -WaitSeconds $RestartWaitSeconds
            }

        $serviceRunning = (@(Get-ManagedProcesses -FilePath $apiPath).Count -gt 0) -or (@(Get-ManagedProcesses -FilePath $cloudflaredPath).Count -gt 0)
        if ($serviceRunning) {
            if (-not $powerPolicyApplied) {
                Enable-AlwaysOnPowerPolicy
                $powerPolicyApplied = $true
                Write-WatchdogLog -Message 'Applied always-on power policy: sleep/hibernate/lid sleep disabled.'
            }
            if (-not $sleepPreventionHeld) {
                if (Set-SleepPrevention -Prevent $true) {
                    $sleepPreventionHeld = $true
                    Write-WatchdogLog -Message 'Sleep prevention enabled while new-api or cloudflared is running.'
                } else {
                    Write-WatchdogLog -Level 'WARN' -Message 'Failed to enable sleep prevention.'
                }
            }
        } elseif ($sleepPreventionHeld) {
            if (Set-SleepPrevention -Prevent $false) {
                Write-WatchdogLog -Message 'Sleep prevention released; new-api and cloudflared are not running.'
            }
            $sleepPreventionHeld = $false
        }

        if ($Once) {
            if (-not ($apiOk -and $tunnelOk)) {
                exit 1
            }
            break
        }

        if ($apiOk -and $tunnelOk) {
            $sleepSeconds = $HealthyIntervalSeconds
        } else {
            $sleepSeconds = $UnhealthyIntervalSeconds
        }

        Start-Sleep -Seconds $sleepSeconds
    }
} catch {
    Write-WatchdogLog -Level 'ERROR' -Message "Watchdog fatal: $($_.Exception.Message)"
    throw
} finally {
    try {
        $null = Set-SleepPrevention -Prevent $false
    } catch {
    }
    if (-not $Once -and (Test-Path -LiteralPath $pidFile)) {
        $currentPidText = (Get-Content -LiteralPath $pidFile -Raw -ErrorAction SilentlyContinue)
        if ($null -ne $currentPidText -and $currentPidText.Trim() -eq [string]$PID) {
            Remove-Item -LiteralPath $pidFile -Force -ErrorAction SilentlyContinue
        }
    }
    Write-WatchdogLog -Message 'Watchdog stopped.'
}
