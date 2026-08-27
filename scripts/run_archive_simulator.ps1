[CmdletBinding(PositionalBinding = $false)]
param(
    [string]$ProjectName = 'mochat-go-desktop',
    [string]$DockerCommand = 'docker',
    [Parameter(Position = 0, ValueFromRemainingArguments = $true)]
    [string[]]$SimulatorArguments
)

$ErrorActionPreference = 'Stop'

$repositoryRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $repositoryRoot 'deploy\standalone\docker-compose.yml'
$localEnvironmentFile = Join-Path $repositoryRoot 'deploy\standalone\.env.local'
$secretRoot = if ([string]::IsNullOrWhiteSpace($env:MOCHAT_DOCKER_DESKTOP_SECRET_DIR)) {
    Join-Path (Split-Path -Parent $repositoryRoot) "output\docker-desktop-secrets\$ProjectName"
} else {
    $env:MOCHAT_DOCKER_DESKTOP_SECRET_DIR
}

function Import-LocalEnvironmentFile {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return }
    foreach ($rawLine in Get-Content -LiteralPath $Path) {
        $line = $rawLine.Trim()
        if (-not $line -or $line.StartsWith('#')) { continue }
        $separator = $line.IndexOf('=')
        if ($separator -le 0) { continue }
        $name = $line.Substring(0, $separator).Trim()
        $value = $line.Substring($separator + 1).Trim()
        if ($value.Length -ge 2 -and (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'")))) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        if ($name -match '^[A-Z][A-Z0-9_]*$' -and $value -notmatch '^CHANGE_ME|^/secure/path/' -and [string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($name, 'Process'))) {
            [Environment]::SetEnvironmentVariable($name, $value, 'Process')
        }
    }
}

function Import-LocalSecret {
    param(
        [string]$EnvironmentName,
        [string]$FileName
    )

    if (-not [string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($EnvironmentName, 'Process'))) { return }
    $path = Join-Path $secretRoot $FileName
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "Local simulator secret is missing; run deploy_docker_desktop.ps1 -EnableArchiveFixture first: $FileName"
    }
    $value = (Get-Content -LiteralPath $path -Raw).Trim()
    if ($value -notmatch '^[a-f0-9]{64}$') {
        throw "Local simulator secret file is invalid: $FileName"
    }
    [Environment]::SetEnvironmentVariable($EnvironmentName, $value, 'Process')
}

function Import-LocalSecretFile {
    param(
        [string]$EnvironmentName,
        [string]$FileName
    )

    if (-not [string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($EnvironmentName, 'Process'))) { return }
    $path = Join-Path $secretRoot $FileName
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "Local simulator secret is missing; run deploy_docker_desktop.ps1 -EnableArchiveFixture first: $FileName"
    }
    $value = (Get-Content -LiteralPath $path -Raw).Trim()
    if ($value -notmatch '^[a-f0-9]{64}$') {
        throw "Local simulator secret file is invalid: $FileName"
    }
    [Environment]::SetEnvironmentVariable($EnvironmentName, $path, 'Process')
}

if (-not $SimulatorArguments -or $SimulatorArguments.Count -eq 0) {
    throw 'Provide a simulator subcommand: seed, send, status, or cleanup'
}
if (-not (Test-Path -LiteralPath $composeFile -PathType Leaf)) {
    throw "Compose file is missing: $composeFile"
}

Import-LocalEnvironmentFile -Path $localEnvironmentFile
Import-LocalSecretFile -EnvironmentName 'MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE' -FileName 'saas-admin-mfa.key'
Import-LocalSecretFile -EnvironmentName 'MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_FILE' -FileName 'dashboard-mfa.key'
Import-LocalSecret -EnvironmentName 'MOCHAT_SAAS_ADMIN_JWT_SECRET' -FileName 'saas-admin-jwt.key'
Import-LocalSecret -EnvironmentName 'MOCHAT_DASHBOARD_JWT_SECRET' -FileName 'dashboard-jwt.key'
Import-LocalSecret -EnvironmentName 'MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY' -FileName 'wecom-credentials.key'
Import-LocalSecret -EnvironmentName 'MOCHAT_ARCHIVE_BRIDGE_BEARER' -FileName 'archive-bridge-bearer.key'
Import-LocalSecret -EnvironmentName 'MOCHAT_ARCHIVE_FIXTURE_ADMIN_BEARER' -FileName 'archive-fixture-admin-bearer.key'

$env:MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_ID = if ([string]::IsNullOrWhiteSpace($env:MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_ID)) { 'local-saas-mfa-primary' } else { $env:MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_ID }
$env:MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_ID = if ([string]::IsNullOrWhiteSpace($env:MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_ID)) { 'local-dashboard-mfa-primary' } else { $env:MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_ID }
$env:MOCHAT_GO_ENABLE_DURABLE_WORK_MESSAGE_ARCHIVE = '1'
$env:MOCHAT_ARCHIVE_FIXTURE_ENABLED = 'true'
$env:MOCHAT_GO_ENABLE_WECOM_SUITE_CALLBACK = 'true'
$env:MOCHAT_GO_WECOM_SUITE_ID = 'ww-local-fixture-suite'
$env:MOCHAT_GO_WECOM_SUITE_SECRET = 'local-fixture-suite-secret'
$env:MOCHAT_GO_WECOM_SUITE_CALLBACK_TOKEN = 'local-fixture-callback-token'
$env:MOCHAT_GO_WECOM_SUITE_ENCODING_AES_KEY = 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG'

$composeArguments = @(
    'compose',
    '--project-name', $ProjectName,
    '-f', $composeFile,
    '--profile', 'app',
    '--profile', 'archive-tools',
    'run', '--rm', 'archive-simulator'
)
& $DockerCommand @composeArguments @SimulatorArguments
if ($LASTEXITCODE -ne 0) {
    throw "Local archive simulator failed with exit code $LASTEXITCODE"
}
