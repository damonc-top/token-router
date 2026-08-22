[CmdletBinding()]
param(
    [string]$Workspace = '',
    [string]$MetricsUri = 'http://127.0.0.1:20241/metrics',
    [int]$HealthyIntervalSeconds = 60,
    [int]$UnhealthyIntervalSeconds = 15,
    [int]$FailureThreshold = 4,
    [int]$CheckTimeoutSeconds = 5,
    [int]$RestartWaitSeconds = 60,
    [int]$RestartCooldownSeconds = 120,
    [switch]$Once
)

$script = Join-Path $PSScriptRoot 'token-router-watchdog.ps1'
$forward = @{
    Workspace                = $Workspace
    MetricsUri               = $MetricsUri
    HealthyIntervalSeconds   = $HealthyIntervalSeconds
    UnhealthyIntervalSeconds = $UnhealthyIntervalSeconds
    FailureThreshold         = $FailureThreshold
    CheckTimeoutSeconds      = $CheckTimeoutSeconds
    RestartWaitSeconds       = $RestartWaitSeconds
    RestartCooldownSeconds   = $RestartCooldownSeconds
}
if ($Once) { $forward.Once = $true }
& $script @forward
