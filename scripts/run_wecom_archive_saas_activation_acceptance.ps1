[CmdletBinding()]
param(
    [ValidateSet('seed', 'verify', 'cleanup', 'status')]
    [string]$Action = 'status',
    [switch]$DryRun,
    [switch]$NoBuild
)

$ErrorActionPreference = 'Stop'
$ProjectName = 'mochat-wecom-acceptance-20260827'
$Dataset = 'MOCHAT-LOCAL-ACCEPTANCE-20260827'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$ComposeFile = Join-Path $RepositoryRoot 'deploy/local-acceptance/docker-compose.yml'
$RuntimeDirectory = Join-Path $RepositoryRoot '.tmp-wecom-acceptance-runtime'
$EnvironmentFile = Join-Path $RuntimeDirectory '.env.local'

function New-RandomBytes([int]$Length) {
    $bytes = New-Object byte[] $Length
    $generator = [Security.Cryptography.RandomNumberGenerator]::Create()
    try {
        $generator.GetBytes($bytes)
        return $bytes
    } finally {
        $generator.Dispose()
    }
}

function New-RandomText([int]$Bytes = 32) {
    return [Convert]::ToBase64String((New-RandomBytes $Bytes)).TrimEnd('=').Replace('+', '-').Replace('/', '_')
}

function New-HexKey {
    return -join ((New-RandomBytes 32) | ForEach-Object { $_.ToString('x2') })
}

function Initialize-RuntimeFiles {
    New-Item -ItemType Directory -Force -Path $RuntimeDirectory | Out-Null
    $saasMfaPath = Join-Path $RuntimeDirectory 'saas-mfa.key'
    $dashboardMfaPath = Join-Path $RuntimeDirectory 'dashboard-mfa.key'
    $dashboardPasswordPath = Join-Path $RuntimeDirectory 'dashboard-acceptance-password'
    $saasAdminPasswordPath = Join-Path $RuntimeDirectory 'saas-admin-password'
    if (-not (Test-Path -LiteralPath $saasMfaPath)) {
        [IO.File]::WriteAllText($saasMfaPath, (New-HexKey), [Text.Encoding]::ASCII)
    }
    if (-not (Test-Path -LiteralPath $dashboardMfaPath)) {
        [IO.File]::WriteAllText($dashboardMfaPath, (New-HexKey), [Text.Encoding]::ASCII)
    }
    if (-not (Test-Path -LiteralPath $dashboardPasswordPath)) {
        [IO.File]::WriteAllText($dashboardPasswordPath, (New-RandomText 24), [Text.Encoding]::ASCII)
    }
    if (-not (Test-Path -LiteralPath $saasAdminPasswordPath)) {
        [IO.File]::WriteAllText($saasAdminPasswordPath, (New-RandomText 24), [Text.Encoding]::ASCII)
    }
    if (-not (Test-Path -LiteralPath $EnvironmentFile)) {
        $runtimeForCompose = $RuntimeDirectory.Replace('\', '/')
        $lines = @(
            "MOCHAT_ACCEPTANCE_MYSQL_ROOT_PASSWORD=$(New-RandomText)",
            "MOCHAT_ACCEPTANCE_MYSQL_PASSWORD=$(New-RandomText)",
            "MOCHAT_ACCEPTANCE_BRIDGE_TOKEN=$(New-RandomText 48)",
            "MOCHAT_ACCEPTANCE_SIMPLE_JWT_SECRET=$(New-RandomText 48)",
            "MOCHAT_ACCEPTANCE_SAAS_JWT_SECRET=$(New-RandomText 48)",
            "MOCHAT_ACCEPTANCE_DASHBOARD_JWT_SECRET=$(New-RandomText 48)",
            "MOCHAT_ACCEPTANCE_SIDEBAR_JWT_SECRET=$(New-RandomText 48)",
            "MOCHAT_ACCEPTANCE_WECOM_KEY=$(New-HexKey)",
            "MOCHAT_ACCEPTANCE_RUNTIME_DIR=$runtimeForCompose"
        )
        [IO.File]::WriteAllLines($EnvironmentFile, $lines, [Text.Encoding]::ASCII)
    }
}

function Invoke-Compose([string[]]$Arguments) {
    & docker compose -p $ProjectName --env-file $EnvironmentFile -f $ComposeFile @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose command failed with exit code $LASTEXITCODE"
    }
}

function Start-AcceptanceServices {
    $arguments = @('up', '-d')
    if (-not $NoBuild) {
        $arguments += '--build'
    }
    $arguments += @('mysql', 'redis', 'bridge', 'app')
    Invoke-Compose $arguments
}

Initialize-RuntimeFiles
Write-Host "Dataset: $Dataset (本地验收 / 非生产)"

switch ($Action) {
    'seed' {
        Start-AcceptanceServices
        Invoke-Compose @('--profile', 'tools', 'run', '--rm', 'bootstrap')
        Invoke-Compose @('--profile', 'tools', 'run', '--rm', 'acceptance', 'seed')
        Invoke-Compose @('ps')
    }
    'verify' {
        Invoke-Compose @('--profile', 'tools', 'run', '--rm', 'acceptance', 'verify')
    }
    'cleanup' {
        $arguments = @('--profile', 'tools', 'run', '--rm', 'acceptance', 'cleanup')
        if ($DryRun) {
            $arguments += '-dry-run'
        }
        Invoke-Compose $arguments
    }
    'status' {
        Invoke-Compose @('ps')
        Write-Host 'Dashboard: http://127.0.0.1:19080'
        Write-Host 'SaaS 管理端: http://127.0.0.1:19080/saas/'
        Write-Host 'Fixture bridge health: http://127.0.0.1:19091/healthz'
        Write-Host 'MariaDB: 127.0.0.1:19016; Redis: 127.0.0.1:29089'
        Write-Host 'Dashboard 本地验收账号: 19008208270；随机密码仅保存在 ignored runtime 文件 dashboard-acceptance-password'
        Write-Host 'SaaS 本地验收账号: mochat-local-acceptance-admin；随机密码仅保存在 ignored runtime 文件 saas-admin-password'
    }
}
