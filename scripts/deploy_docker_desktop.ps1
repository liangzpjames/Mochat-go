[CmdletBinding()]
param(
    [switch]$ResetData,
    [switch]$DryRun,
    [switch]$SkipHttpCheck,
    [switch]$EnableArchiveFixture,
    [switch]$ApproveControlledMigrations,
    [string]$ControlledRequestID = '',
    [string]$ControlledMaintenanceConfirmationFile = '',
    [string]$ProjectName = 'mochat-go-desktop',
    [ValidateRange(1, 65535)][int]$DashboardPort = 18080,
    [ValidateRange(1, 65535)][int]$SidebarPort = 18081,
    [ValidateRange(1, 65535)][int]$OperationPort = 18082,
    [ValidateRange(1, 65535)][int]$MySQLPort = 13316,
    [ValidateRange(1, 65535)][int]$RedisPort = 26389,
    [ValidateSet('completed', 'controlled_pending')][string]$DryRunMigrationState = 'completed',
    [string]$DockerCommand = 'docker'
)

$ErrorActionPreference = 'Stop'

$repositoryRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $repositoryRoot 'deploy\standalone\docker-compose.yml'
$composeArguments = @(
    'compose',
    '--project-name', $ProjectName,
    '-f', $composeFile,
    '--profile', 'app'
)
$localEnvironmentFile = Join-Path $repositoryRoot 'deploy\standalone\.env.local'

function Import-LocalEnvironmentFile {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return
    }
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
        if ($name -notmatch '^[A-Z][A-Z0-9_]*$' -or $value -match '^CHANGE_ME|^/secure/path/') { continue }
        if ([string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($name, 'Process'))) {
            [Environment]::SetEnvironmentVariable($name, $value, 'Process')
        }
    }
}

function New-CryptographicSecret {
    $bytes = New-Object byte[] 32
    $generator = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try {
        $generator.GetBytes($bytes)
    } finally {
        $generator.Dispose()
    }
    return ([BitConverter]::ToString($bytes).Replace('-', '')).ToLowerInvariant()
}

function Protect-LocalSecretFile {
    param([string]$Path)

    if ($env:OS -ne 'Windows_NT') {
        return
    }
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent().Name
    & icacls.exe $Path '/inheritance:r' '/grant:r' "${identity}:(F)" '*S-1-5-18:(F)' '*S-1-5-32-544:(F)' | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "无法限制本地密钥文件 ACL：$Path"
    }
    & icacls.exe $Path '/remove:g' '*S-1-5-11' '*S-1-5-32-545' | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "无法移除本地密钥文件的宽泛 ACL：$Path"
    }
}

function Get-OrCreateSecretFile {
    param(
        [string]$Directory,
        [string]$Name
    )

    $path = Join-Path $Directory $Name
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        New-Item -ItemType Directory -Path $Directory -Force | Out-Null
        [System.IO.File]::WriteAllText($path, (New-CryptographicSecret), [System.Text.Encoding]::ASCII)
    }
    $rawValue = Get-Content -LiteralPath $path -Raw
    $value = if ($null -eq $rawValue) { '' } else { ([string]$rawValue).Trim() }
    if ($value -notmatch '^[a-f0-9]{64}$') {
        throw "本地密钥文件格式无效：$path"
    }
    return $path
}

function Get-DefaultSecretRoot {
    $fallbackRoot = Split-Path -Parent $repositoryRoot
    $commonDirectory = (& git -C $repositoryRoot rev-parse --git-common-dir 2>$null | Select-Object -First 1)
    if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($commonDirectory)) {
        $commonDirectory = $commonDirectory.Trim()
        if (-not [System.IO.Path]::IsPathRooted($commonDirectory)) {
            $commonDirectory = Join-Path $repositoryRoot $commonDirectory
        }
        $mainRepositoryRoot = Split-Path -Parent ([System.IO.Path]::GetFullPath($commonDirectory))
        $fallbackRoot = Split-Path -Parent $mainRepositoryRoot
    }
    return (Join-Path $fallbackRoot "output\docker-desktop-secrets\$ProjectName")
}

