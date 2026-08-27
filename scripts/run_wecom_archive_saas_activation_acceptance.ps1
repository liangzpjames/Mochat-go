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

function Protect-RuntimePath([string]$Path) {
    if ($env:OS -ne 'Windows_NT') {
        return
    }
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent().Name
    & icacls.exe $Path '/inheritance:r' '/grant:r' "${identity}:(OI)(CI)(F)" '*S-1-5-18:(OI)(CI)(F)' '*S-1-5-32-544:(OI)(CI)(F)' | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "failed to restrict runtime ACL grants"
    }
    & icacls.exe $Path '/remove:g' '*S-1-5-11' '*S-1-5-32-545' | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "failed to remove inherited runtime ACL groups"
    }
    foreach ($item in Get-ChildItem -LiteralPath $Path -Recurse -Force) {
        $grant = if ($item.PSIsContainer) { "${identity}:(OI)(CI)(F)" } else { "${identity}:(F)" }
        $systemGrant = if ($item.PSIsContainer) { '*S-1-5-18:(OI)(CI)(F)' } else { '*S-1-5-18:(F)' }
        $adminGrant = if ($item.PSIsContainer) { '*S-1-5-32-544:(OI)(CI)(F)' } else { '*S-1-5-32-544:(F)' }
        & icacls.exe $item.FullName '/inheritance:r' '/grant:r' $grant $systemGrant $adminGrant | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "failed to restrict child runtime ACL grants"
        }
        & icacls.exe $item.FullName '/remove:g' '*S-1-5-11' '*S-1-5-32-545' | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "failed to remove child runtime ACL groups"
        }
    }
}

function Initialize-RuntimeFiles {
    New-Item -ItemType Directory -Force -Path $RuntimeDirectory | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $RuntimeDirectory 'activation') | Out-Null
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
    Protect-RuntimePath $RuntimeDirectory
}

function Invoke-Compose([string[]]$Arguments) {
    & docker compose -p $ProjectName --env-file $EnvironmentFile -f $ComposeFile @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose command failed with exit code $LASTEXITCODE"
    }
}

function Ensure-AcceptanceImage {
    if (-not $NoBuild) {
        Invoke-Compose @('build', 'acceptance')
    }
}

function Test-CheckpointState([string]$State) {
    & docker compose -p $ProjectName --env-file $EnvironmentFile -f $ComposeFile --profile tools run --rm acceptance checkpoint -checkpoint-state $State 2>$null | Out-Null
    return $LASTEXITCODE -eq 0
}

function Wait-CheckpointState([string]$State, [int]$Attempts = 120) {
    for ($attempt = 0; $attempt -lt $Attempts; $attempt++) {
        if (Test-CheckpointState $State) {
            return
        }
        Start-Sleep -Milliseconds 250
    }
    throw "acceptance checkpoint state '$State' was not observed"
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
        Ensure-AcceptanceImage
        Start-AcceptanceServices
		Invoke-Compose @('stop', 'worker')
        Invoke-Compose @('--profile', 'tools', 'run', '--rm', 'bootstrap')
		Invoke-Compose @('--profile', 'tools', 'run', '--rm', 'acceptance', 'seed', '-defer-media')
		if (-not (Test-CheckpointState 'recovered')) {
			Invoke-Compose @('up', '-d', 'worker')
			Wait-CheckpointState 'partial'
			# A zero-second stop interrupts the active worker with SIGKILL after a real persisted checkpoint.
			Invoke-Compose @('stop', '-t', '0', 'worker')
			Invoke-Compose @('--profile', 'tools', 'run', '--rm', 'acceptance', 'checkpoint', '-checkpoint-state', 'partial')
			Invoke-Compose @('--profile', 'tools', 'run', '--rm', 'acceptance', 'checkpoint', '-checkpoint-state', 'expire')
			Invoke-Compose @('up', '-d', 'worker')
			Wait-CheckpointState 'recovered'
		}
		Invoke-Compose @('--profile', 'tools', 'run', '--rm', 'acceptance', 'seed')
        Invoke-Compose @('ps')
    }
    'verify' {
		Ensure-AcceptanceImage
		Invoke-Compose @('up', '-d', 'worker')
		Invoke-Compose @('--profile', 'tools', 'run', '--rm', 'acceptance', 'checkpoint', '-checkpoint-state', 'recovered')
        Invoke-Compose @('--profile', 'tools', 'run', '--rm', 'acceptance', 'verify')
    }
    'cleanup' {
		Ensure-AcceptanceImage
		Invoke-Compose @('stop', 'worker')
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
