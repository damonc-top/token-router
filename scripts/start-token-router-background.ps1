$ErrorActionPreference = 'Stop'

$workspace = Split-Path -Parent $PSScriptRoot
$logDir = Join-Path $workspace 'logs'
New-Item -ItemType Directory -Path $logDir -Force | Out-Null

$directEnv = @{
    HTTP_PROXY  = ''
    HTTPS_PROXY = ''
    ALL_PROXY   = ''
    NO_PROXY    = '127.0.0.1,localhost,::1,argotunnel.com,.argotunnel.com,cloudflare.com,.cloudflare.com,cloudflarestatus.com,.cloudflarestatus.com,api.seawork.ai,seawork.ai,.seawork.ai'
}

function Start-DirectProcess {
    param(
        [string]$FilePath,
        [string[]]$ArgumentList,
        [string]$StandardOutputPath,
        [string]$StandardErrorPath
    )

    $previous = @{}
    foreach ($key in @($directEnv.Keys)) {
        $previous[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
        [Environment]::SetEnvironmentVariable($key, $directEnv[$key], 'Process')
    }
    try {
        Start-Process -FilePath $FilePath `
            -ArgumentList $ArgumentList `
            -WorkingDirectory $workspace `
            -RedirectStandardOutput $StandardOutputPath `
            -RedirectStandardError $StandardErrorPath `
            -WindowStyle Hidden
    } finally {
        foreach ($key in @($previous.Keys)) {
            [Environment]::SetEnvironmentVariable($key, $previous[$key], 'Process')
        }
    }
}

if ($null -eq (Get-Process -Name new-api -ErrorAction SilentlyContinue)) {
    Start-DirectProcess `
        -FilePath (Join-Path $workspace 'new-api.exe') `
        -ArgumentList @('--log-dir', $logDir) `
        -StandardOutputPath (Join-Path $logDir 'service-stdout.log') `
        -StandardErrorPath (Join-Path $logDir 'service-stderr.log')
}

if ($null -eq (Get-Process -Name cloudflared -ErrorAction SilentlyContinue)) {
    Start-DirectProcess `
        -FilePath (Join-Path $workspace 'bin\cloudflared.exe') `
        -ArgumentList @('--config', (Join-Path $env:USERPROFILE '.cloudflared\config.yml'), '--protocol', 'http2', 'tunnel', 'run', '782f4334-8554-4eab-8182-ac66ca77061e') `
        -StandardOutputPath (Join-Path $logDir 'cloudflared-stdout.log') `
        -StandardErrorPath (Join-Path $logDir 'cloudflared-stderr.log')
}