function Initialize-LocalIdentityRealmSecrets {
    Import-LocalEnvironmentFile -Path $localEnvironmentFile
    $secretRoot = if ([string]::IsNullOrWhiteSpace($env:MOCHAT_DOCKER_DESKTOP_SECRET_DIR)) {
        Get-DefaultSecretRoot
    } else {
        $env:MOCHAT_DOCKER_DESKTOP_SECRET_DIR
    }

    if ($DryRun) {
        if ([string]::IsNullOrWhiteSpace($env:MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE)) {
            $env:MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE = Join-Path $secretRoot 'saas-admin-mfa.key'
        }
        if ([string]::IsNullOrWhiteSpace($env:MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_FILE)) {
            $env:MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_FILE = Join-Path $secretRoot 'dashboard-mfa.key'
        }
        if ([string]::IsNullOrWhiteSpace($env:MOCHAT_SAAS_ADMIN_JWT_SECRET)) { $env:MOCHAT_SAAS_ADMIN_JWT_SECRET = 'dry-run-saas-jwt-secret' }
        if ([string]::IsNullOrWhiteSpace($env:MOCHAT_DASHBOARD_JWT_SECRET)) { $env:MOCHAT_DASHBOARD_JWT_SECRET = 'dry-run-dashboard-jwt-secret' }
        if ([string]::IsNullOrWhiteSpace($env:MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY)) { $env:MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY = 'dry-run-wecom-key' }
        if ([string]::IsNullOrWhiteSpace($env:MOCHAT_ARCHIVE_BRIDGE_BEARER)) { $env:MOCHAT_ARCHIVE_BRIDGE_BEARER = 'dry-run-archive-bridge-bearer' }
        if ([string]::IsNullOrWhiteSpace($env:MOCHAT_ARCHIVE_FIXTURE_ADMIN_BEARER)) { $env:MOCHAT_ARCHIVE_FIXTURE_ADMIN_BEARER = 'dry-run-archive-fixture-admin-bearer' }
        if ([string]::IsNullOrWhiteSpace($env:MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY)) { $env:MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY = ('0a' * 32) }
    } else {
        if ([string]::IsNullOrWhiteSpace($env:MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE) -or -not (Test-Path -LiteralPath $env:MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE -PathType Leaf)) {
            $env:MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE = Get-OrCreateSecretFile -Directory $secretRoot -Name 'saas-admin-mfa.key'
        }
        if ([string]::IsNullOrWhiteSpace($env:MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_FILE) -or -not (Test-Path -LiteralPath $env:MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_FILE -PathType Leaf)) {
            $env:MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_FILE = Get-OrCreateSecretFile -Directory $secretRoot -Name 'dashboard-mfa.key'
        }
        if ([string]::IsNullOrWhiteSpace($env:MOCHAT_SAAS_ADMIN_JWT_SECRET)) {
            $jwtPath = Get-OrCreateSecretFile -Directory $secretRoot -Name 'saas-admin-jwt.key'
            $env:MOCHAT_SAAS_ADMIN_JWT_SECRET = (Get-Content -LiteralPath $jwtPath -Raw).Trim()
        }
        if ([string]::IsNullOrWhiteSpace($env:MOCHAT_DASHBOARD_JWT_SECRET)) {
            $jwtPath = Get-OrCreateSecretFile -Directory $secretRoot -Name 'dashboard-jwt.key'
            $env:MOCHAT_DASHBOARD_JWT_SECRET = (Get-Content -LiteralPath $jwtPath -Raw).Trim()
        }
        if ([string]::IsNullOrWhiteSpace($env:MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY)) {
            $keyPath = Get-OrCreateSecretFile -Directory $secretRoot -Name 'wecom-credentials.key'
            $env:MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY = (Get-Content -LiteralPath $keyPath -Raw).Trim()
        }
        if ([string]::IsNullOrWhiteSpace($env:MOCHAT_ARCHIVE_BRIDGE_BEARER)) {
            $keyPath = Get-OrCreateSecretFile -Directory $secretRoot -Name 'archive-bridge-bearer.key'
            $env:MOCHAT_ARCHIVE_BRIDGE_BEARER = (Get-Content -LiteralPath $keyPath -Raw).Trim()
        }
        if ([string]::IsNullOrWhiteSpace($env:MOCHAT_ARCHIVE_FIXTURE_ADMIN_BEARER)) {
            $keyPath = Get-OrCreateSecretFile -Directory $secretRoot -Name 'archive-fixture-admin-bearer.key'
            $env:MOCHAT_ARCHIVE_FIXTURE_ADMIN_BEARER = (Get-Content -LiteralPath $keyPath -Raw).Trim()
        }
		if ([string]::IsNullOrWhiteSpace($env:MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY)) {
			$keyPath = Get-OrCreateSecretFile -Directory $secretRoot -Name 'saas-backup-encryption.key'
			Protect-LocalSecretFile -Path $keyPath
			$env:MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY = (Get-Content -LiteralPath $keyPath -Raw).Trim()
		}
    }
    if ($env:MOCHAT_SAAS_ADMIN_JWT_SECRET -eq $env:MOCHAT_DASHBOARD_JWT_SECRET) {
        throw 'SaaS 与 Dashboard 必须使用不同的 JWT 密钥'
    }
    if ([string]::IsNullOrWhiteSpace($env:MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY)) {
        throw 'SaaS 备份加密密钥为空；受控迁移维护已阻断'
    }
    if ([string]::IsNullOrWhiteSpace($env:MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_ID)) { $env:MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_ID = 'local-saas-mfa-primary' }
    if ([string]::IsNullOrWhiteSpace($env:MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_ID)) { $env:MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_ID = 'local-dashboard-mfa-primary' }
    $env:MOCHAT_GO_ENABLE_DURABLE_WORK_MESSAGE_ARCHIVE = '1'
    $env:MOCHAT_ARCHIVE_FIXTURE_ENABLED = if ($EnableArchiveFixture) { 'true' } else { 'false' }
    if ($EnableArchiveFixture) {
        $env:MOCHAT_GO_ENABLE_WECOM_SUITE_CALLBACK = 'true'
        $env:MOCHAT_GO_WECOM_SUITE_ID = 'ww-local-fixture-suite'
        $env:MOCHAT_GO_WECOM_SUITE_SECRET = 'local-fixture-suite-secret'
        $env:MOCHAT_GO_WECOM_SUITE_CALLBACK_TOKEN = 'local-fixture-callback-token'
        $env:MOCHAT_GO_WECOM_SUITE_ENCODING_AES_KEY = 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG'
    } elseif ([string]::IsNullOrWhiteSpace($env:MOCHAT_GO_ENABLE_WECOM_SUITE_CALLBACK)) {
        $env:MOCHAT_GO_ENABLE_WECOM_SUITE_CALLBACK = 'false'
    }
}

