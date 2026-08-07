[CmdletBinding()]
param(
    [switch]$ResetData,
    [switch]$DryRun,
    [switch]$SkipHttpCheck,
    [string]$ProjectName = 'mochat-go-desktop',
    [ValidateRange(1, 65535)][int]$DashboardPort = 18080,
    [ValidateRange(1, 65535)][int]$SidebarPort = 18081,
    [ValidateRange(1, 65535)][int]$OperationPort = 18082,
    [ValidateRange(1, 65535)][int]$MySQLPort = 13316,
    [ValidateRange(1, 65535)][int]$RedisPort = 26389,
    [string]$AdminPhone = '13800000000',
    [string]$AdminPassword = 'MochatLocal@123',
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

function Format-Command {
    param(
        [string[]]$Arguments,
        [string[]]$Secrets = @()
    )

    $rendered = 'docker ' + ($Arguments -join ' ')
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
    return (Test-CapturedScalar -Output $output -Expected '1')
}

function Test-BootstrapDashboardAccess {
    param([string]$Phone)

    if ($DryRun) {
        Write-Host '[预览] 将验证管理员、租户企业和通讯录员工映射。'
        return
    }

    $database = if ([string]::IsNullOrWhiteSpace($env:MOCHAT_MYSQL_DATABASE)) { 'mochat' } else { $env:MOCHAT_MYSQL_DATABASE }
    $user = if ([string]::IsNullOrWhiteSpace($env:MOCHAT_MYSQL_USER)) { 'mochat' } else { $env:MOCHAT_MYSQL_USER }
    $password = if ([string]::IsNullOrWhiteSpace($env:MOCHAT_MYSQL_PASSWORD)) { 'mochat_pass' } else { $env:MOCHAT_MYSQL_PASSWORD }
    $escapedPhone = $Phone.Replace("'", "''")
    $query = @"
SELECT CASE WHEN EXISTS (
  SELECT 1
  FROM mc_user u
  JOIN mc_corp c ON c.tenant_id = u.tenant_id AND c.deleted_at IS NULL
  JOIN mc_work_employee e ON e.corp_id = c.id AND e.log_user_id = u.id AND e.deleted_at IS NULL
  WHERE u.phone = '$escapedPhone' AND u.tenant_id = 1 AND u.deleted_at IS NULL
) THEN 1 ELSE 0 END
"@
    $output = Invoke-Compose -Arguments @(
        'exec', '-T', 'mysql',
        'mariadb', '--batch', '--skip-column-names',
        "-u$user", "-p$password", "--database=$database", '-e', $query
    ) -Capture -Secrets @($password)
    if (-not (Test-CapturedScalar -Output $output -Expected '1')) {
        throw "管理员 $Phone 未获得 tenant 1 的企业和员工映射"
    }
    Write-Host "管理员企业访问映射通过：$Phone" -ForegroundColor Green
}

if (-not (Test-Path -LiteralPath $composeFile)) {
    throw "找不到 Compose 文件：$composeFile"
}
if ([string]::IsNullOrWhiteSpace($ProjectName) -or $ProjectName -notmatch '^[a-zA-Z0-9][a-zA-Z0-9_-]*$') {
    throw 'ProjectName 只能包含字母、数字、下划线和连字符，并且必须以字母或数字开头'
}
if ([string]::IsNullOrWhiteSpace($AdminPhone) -or [string]::IsNullOrWhiteSpace($AdminPassword)) {
    throw '管理员手机号和密码不能为空'
}

$env:MOCHAT_GO_PORT = [string]$DashboardPort
$env:MOCHAT_SIDEBAR_PORT = [string]$SidebarPort
$env:MOCHAT_OPERATION_PORT = [string]$OperationPort
$env:MOCHAT_MYSQL_PORT = [string]$MySQLPort
$env:MOCHAT_REDIS_PORT = [string]$RedisPort
$env:MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD = '1'
$env:MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY = '1'
if ([string]::IsNullOrWhiteSpace($env:MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY)) {
    $env:MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY = '8d7b6a59483726150f1e2d3c4b5a69788d7b6a59483726150f1e2d3c4b5a6978'
}
if ([string]::IsNullOrWhiteSpace($env:MOCHAT_SIMPLE_JWT_SECRET)) {
    $env:MOCHAT_SIMPLE_JWT_SECRET = 'mochat-go-docker-desktop-local-secret'
}

try {
    Write-Host "MoChat Go Docker Desktop 快速部署" -ForegroundColor Cyan
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

    if (Test-MigrationLedgerExists) {
        Write-Host '检测到迁移账本，跳过 baseline。'
    } else {
        Invoke-Compose -Arguments @(
            'exec', '-T', 'app',
            'mochat-migrate', '-action', 'baseline', '-project-root', '/app'
        )
    }
    Invoke-Compose -Arguments @(
        'exec', '-T', 'app',
        'mochat-migrate', '-action', 'up', '-project-root', '/app'
    )
    Invoke-Compose -Arguments @(
        'exec', '-T', 'app',
        'mochat-bootstrap',
        '-phone', $AdminPhone,
        '-password', $AdminPassword
    ) -Secrets @($AdminPassword)
    Test-BootstrapDashboardAccess -Phone $AdminPhone

    $dashboardUrl = "http://127.0.0.1:$DashboardPort/"
    $saasAdminUrl = "http://127.0.0.1:$DashboardPort/saas-admin/"
    $sidebarUrl = "http://127.0.0.1:$SidebarPort/"
    $operationUrl = "http://127.0.0.1:$OperationPort/"

    Wait-HttpEndpoint -Name '应用就绪状态' -Url "http://127.0.0.1:$DashboardPort/readyz"
    Wait-HttpEndpoint -Name 'Dashboard' -Url $dashboardUrl
    Wait-HttpEndpoint -Name 'SaaS Admin' -Url $saasAdminUrl
    Wait-HttpEndpoint -Name 'SaaS 身份登录' -Url "http://127.0.0.1:$DashboardPort/security/login"
    Wait-HttpEndpoint -Name 'Sidebar' -Url $sidebarUrl
    Wait-HttpEndpoint -Name 'Operation' -Url $operationUrl

    Write-Host ''
    Write-Host '部署完成。' -ForegroundColor Green
    Write-Host "Dashboard：$dashboardUrl"
    Write-Host "SaaS Admin：$saasAdminUrl"
    Write-Host "Sidebar：$sidebarUrl"
    Write-Host "Operation：$operationUrl"
    Write-Host "管理员账号：$AdminPhone"
} catch {
    Write-Host ''
    Write-Host "部署失败：$($_.Exception.Message)" -ForegroundColor Red
    Show-Diagnostics
    exit 1
}