function Format-Command {
    param(
        [string[]]$Arguments,
        [string[]]$Secrets = @()
    )

	$displayArguments = foreach ($argument in $Arguments) {
		if ($argument -match '[\s;`"$<>|&]') {
			"'" + $argument.Replace("'", "''") + "'"
		} else {
			$argument
		}
	}
	$rendered = 'docker ' + ($displayArguments -join ' ')
    foreach ($secret in $Secrets) {
        if (-not [string]::IsNullOrEmpty($secret)) {
            $rendered = $rendered.Replace($secret, '<已隐藏>')
        }
    }
    return $rendered
}

function Invoke-Docker {
    param(
        [string[]]$Arguments,
        [switch]$Capture,
        [string[]]$Secrets = @()
    )

    Write-Host ('> ' + (Format-Command -Arguments $Arguments -Secrets $Secrets))
    if ($DryRun) {
        return ''
    }

    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        if ($Capture) {
            $stderrPath = [System.IO.Path]::GetTempFileName()
            try {
                # Keep progress/warnings on stderr out of machine-readable stdout.
                $output = & $DockerCommand @Arguments 2> $stderrPath
                $stderr = Get-Content -LiteralPath $stderrPath -Raw -ErrorAction SilentlyContinue
            } finally {
                Remove-Item -LiteralPath $stderrPath -Force -ErrorAction SilentlyContinue
            }
        } else {
            & $DockerCommand @Arguments
            $output = $null
            $stderr = ''
        }
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousPreference
    }
    if ($exitCode -ne 0) {
        $details = (($output | Out-String) + [string]$stderr).Trim()
        throw "Docker 命令执行失败（退出码 $exitCode）：`n$details"
    }
    if ($Capture) {
        $capturedText = [string]($output | Out-String).Trim()
        return $capturedText
    }
    return ''
}

function Invoke-Compose {
    param(
        [string[]]$Arguments,
        [switch]$Capture,
        [string[]]$Secrets = @()
    )

    $result = Invoke-Docker -Arguments ($composeArguments + $Arguments) -Capture:$Capture -Secrets $Secrets
    return $result
}

function Test-CapturedScalar {
    param([string]$Output, [string]$Expected)
    return (($Output -split "`r?`n" | ForEach-Object { $_.Trim() } | Where-Object { $_ }) -contains $Expected)
}

function Get-CapturedContainerId {
    param([string]$Output)
    $normalized = [string]$Output
    $candidates = $normalized -split "`r?`n" | ForEach-Object { $_.Trim() } | Where-Object { $_ -match '^[0-9a-f]{12,64}$' }
    if ($candidates) { return $candidates[-1] }
    return ''
}

function Show-Diagnostics {
    if ($DryRun) {
        return
    }

    Write-Host '正在输出容器状态和最近日志……' -ForegroundColor Yellow
    try { Invoke-Compose -Arguments @('ps') } catch { Write-Warning $_ }
    foreach ($service in @('app', 'mysql', 'redis')) {
        try {
            Invoke-Compose -Arguments @('logs', '--tail', '80', $service)
        } catch {
            Write-Warning "无法读取 $service 日志：$_"
        }
    }
}

function Wait-ComposeService {
    param(
        [string]$Service,
        [int]$TimeoutSeconds
    )

    if ($DryRun) {
        Write-Host "[预览] 等待服务健康：$Service"
        return
    }

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $containerName = "$ProjectName-$Service-1"
    while ((Get-Date) -lt $deadline) {
        try {
            $stateOutput = Invoke-Docker -Arguments @(
                'inspect',
                '--format', '{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{end}}',
                $containerName
            ) -Capture

            $state = ($stateOutput -split "`r?`n" | ForEach-Object { $_.Trim() } | Where-Object { $_ -match '^(running|created|exited|dead)\|' } | Select-Object -Last 1)
            if ([string]::IsNullOrWhiteSpace($state)) { $state = $stateOutput.Trim() }

            $parts = $state -split '\|', 2
            $containerState = $parts[0]
            $healthState = if ($parts.Length -gt 1) { $parts[1] } else { '' }
            if ($containerState -eq 'running' -and ($healthState -eq 'healthy' -or $healthState -eq '')) {
                Write-Host "服务已就绪：$Service" -ForegroundColor Green
                return
            }
            if ($containerState -in @('exited', 'dead') -or $healthState -eq 'unhealthy') {
                throw "服务 $Service 状态异常：$state"
            }
        } catch {
            # Container may not exist during the first compose startup polls.
            if ($_.Exception.Message -like '服务 *状态异常*') { throw }
        }
        Start-Sleep -Seconds 2
    }
    throw "等待服务 $Service 就绪超时（$TimeoutSeconds 秒）"
}

function Wait-HttpEndpoint {
    param(
        [string]$Name,
        [string]$Url,
        [int]$TimeoutSeconds = 90
    )

    if ($DryRun -or $SkipHttpCheck) {
        Write-Host "[跳过 HTTP 检查] $Name：$Url"
        return
    }

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $lastError = ''
    while ((Get-Date) -lt $deadline) {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 10
            if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 400) {
                Write-Host "访问检查通过：$Name" -ForegroundColor Green
                return
            }
            $lastError = "HTTP $($response.StatusCode)"
        } catch {
            $lastError = $_.Exception.Message
        }
        Start-Sleep -Seconds 2
    }
    throw "$Name 访问检查超时：$Url；最后错误：$lastError"
}

function Test-MigrationLedgerExists {
    Write-Host '迁移策略：仅在迁移账本不存在时执行 baseline；已有数据库直接执行增量迁移。'
    if ($DryRun) {
        Write-Host '[预览] 将在运行时检查 mochat_go_schema_migrations。'
        return $false
    }

    $database = if ([string]::IsNullOrWhiteSpace($env:MOCHAT_MYSQL_DATABASE)) { 'mochat' } else { $env:MOCHAT_MYSQL_DATABASE }
    $user = if ([string]::IsNullOrWhiteSpace($env:MOCHAT_MYSQL_USER)) { 'mochat' } else { $env:MOCHAT_MYSQL_USER }
    $password = if ([string]::IsNullOrWhiteSpace($env:MOCHAT_MYSQL_PASSWORD)) { 'mochat_pass' } else { $env:MOCHAT_MYSQL_PASSWORD }
    $query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = '$database' AND table_name = 'mochat_go_schema_migrations'"
    $output = Invoke-Compose -Arguments @(
        'exec', '-T', 'mysql',
        'mariadb', '--batch', '--skip-column-names',
        "-u$user", "-p$password", "--database=$database", '-e', $query
    ) -Capture -Secrets @($password)
    if (-not (Test-CapturedScalar -Output $output -Expected '1')) {
        return $false
    }

    $rowQuery = 'SELECT IF(COUNT(*) > 0, 1, 0) FROM mochat_go_schema_migrations'
    $rowOutput = Invoke-Compose -Arguments @(
        'exec', '-T', 'mysql',
        'mariadb', '--batch', '--skip-column-names',
        "-u$user", "-p$password", "--database=$database", '-e', $rowQuery
    ) -Capture -Secrets @($password)
    return (Test-CapturedScalar -Output $rowOutput -Expected '1')
}

function Invoke-AutomaticMigrations {
    try {
        if ($DryRun -and $DryRunMigrationState -eq 'controlled_pending') {
            throw 'MIGRATION_CONTROLLED_PENDING 0130_identity_realms_single_corp_backfill'
        }
        Invoke-Compose -Arguments @(
            'exec', '-T', 'app',
            'mochat-migrate', '-action', 'up', '-project-root', '/app'
        ) -Capture
    } catch {
        $message = $_.Exception.Message
        if ($message -notmatch 'MIGRATION_CONTROLLED_PENDING[\s\t]+(0130_identity_realms_single_corp_backfill|0131_identity_realms_single_corp_cutover)') {
            throw
        }
        Stop-AtControlledMigrationCheckpoint -Version $Matches[1]
    }
}

function Wait-ComposeServiceStopped {
    param(
        [string]$Service,
        [int]$TimeoutSeconds = 60
    )

    if ($DryRun) {
        Write-Host "[预览] 已确认服务停止：$Service"
        return
    }

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $containerName = "$ProjectName-$Service-1"
    while ((Get-Date) -lt $deadline) {
        $stateOutput = Invoke-Docker -Arguments @(
            'inspect', '--format', '{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{end}}', $containerName
        ) -Capture
        $state = ($stateOutput -split "`r?`n" | ForEach-Object { $_.Trim() } | Where-Object { $_ -match '^(running|created|exited|dead)\|' } | Select-Object -Last 1)
        if ($state -match '^(exited|created|dead)\|') {
            Write-Host "已确认服务停止：$Service" -ForegroundColor Green
            return
        }
        Start-Sleep -Seconds 2
    }
    throw "等待服务 $Service 停止超时（$TimeoutSeconds 秒）"
}

function Get-CapturedField {
    param(
        [string]$Output,
        [string]$Name
    )

    foreach ($line in ($Output -split "`r?`n")) {
		$parts = $line.Trim() -split '\s+', 2
        if ($parts.Length -eq 2 -and $parts[0] -eq $Name) {
            return $parts[1].Trim()
        }
    }
    return ''
}

function Stop-AtControlledMigrationCheckpoint {
    param([string]$Version)

    Invoke-Compose -Arguments @('stop', 'app')
    Wait-ComposeServiceStopped -Service 'app'
    Invoke-Compose -Arguments @(
        'run', '--rm', '--no-deps', '--entrypoint', '/bin/sh', 'app', '-c',
        'test -n "$MOCHAT_MYSQL_DSN" && test -n "$MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY"'
    )

    Write-Host ''
    Write-Host "受控迁移维护检查点：$Version 尚未完成；app 已停止，MySQL/Redis 与命名卷保持不变，且不会等待 /readyz。" -ForegroundColor Yellow
    Write-Host '默认流程不会执行受控写入。确认备份/恢复要求及维护确认工件均获审批后，复制并执行：' -ForegroundColor Yellow
    Write-Host ("  & '" + $PSCommandPath + "' -ApproveControlledMigrations -ProjectName '" + $ProjectName + "' -ControlledRequestID '<approved-request-id>' -ControlledMaintenanceConfirmationFile '<approved-maintenance-confirmation-file>'")
    throw "controlled migration $Version requires explicit maintenance authorization"
}

function Invoke-ControlledMigrationResume {
    if ($ResetData) {
        throw '-ApproveControlledMigrations 不能与 -ResetData 同时使用'
    }
    if ([string]::IsNullOrWhiteSpace($ControlledRequestID) -or $ControlledRequestID -eq '<approved-request-id>') {
        throw '显式受控迁移必须提供已审批的 -ControlledRequestID'
    }
    if ([string]::IsNullOrWhiteSpace($ControlledMaintenanceConfirmationFile) -or $ControlledMaintenanceConfirmationFile -eq '<approved-maintenance-confirmation-file>') {
        throw '显式受控迁移必须提供已审批的 -ControlledMaintenanceConfirmationFile'
    }
    if (-not $DryRun -and -not (Test-Path -LiteralPath $ControlledMaintenanceConfirmationFile -PathType Leaf)) {
        throw "维护确认工件不存在：$ControlledMaintenanceConfirmationFile"
    }

    $database = if ([string]::IsNullOrWhiteSpace($env:MOCHAT_MYSQL_DATABASE)) { 'mochat' } else { $env:MOCHAT_MYSQL_DATABASE }
    $platformTenantID = if ([string]::IsNullOrWhiteSpace($env:MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID)) { '1' } else { $env:MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID }
    $credentialKeyID = if ([string]::IsNullOrWhiteSpace($env:MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID)) { 'primary' } else { $env:MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID }
    $maintenanceRoot = '/app/storage/identity-maintenance'

    Write-Host '显式受控迁移维护：先 quiesce app，再以一次性容器验证秘密并执行备份；MySQL/Redis 与命名卷保持运行。' -ForegroundColor Yellow
    Invoke-Compose -Arguments @('stop', 'app')
    Wait-ComposeServiceStopped -Service 'app'
    Invoke-Compose -Arguments @(
        'run', '--rm', '--no-deps', '--entrypoint', '/bin/sh', 'app', '-c',
        'test -n "$MOCHAT_MYSQL_DSN" && test -n "$MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY"'
    )

    $backupOutput = Invoke-Compose -Arguments @(
        'run', '--rm', '--no-deps', '--entrypoint', '/usr/local/bin/mochat-saas-maintenance', 'app', '-action', 'backup-create'
    ) -Capture
    if ($DryRun) {
        $backupRunID = 'dry-run-backup-id'
    } else {
        $backupRunID = Get-CapturedField -Output $backupOutput -Name 'backup_run_id'
        $backupStatus = Get-CapturedField -Output $backupOutput -Name 'status'
        $backupEncrypted = Get-CapturedField -Output $backupOutput -Name 'encrypted'
        if ([string]::IsNullOrWhiteSpace($backupRunID) -or $backupStatus -ne 'succeeded' -or $backupEncrypted -ne 'true') {
            throw '受控迁移备份创建结果未证明成功且已加密；app 保持停止'
        }
    }

    $verifyOutput = Invoke-Compose -Arguments @(
        'run', '--rm', '--no-deps', '--entrypoint', '/usr/local/bin/mochat-saas-maintenance', 'app', '-action', 'backup-verify', '-backup-run-id', $backupRunID
    ) -Capture
    if (-not $DryRun) {
        $verificationStatus = Get-CapturedField -Output $verifyOutput -Name 'verification_status'
        if ($verificationStatus -ne 'passed') {
            throw '受控迁移备份验证未通过；app 保持停止'
        }
    }

    Invoke-Compose -Arguments @(
        'run', '--rm', '--no-deps', '--entrypoint', '/bin/sh', 'app', '-c',
        "umask 077; mkdir -p $maintenanceRoot; printf '%s' `"`$MOCHAT_MYSQL_DSN`" > $maintenanceRoot/mysql.dsn; printf '%s' `"`$MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY`" > $maintenanceRoot/wecom.key"
    )
    Invoke-Compose -Arguments @('cp', $ControlledMaintenanceConfirmationFile, "app:$maintenanceRoot/confirmation.txt")
    Invoke-Compose -Arguments @(
        'run', '--rm', '--no-deps', '--user', 'root', '--entrypoint', '/bin/sh', 'app', '-c',
        "chown mochat:mochat $maintenanceRoot/confirmation.txt; chmod 0400 $maintenanceRoot/confirmation.txt"
    )

    Invoke-Compose -Arguments @(
        'run', '--rm', '--no-deps', '--entrypoint', '/usr/local/bin/mochat-identity-preflight', 'app',
        '--dsn-file', "$maintenanceRoot/mysql.dsn", '--schema', $database, '--platform-tenant-id', $platformTenantID,
        '--credential-key-file', "$maintenanceRoot/wecom.key", '--credential-key-id', $credentialKeyID
    )
    $identityCommon = @(
        '--execute', '--request-id', $ControlledRequestID, '--dsn-file', "$maintenanceRoot/mysql.dsn", '--schema', $database,
        '--platform-tenant-id', $platformTenantID, '--maintenance-confirmation-file', "$maintenanceRoot/confirmation.txt",
        '--credential-key-file', "$maintenanceRoot/wecom.key", '--credential-key-id', $credentialKeyID, '--project-root', '/app'
    )
    Invoke-Compose -Arguments (@('run', '--rm', '--no-deps', '--entrypoint', '/usr/local/bin/mochat-identity-migrate', 'app', 'up') + $identityCommon)
    Invoke-Compose -Arguments (@('run', '--rm', '--no-deps', '--entrypoint', '/usr/local/bin/mochat-identity-migrate', 'app', 'encrypt-credentials') + $identityCommon)
    Invoke-Compose -Arguments (@('run', '--rm', '--no-deps', '--entrypoint', '/usr/local/bin/mochat-identity-migrate', 'app', 'cutover') + $identityCommon)

    Invoke-Compose -Arguments @(
        'run', '--rm', '--no-deps', '--entrypoint', '/usr/local/bin/mochat-migrate', 'app', '-action', 'up', '-project-root', '/app'
    ) -Capture
    Invoke-Compose -Arguments @(
        'run', '--rm', '--no-deps', '--entrypoint', '/bin/sh', 'app', '-c',
        "rm -f $maintenanceRoot/mysql.dsn $maintenanceRoot/wecom.key $maintenanceRoot/confirmation.txt"
    )
    Invoke-Compose -Arguments @('up', '-d', 'app')
    Wait-ComposeService -Service 'app' -TimeoutSeconds 300
    Wait-HttpEndpoint -Name '应用存活状态（受控迁移后）' -Url "http://127.0.0.1:$DashboardPort/healthz"
    Wait-HttpEndpoint -Name '应用就绪状态（受控迁移后）' -Url "http://127.0.0.1:$DashboardPort/readyz"
    Write-Host '受控迁移、automatic up、app 重启及 health/ready 验证已完成。' -ForegroundColor Green
}

if (-not (Test-Path -LiteralPath $composeFile)) {
    throw "找不到 Compose 文件：$composeFile"
}
if ([string]::IsNullOrWhiteSpace($ProjectName) -or $ProjectName -notmatch '^[a-zA-Z0-9][a-zA-Z0-9_-]*$') {
    throw 'ProjectName 只能包含字母、数字、下划线和连字符，并且必须以字母或数字开头'
}

Initialize-LocalIdentityRealmSecrets
$env:MOCHAT_GO_PORT = [string]$DashboardPort
$env:MOCHAT_SIDEBAR_PORT = [string]$SidebarPort
$env:MOCHAT_OPERATION_PORT = [string]$OperationPort
$env:MOCHAT_MYSQL_PORT = [string]$MySQLPort
$env:MOCHAT_REDIS_PORT = [string]$RedisPort
$env:MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD = '1'
$env:MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY = '1'
# 开发环境默认开启企业微信相关 Worker：回调事件、员工同步、客户同步、客户群同步。
if ([string]::IsNullOrWhiteSpace($env:MOCHAT_GO_ENABLE_WEWORK_CALLBACK_WORKER)) {
    $env:MOCHAT_GO_ENABLE_WEWORK_CALLBACK_WORKER = '1'
}
if ([string]::IsNullOrWhiteSpace($env:MOCHAT_GO_ENABLE_EMPLOYEE_APPLY_WORKER)) {
    $env:MOCHAT_GO_ENABLE_EMPLOYEE_APPLY_WORKER = '1'
}
if ([string]::IsNullOrWhiteSpace($env:MOCHAT_GO_ENABLE_WORK_CONTACT_SYNC_WORKER)) {
    $env:MOCHAT_GO_ENABLE_WORK_CONTACT_SYNC_WORKER = '1'
}
if ([string]::IsNullOrWhiteSpace($env:MOCHAT_GO_ENABLE_WORK_ROOM_SYNC_WORKER)) {
    $env:MOCHAT_GO_ENABLE_WORK_ROOM_SYNC_WORKER = '1'
}
if ([string]::IsNullOrWhiteSpace($env:MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY)) {
    $env:MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY = '8d7b6a59483726150f1e2d3c4b5a69788d7b6a59483726150f1e2d3c4b5a6978'
}
if ([string]::IsNullOrWhiteSpace($env:MOCHAT_SIMPLE_JWT_SECRET)) {
    $env:MOCHAT_SIMPLE_JWT_SECRET = 'mochat-go-docker-desktop-local-secret'
}

try {
    Write-Host "MoChat Go Docker Desktop 快速部署" -ForegroundColor Cyan
    Write-Host ("本地会话模拟器：" + $(if ($EnableArchiveFixture) { '已启用' } else { '未启用' }))
    Write-Host "Compose 项目：$ProjectName"
    Write-Host "基础设施端口：MySQL：$MySQLPort；Redis：$RedisPort"

    if (-not $DryRun) {
        if ($null -eq (Get-Command $DockerCommand -ErrorAction SilentlyContinue)) {
            throw '未找到 docker 命令，请先安装并启动 Docker Desktop'
        }
        Invoke-Docker -Arguments @('info') | Out-Null
        Invoke-Docker -Arguments @('compose', 'version') | Out-Null
    }
    Invoke-Compose -Arguments @('config', '--quiet') | Out-Null

    if ($ApproveControlledMigrations) {
        Invoke-ControlledMigrationResume
        return
    }

    if ($ResetData) {
        Write-Host '警告：已指定 -ResetData，将删除当前项目的数据库、Redis、上传文件、备份和审计锚点数据卷。' -ForegroundColor Red
        Invoke-Compose -Arguments @('down', '--volumes', '--remove-orphans')
    } else {
        Write-Host '默认安全模式：替换容器，但保留所有项目数据卷。'
        Invoke-Compose -Arguments @('down', '--remove-orphans')
    }

    Invoke-Compose -Arguments @('up', '-d', '--build', '--force-recreate', '--remove-orphans')

    Wait-ComposeService -Service 'mysql' -TimeoutSeconds 180
    Wait-ComposeService -Service 'redis' -TimeoutSeconds 180
    Wait-ComposeService -Service 'app' -TimeoutSeconds 300

    Wait-HttpEndpoint -Name '应用存活状态（迁移前）' -Url "http://127.0.0.1:$DashboardPort/healthz"

    if (Test-MigrationLedgerExists) {
        Write-Host '检测到迁移账本，跳过 baseline。'
    } else {
        try {
            Invoke-Compose -Arguments @(
                'exec', '-T', 'app',
                'mochat-migrate', '-action', 'baseline', '-project-root', '/app'
            ) -Capture
        } catch {
            if ($_.Exception.Message -notmatch 'baseline requires complete 0129 schema') {
                throw
            }
            Write-Host '检测到 Compose fresh schema：以 0104 为实际 init 边界，禁止虚假 baseline，随后按真实 DDL 顺序补齐至 0129。' -ForegroundColor Yellow
            Invoke-Compose -Arguments @(
                'exec', '-T', 'app',
                'mochat-migrate', '-action', 'baseline-compose-init', '-project-root', '/app'
            ) -Capture
        }
    }
    Invoke-AutomaticMigrations
    Write-Host '空业务库安全顺序：先显式 bootstrap SaaS Admin，再通过受保护 SaaS API 完成平台/企业事实；只读 preflight 通过后执行 0130 backfill，随后 encrypt-credentials，最后执行 0131 cutover。脚本不会自动执行 controlled migration。' -ForegroundColor Yellow
    Write-Host 'SaaS 管理员初始化未自动执行；请按 deploy/standalone/README.md 的一次性 PasswordFile + RequestKey 流程显式执行。' -ForegroundColor Yellow

    $dashboardUrl = "http://127.0.0.1:$DashboardPort/"
    $saasAdminUrl = "http://127.0.0.1:$DashboardPort/saas-admin/"
    $sidebarUrl = "http://127.0.0.1:$SidebarPort/"
    $operationUrl = "http://127.0.0.1:$OperationPort/"

    Wait-HttpEndpoint -Name '应用就绪状态' -Url "http://127.0.0.1:$DashboardPort/readyz"
    Wait-HttpEndpoint -Name 'Dashboard' -Url $dashboardUrl
    Wait-HttpEndpoint -Name 'SaaS Admin' -Url $saasAdminUrl
    Wait-HttpEndpoint -Name 'SaaS 身份登录' -Url "http://127.0.0.1:$DashboardPort/saas/login"
    Wait-HttpEndpoint -Name 'Sidebar' -Url $sidebarUrl
    Wait-HttpEndpoint -Name 'Operation' -Url $operationUrl

    Write-Host ''
    Write-Host '部署完成。' -ForegroundColor Green
    Write-Host "Dashboard：$dashboardUrl"
    Write-Host "SaaS Admin：$saasAdminUrl"
    Write-Host "Sidebar：$sidebarUrl"
    Write-Host "Operation：$operationUrl"
} catch {
    Write-Host ''
    Write-Host "部署失败：$($_.Exception.Message)" -ForegroundColor Red
    Show-Diagnostics
    exit 1
}
