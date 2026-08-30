$ErrorActionPreference = 'Stop'

$repositoryRoot = Split-Path -Parent $PSScriptRoot
$deployScript = Join-Path $PSScriptRoot 'deploy_docker_desktop.ps1'
$simulatorScript = Join-Path $PSScriptRoot 'run_archive_simulator.ps1'
$dockerIgnore = Join-Path $repositoryRoot '.dockerignore'
$dockerfile = Join-Path $repositoryRoot 'Dockerfile'
$composeFile = Join-Path $repositoryRoot 'deploy\standalone\docker-compose.yml'

if (-not (Test-Path -LiteralPath $deployScript)) {
    throw "部署脚本不存在：$deployScript"
}
if (-not (Test-Path -LiteralPath $dockerIgnore)) {
    throw "Docker 忽略文件不存在：$dockerIgnore"
}
if (-not (Test-Path -LiteralPath $simulatorScript)) {
    throw "模拟器安全入口不存在：$simulatorScript"
}

function Invoke-DeploymentPreview {
    param(
        [switch]$ResetData
    )

    $arguments = @(
        '-NoProfile',
        '-ExecutionPolicy', 'Bypass',
        '-File', $deployScript,
        '-DryRun',
        '-SkipHttpCheck'
    )
    if ($ResetData) {
        $arguments += '-ResetData'
    }

    $output = & powershell @arguments 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0) {
        throw "部署预览失败：`n$output"
    }
    return $output
}

function Invoke-ControlledResumePreview {
    $output = & powershell -NoProfile -ExecutionPolicy Bypass -File $deployScript `
        -DryRun `
        -SkipHttpCheck `
        -ApproveControlledMigrations `
        -ControlledRequestID 'approved-test-request' `
        -ControlledMaintenanceConfirmationFile 'approved-confirmation.txt' 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0) {
        throw "受控迁移恢复预览失败：`n$output"
    }
    return $output
}

function Assert-Matches {
    param(
        [string]$Actual,
        [string]$Pattern,
        [string]$Message
    )

    if ($Actual -notmatch $Pattern) {
        throw "$Message`n实际输出：`n$Actual"
    }
}

$defaultOutput = Invoke-DeploymentPreview
$resetOutput = Invoke-DeploymentPreview -ResetData
$controlledResumeOutput = Invoke-ControlledResumePreview
$previousPreference = $ErrorActionPreference
$ErrorActionPreference = 'Continue'
try {
    $controlledPendingOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $deployScript `
        -DryRun `
        -SkipHttpCheck `
        -DryRunMigrationState controlled_pending 2>&1 | Out-String
} finally {
    $ErrorActionPreference = $previousPreference
}

function Get-FreeTCPPort {
    $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    $listener.Start()
    try {
        return ([System.Net.IPEndPoint]$listener.LocalEndpoint).Port
    } finally {
        $listener.Stop()
    }
}

function Start-FakeHTTPServer {
    param(
        [int]$Port,
        [string]$FailurePath,
        [string]$ReadyFile
    )

    return Start-Job -ArgumentList $Port, $FailurePath, $ReadyFile -ScriptBlock {
        param($Port, $FailurePath, $ReadyFile)
        $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, $Port)
        $listener.Start()
        [System.IO.File]::WriteAllText($ReadyFile, 'ready')
        try {
            $deadline = (Get-Date).AddSeconds(30)
            while ((Get-Date) -lt $deadline) {
                if (-not $listener.Pending()) {
                    Start-Sleep -Milliseconds 20
                    continue
                }
                $client = $listener.AcceptTcpClient()
                try {
                    $stream = $client.GetStream()
                    $reader = [System.IO.StreamReader]::new($stream, [System.Text.Encoding]::ASCII, $false, 1024, $true)
                    $requestLine = $reader.ReadLine()
                    while ($reader.ReadLine()) {}
                    $path = if ($requestLine) { ($requestLine -split ' ')[1] } else { '' }
                    $status = if ($path -eq $FailurePath) { '503 Service Unavailable' } else { '204 No Content' }
                    $payload = [System.Text.Encoding]::ASCII.GetBytes("HTTP/1.1 $status`r`nContent-Length: 0`r`nConnection: close`r`n`r`n")
                    $stream.Write($payload, 0, $payload.Length)
                    $stream.Flush()
                } finally {
                    $client.Dispose()
                }
            }
        } finally {
            $listener.Stop()
        }
    }
}
if ($LASTEXITCODE -eq 0) {
    throw "controlled_pending 预览必须停在维护检查点：`n$controlledPendingOutput"
}
$fixtureOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $deployScript -DryRun -SkipHttpCheck -EnableArchiveFixture 2>&1 | Out-String
if ($LASTEXITCODE -ne 0) {
    throw "模拟器部署预览失败：`n$fixtureOutput"
}
$portOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $deployScript `
    -DryRun `
    -SkipHttpCheck `
    -MySQLPort 23316 `
    -RedisPort 36389 2>&1 | Out-String
if ($LASTEXITCODE -ne 0) {
    throw "数据库端口参数不可用：`n$portOutput"
}

Assert-Matches $defaultOutput '--project-name mochat-go-desktop' '默认项目名不固定'
Assert-Matches $fixtureOutput '本地会话模拟器：已启用' '模拟器部署开关未生效'
Assert-Matches $defaultOutput 'down --remove-orphans' '默认部署未替换旧项目'
Assert-Matches $defaultOutput 'up -d --build --force-recreate --remove-orphans' '缺少强制重建参数'
Assert-Matches $defaultOutput '仅在迁移账本不存在时执行 baseline' '未声明安全的条件基线策略'
Assert-Matches $defaultOutput 'exec -T app mochat-migrate -action up -project-root /app' '未执行数据库迁移'
Assert-Matches $controlledPendingOutput '受控迁移维护检查点' 'controlled_pending 未输出明确维护检查点'
Assert-Matches $controlledPendingOutput 'stop --timeout 70 app' 'controlled_pending 未给 HTTP/worker drain 足够的停止期限'
Assert-Matches $controlledPendingOutput '已确认服务停止：app' 'controlled_pending 未确认 app 已停止'
Assert-Matches $controlledPendingOutput '-ApproveControlledMigrations' '维护检查点缺少可复制的显式恢复命令'
Assert-Matches $controlledResumeOutput 'run --rm --no-deps --entrypoint /usr/local/bin/mochat-saas-maintenance app -action backup-create' '显式恢复缺少容器内备份创建'
Assert-Matches $controlledResumeOutput 'run --rm --no-deps --entrypoint /usr/local/bin/mochat-saas-maintenance app -action backup-verify' '显式恢复缺少容器内备份验证'
Assert-Matches $controlledResumeOutput 'run --rm --no-deps --entrypoint /usr/local/bin/mochat-identity-preflight app' '显式恢复 preflight 未复用 app 容器拓扑'
Assert-Matches $controlledResumeOutput 'run --rm --no-deps --entrypoint /usr/local/bin/mochat-identity-migrate app up' '显式恢复缺少容器内 0130 命令'
Assert-Matches $controlledResumeOutput 'run --rm --no-deps --entrypoint /usr/local/bin/mochat-identity-migrate app cutover' '显式恢复缺少容器内 0131 命令'
Assert-Matches $controlledResumeOutput 'run --rm --no-deps --entrypoint /usr/local/bin/mochat-migrate app -action up -project-root /app' '显式恢复缺少 controlled 完成后的 automatic up'
if ($controlledPendingOutput -match '@tcp\(|mochat_pass|--dsn(?:\s|=)') {
    throw "维护检查点输出泄露 DSN 或密码：`n$controlledPendingOutput"
}
if ($controlledPendingOutput -match 'mochat-saas-maintenance app -action backup-create|mochat-identity-migrate app (?:up|cutover)') {
    throw "未授权 controlled_pending 路径执行或展开了受控写入命令：`n$controlledPendingOutput"
}
$controlledOrder = @(
    'stop --timeout 70 app',
    'mochat-saas-maintenance app -action backup-create',
    'mochat-saas-maintenance app -action backup-verify',
    'mochat-identity-preflight app',
    'mochat-identity-migrate app up',
    'mochat-identity-migrate app encrypt-credentials',
    'mochat-identity-migrate app cutover',
    'mochat-migrate app -action up',
    'up -d app',
    '/healthz',
    '/readyz'
)
$previousControlledIndex = -1
foreach ($step in $controlledOrder) {
    $stepIndex = $controlledResumeOutput.IndexOf($step)
    if ($stepIndex -le $previousControlledIndex) {
        throw "受控维护顺序不完整或错序（$step）：`n$controlledResumeOutput"
    }
    $previousControlledIndex = $stepIndex
}
if ($controlledPendingOutput -match '\[跳过 HTTP 检查\] 应用就绪状态|访问检查通过：应用就绪状态') {
    throw "controlled_pending 后仍进入 ready 等待，会形成部署自锁：`n$controlledPendingOutput"
}
$healthIndex = $defaultOutput.IndexOf('/healthz')
$migrationIndex = $defaultOutput.IndexOf('mochat-migrate -action up')
$readyIndex = $defaultOutput.IndexOf('/readyz')
if ($healthIndex -lt 0 -or $migrationIndex -le $healthIndex -or $readyIndex -le $migrationIndex) {
    throw "已完成 controlled migration 的部署预览顺序必须是 health -> automatic up -> ready：`n$defaultOutput"
}
$deploySource = Get-Content -LiteralPath $deployScript -Raw
$simulatorSource = Get-Content -LiteralPath $simulatorScript -Raw
$dockerIgnoreSource = Get-Content -LiteralPath $dockerIgnore -Raw
$dockerfileSource = Get-Content -LiteralPath $dockerfile -Raw
$composeSource = Get-Content -LiteralPath $composeFile -Raw
Assert-Matches $composeSource '(?ms)^\s{2}app:.*?^\s{4}stop_grace_period:\s*70s\s*$' 'Compose app 未声明至少 70 秒的优雅停止期限'
Assert-Matches $dockerIgnoreSource '(?m)^\.worktrees/\r?$' 'Docker 构建上下文仍包含多 GB 工作树'
Assert-Matches $dockerIgnoreSource '(?m)^\.tmp\*/\r?$' 'Docker 构建上下文仍包含临时 Go 缓存'
Assert-Matches $dockerIgnoreSource '(?m)^web/saas-admin/\r?$' 'Docker 构建上下文仍包含旧的未跟踪 SaaS 构建目录'
Assert-Matches $dockerfileSource '/out/mochat-identity-preflight' 'app 镜像缺少维护检查点所需 preflight binary'
Assert-Matches $dockerfileSource '/out/mochat-identity-migrate' 'app 镜像缺少 controlled migration binary'
Assert-Matches $deploySource 'baseline-compose-init' 'fresh schema 未使用 0104 init 边界恢复后再执行增量迁移'
Assert-Matches $deploySource 'baseline requires complete 0129 schema' '未声明 baseline 的 0129 schema 兼容性边界'
Assert-Matches $deploySource 'IF\(COUNT\(\*\) > 0, 1, 0\)' '空迁移账本未回到专用 compose-init baseline'
Assert-Matches $defaultOutput 'SaaS 管理员初始化未自动执行' '部署入口仍未声明 SaaS-only 初始化边界'
Assert-Matches $deploySource '"--database=\$database"' '迁移/管理员数据库校验未显式指定目标 database'
Assert-Matches $deploySource '12,64' '容器 ID 未执行 12-64 位长度校验'
Assert-Matches $deploySource '2> \$stderrPath' 'Docker Capture 未分离 stderr'
Assert-Matches $deploySource '\.env\.local' '部署入口未加载本地持久环境配置'
Assert-Matches $deploySource 'RandomNumberGenerator' '部署入口未生成持久的本地身份密钥'
Assert-Matches $deploySource 'rev-parse --git-common-dir' '部署入口的密钥目录未绑定到主仓库，worktree 会生成不兼容密钥'
Assert-Matches $simulatorSource 'rev-parse --git-common-dir' '模拟器入口的密钥目录未绑定到主仓库'
Assert-Matches $simulatorSource 'archive-bridge-bearer\.key' '模拟器入口未复用本地 bridge 密钥文件'
Assert-Matches $simulatorSource 'archive-fixture-admin-bearer\.key' '模拟器入口未复用本地管理密钥文件'
Assert-Matches $simulatorSource "'archive-tools'" '模拟器入口未使用隔离的工具 profile'
Assert-Matches $simulatorSource 'ValueFromRemainingArguments' '模拟器入口不能安全透传子命令参数'
Assert-Matches $defaultOutput 'SaaS 身份登录：http://127\.0\.0\.1:18080/saas/login' '未检查 SaaS 身份登录入口'
if ($deploySource -match 'mochat-bootstrap|AdminPassword|AdminPhone') {
    throw '生产部署入口仍包含旧 bootstrap 命令行密码或旧管理员参数'
}
Assert-Matches $resetOutput 'down --volumes --remove-orphans' 'ResetData 未删除项目数据卷'
Assert-Matches $portOutput 'MySQL：23316' '未应用 MySQL 端口参数'
Assert-Matches $portOutput 'Redis：36389' '未应用 Redis 端口参数'

if ($defaultOutput -match 'down[^\r\n]*(--volumes|(?:^|\s)-v(?:\s|$))') {
    throw '默认部署错误地删除了数据卷'
}
if ($defaultOutput -match 'test-password-must-not-be-printed') {
    throw '部署输出泄露了管理员密码'
}

$fakeRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('mochat-docker-test-' + [guid]::NewGuid().ToString('N'))
$fakeDocker = Join-Path $fakeRoot 'docker.cmd'
New-Item -ItemType Directory -Path $fakeRoot | Out-Null
$previousSecretDirectory = $env:MOCHAT_DOCKER_DESKTOP_SECRET_DIR
$previousFakeComposeFile = $env:MOCHAT_TEST_COMPOSE_FILE
$previousBackupEncryptionKey = $env:MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY
$env:MOCHAT_DOCKER_DESKTOP_SECRET_DIR = Join-Path $fakeRoot 'secrets'
$env:MOCHAT_TEST_COMPOSE_FILE = Join-Path $repositoryRoot 'deploy\standalone\docker-compose.yml'
$env:MOCHAT_TEST_STOP_STATE = Join-Path $fakeRoot 'app-stopped'
$env:MOCHAT_TEST_MAINTENANCE_STATE = Join-Path $fakeRoot 'maintenance-files'
Remove-Item Env:MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY -ErrorAction SilentlyContinue
try {
@'
@echo off
setlocal EnableDelayedExpansion
echo docker progress 1>&2
if "%1"=="info" (
  echo identity=%MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY% 1>&2
  if defined MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY echo identity_key=set 1>&2
  if exist "%MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE%" if exist "%MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_FILE%" echo mfa_files=ready 1>&2
  if not "%MOCHAT_SAAS_ADMIN_JWT_SECRET%"=="%MOCHAT_DASHBOARD_JWT_SECRET%" echo realm_jwt=separate 1>&2
  if defined MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY echo backup_key=set 1>&2
  exit /b 0
)
if "%1"=="inspect" (
  if exist "%MOCHAT_TEST_STOP_STATE%" (
    echo exited^|
    exit /b 0
  )
  if "!MOCHAT_TEST_FAIL_STAGE!"=="service-health" (
    echo running^|unhealthy
    exit /b 0
  )
  echo running^|healthy
  exit /b 0
)
echo %* | findstr /C:" stop --timeout 70 app" >nul
if not errorlevel 1 (
  type nul > "%MOCHAT_TEST_STOP_STATE%"
  echo app_stop=done 1>&2
  exit /b 0
)
echo %* | findstr /C:" up -d app" >nul
if not errorlevel 1 (
  if "!MOCHAT_TEST_FAIL_STAGE!"=="start" goto fail_start
  del /q "%MOCHAT_TEST_STOP_STATE%" >nul 2>&1
  echo app_start=done 1>&2
  exit /b 0
)
echo %* | findstr /C:"umask 077" >nul
if not errorlevel 1 (
  type nul > "%MOCHAT_TEST_MAINTENANCE_STATE%"
  echo maintenance_files=created 1>&2
  exit /b 0
)
echo %* | findstr /C:"rm -f /app/storage/identity-maintenance" >nul
if not errorlevel 1 (
  if "!MOCHAT_TEST_FAIL_STAGE!"=="cleanup" goto fail_cleanup
  del /q "%MOCHAT_TEST_MAINTENANCE_STATE%" >nul 2>&1
  echo maintenance_cleanup=done 1>&2
  exit /b 0
)
echo %* | findstr /C:"/bin/sh app -c" | findstr /C:"MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY" >nul
if not errorlevel 1 (
  findstr /C:"MOCHAT_MYSQL_DSN:" "%MOCHAT_TEST_COMPOSE_FILE%" >nul
  if errorlevel 1 exit /b 42
  findstr /C:"MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY:" "%MOCHAT_TEST_COMPOSE_FILE%" >nul
  if errorlevel 1 exit /b 43
  if not defined MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY exit /b 44
  echo controlled_secrets=ready 1>&2
  exit /b 0
)
echo %* | findstr /C:"mochat-saas-maintenance app -action backup-create" >nul
if not errorlevel 1 (
  echo backup_run_id 77
  echo status succeeded
  echo encrypted true
  exit /b 0
)
echo %* | findstr /C:"mochat-saas-maintenance app -action backup-verify" >nul
if not errorlevel 1 (
  echo verification_status passed
  exit /b 0
)
echo %* | findstr /C:"mochat-identity-preflight app" >nul
if not errorlevel 1 (
  if "!MOCHAT_TEST_FAIL_STAGE!"=="preflight" (
    goto fail_preflight
  )
  exit /b 0
)
echo %* | findstr /C:"mochat-identity-migrate app up" >nul
if not errorlevel 1 (
  if "!MOCHAT_TEST_FAIL_STAGE!"=="identity-up" goto fail_identity_up
  exit /b 0
)
echo %* | findstr /C:"mochat-identity-migrate app cutover" >nul
if not errorlevel 1 (
  if "!MOCHAT_TEST_FAIL_STAGE!"=="cutover" goto fail_cutover
  exit /b 0
)
echo %* | findstr /C:"/usr/local/bin/mochat-migrate app -action up" >nul
if not errorlevel 1 (
  if "!MOCHAT_TEST_FAIL_STAGE!"=="automatic-up" goto fail_automatic_up
  exit /b 0
)
if "%MOCHAT_TEST_CONTROLLED_PENDING%"=="1" (
  echo %* | findstr /C:"mochat-migrate -action up" >nul
  if not errorlevel 1 (
    echo MIGRATION_CONTROLLED_PENDING 0130_identity_realms_single_corp_backfill 1>&2
    exit /b 1
  )
)
if "%9"=="-q" (
  echo 1234567890ab
  exit /b 0
)
echo 1
exit /b 0
:fail_start
exit /b 61
:fail_preflight
exit /b 62
:fail_identity_up
exit /b 63
:fail_cutover
exit /b 64
:fail_automatic_up
exit /b 65
:fail_cleanup
exit /b 69
'@ | Set-Content -LiteralPath $fakeDocker -Encoding Ascii

    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $fakeOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $deployScript `
            -DockerCommand $fakeDocker `
            -SkipHttpCheck 2>&1 | Out-String
    } finally {
        $ErrorActionPreference = $previousPreference
    }
    if ($LASTEXITCODE -ne 0) {
        throw "Docker 向标准错误输出正常进度时，部署被错误中止：`n$fakeOutput"
    }
    Assert-Matches $fakeOutput '部署完成' '正常的 Docker 标准错误进度导致部署失败'
    Assert-Matches $fakeOutput 'identity=1' '启用 SaaS Admin 时未启用身份安全登录'
    Assert-Matches $fakeOutput 'identity_key=set' '启用身份安全登录时未配置本地加密密钥'
    Assert-Matches $fakeOutput 'mfa_files=ready' '本地部署未提供两个可读取的 MFA 密钥文件'
    Assert-Matches $fakeOutput 'realm_jwt=separate' 'SaaS 与 Dashboard 仍共享同一个 JWT 密钥'
    Assert-Matches $fakeOutput 'backup_key=set' '本地部署未生成持久备份加密密钥'
    $backupKeyFile = Join-Path $env:MOCHAT_DOCKER_DESKTOP_SECRET_DIR 'saas-backup-encryption.key'
    if (-not (Test-Path -LiteralPath $backupKeyFile -PathType Leaf) -or ((Get-Content -LiteralPath $backupKeyFile -Raw).Trim() -notmatch '^[a-f0-9]{64}$')) {
        throw '本地部署未在受控 secret 目录持久生成高熵备份加密密钥'
    }
    $backupKeyValue = (Get-Content -LiteralPath $backupKeyFile -Raw).Trim()
    if ($fakeOutput -match 'mochat-migrate -action baseline') {
        throw '已有迁移账本时仍执行 baseline，会跳过新的增量迁移'
    }

    $env:MOCHAT_TEST_CONTROLLED_PENDING = '1'
    $ErrorActionPreference = 'Continue'
    try {
        $fakeControlledOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $deployScript `
            -DockerCommand $fakeDocker `
            -SkipHttpCheck 2>&1 | Out-String
        $fakeControlledExitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousPreference
        Remove-Item Env:MOCHAT_TEST_CONTROLLED_PENDING -ErrorAction SilentlyContinue
    }
    if ($fakeControlledExitCode -eq 0) {
        throw "真实 Docker controlled pending 必须非零退出：`n$fakeControlledOutput"
    }
    Assert-Matches $fakeControlledOutput '受控迁移维护检查点' '真实 Docker stable code 未进入维护检查点'
    Assert-Matches $fakeControlledOutput 'controlled_secrets=ready' '受控维护未实际验证 app 容器收到非空 DSN 与备份密钥'
    Assert-Matches $fakeControlledOutput 'app_stop=done' '受控维护检查点未实际停止 app'
    if ($fakeControlledOutput -match '\[跳过 HTTP 检查\] 应用就绪状态|访问检查通过：应用就绪状态') {
        throw "真实 Docker controlled pending 后仍进入 ready 等待：`n$fakeControlledOutput"
    }

    $confirmationFile = Join-Path $fakeRoot 'approved-confirmation.txt'
    Set-Content -LiteralPath $confirmationFile -Value 'approved test artifact' -Encoding Ascii
    $ErrorActionPreference = 'Continue'
    try {
        $fakeResumeOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $deployScript `
            -DockerCommand $fakeDocker `
            -SkipHttpCheck `
            -ApproveControlledMigrations `
            -ControlledRequestID 'approved-test-request' `
            -ControlledMaintenanceConfirmationFile $confirmationFile 2>&1 | Out-String
        $fakeResumeExitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousPreference
    }
    if ($fakeResumeExitCode -ne 0) {
        throw "显式受控迁移 fake Docker 链失败：`n$fakeResumeOutput"
    }
    Assert-Matches $fakeResumeOutput 'controlled_secrets=ready' '显式恢复的一次性容器未收到非空 DSN 与备份加密密钥'
    if (($fakeOutput + $fakeControlledOutput + $fakeResumeOutput) -match [regex]::Escape($backupKeyValue)) {
        throw '部署、检查点或显式恢复输出泄露了备份加密密钥值'
    }
    $fakeControlledOrder = @(
        'stop --timeout 70 app',
        'mochat-saas-maintenance app -action backup-create',
        'mochat-saas-maintenance app -action backup-verify',
        'mochat-identity-preflight app',
        'mochat-identity-migrate app up',
        'mochat-identity-migrate app encrypt-credentials',
        'mochat-identity-migrate app cutover',
        'mochat-migrate app -action up',
        'maintenance_cleanup=done',
        'up -d app',
        '/healthz',
        '/readyz'
    )
    $previousFakeIndex = -1
    foreach ($step in $fakeControlledOrder) {
        $stepIndex = $fakeResumeOutput.IndexOf($step)
        if ($stepIndex -le $previousFakeIndex) {
            throw "fake Docker 受控维护链不完整或错序（$step）：`n$fakeResumeOutput"
        }
        $previousFakeIndex = $stepIndex
    }
    if (Test-Path -LiteralPath $env:MOCHAT_TEST_MAINTENANCE_STATE) {
        throw '显式受控迁移成功后命名卷仍残留 maintenance 临时秘密文件'
    }

    foreach ($failureStage in @('preflight', 'identity-up', 'cutover', 'automatic-up', 'start', 'service-health')) {
        Remove-Item -LiteralPath $env:MOCHAT_TEST_STOP_STATE -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $env:MOCHAT_TEST_MAINTENANCE_STATE -Force -ErrorAction SilentlyContinue
        $env:MOCHAT_TEST_FAIL_STAGE = $failureStage
        $ErrorActionPreference = 'Continue'
        try {
            $failureOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $deployScript `
                -DockerCommand $fakeDocker `
                -SkipHttpCheck `
                -ApproveControlledMigrations `
                -ControlledRequestID 'approved-test-request' `
                -ControlledMaintenanceConfirmationFile $confirmationFile 2>&1 | Out-String
            $failureExitCode = $LASTEXITCODE
        } finally {
            $ErrorActionPreference = $previousPreference
            Remove-Item Env:MOCHAT_TEST_FAIL_STAGE -ErrorAction SilentlyContinue
        }
        if ($failureExitCode -eq 0) {
            throw "受控维护故障注入 $failureStage 必须非零退出：`n$failureOutput"
        }
        if (Test-Path -LiteralPath $env:MOCHAT_TEST_MAINTENANCE_STATE) {
            throw "受控维护故障 $failureStage 后命名卷残留 maintenance 临时秘密文件：`n$failureOutput"
        }
        if (-not (Test-Path -LiteralPath $env:MOCHAT_TEST_STOP_STATE)) {
            throw "受控维护故障 $failureStage 后 app 未保持停止：`n$failureOutput"
        }
        if ([regex]::Matches($failureOutput, 'stop --timeout 70 app').Count -lt 2) {
            throw "受控维护故障 $failureStage 后未再次执行 70 秒优雅停服：`n$failureOutput"
        }
        Assert-Matches $failureOutput 'maintenance_cleanup=done' "受控维护故障 $failureStage 后未执行 finally 临时秘密清理"
    }

    Remove-Item -LiteralPath $env:MOCHAT_TEST_STOP_STATE -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $env:MOCHAT_TEST_MAINTENANCE_STATE -Force -ErrorAction SilentlyContinue
    $env:MOCHAT_TEST_FAIL_STAGE = 'cleanup'
    $ErrorActionPreference = 'Continue'
    try {
        $cleanupFailureOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $deployScript `
            -DockerCommand $fakeDocker `
            -SkipHttpCheck `
            -ApproveControlledMigrations `
            -ControlledRequestID 'approved-test-request' `
            -ControlledMaintenanceConfirmationFile $confirmationFile 2>&1 | Out-String
        $cleanupFailureExitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousPreference
        Remove-Item Env:MOCHAT_TEST_FAIL_STAGE -ErrorAction SilentlyContinue
    }
    if ($cleanupFailureExitCode -eq 0 -or -not (Test-Path -LiteralPath $env:MOCHAT_TEST_STOP_STATE)) {
        throw "临时秘密清理失败必须报告并保持 app 停止：`n$cleanupFailureOutput"
    }
    Assert-Matches $cleanupFailureOutput '临时秘密清理失败' '临时秘密清理失败没有进入最终错误报告'

    foreach ($httpFailurePath in @('/healthz', '/readyz')) {
        Remove-Item -LiteralPath $env:MOCHAT_TEST_STOP_STATE -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $env:MOCHAT_TEST_MAINTENANCE_STATE -Force -ErrorAction SilentlyContinue
        $httpPort = Get-FreeTCPPort
        $readyFile = Join-Path $fakeRoot ("http-ready-" + ($httpFailurePath.Trim('/')))
        $httpJob = Start-FakeHTTPServer -Port $httpPort -FailurePath $httpFailurePath -ReadyFile $readyFile
        try {
            $deadline = (Get-Date).AddSeconds(10)
            while (-not (Test-Path -LiteralPath $readyFile) -and (Get-Date) -lt $deadline) { Start-Sleep -Milliseconds 50 }
            if (-not (Test-Path -LiteralPath $readyFile)) { throw "fake HTTP server 未启动：$httpFailurePath" }
            $ErrorActionPreference = 'Continue'
            try {
                $httpFailureOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $deployScript `
                    -DockerCommand $fakeDocker `
                    -DashboardPort $httpPort `
                    -HttpCheckTimeoutSeconds 1 `
                    -ApproveControlledMigrations `
                    -ControlledRequestID 'approved-test-request' `
                    -ControlledMaintenanceConfirmationFile $confirmationFile 2>&1 | Out-String
                $httpFailureExitCode = $LASTEXITCODE
            } finally {
                $ErrorActionPreference = $previousPreference
            }
        } finally {
            Stop-Job -Job $httpJob -ErrorAction SilentlyContinue
            Remove-Job -Job $httpJob -Force -ErrorAction SilentlyContinue
        }
        if ($httpFailureExitCode -eq 0) {
            throw "受控维护 $httpFailurePath 故障必须非零退出：`n$httpFailureOutput"
        }
        if (Test-Path -LiteralPath $env:MOCHAT_TEST_MAINTENANCE_STATE) {
            throw "受控维护 $httpFailurePath 故障后仍残留临时秘密：`n$httpFailureOutput"
        }
        if (-not (Test-Path -LiteralPath $env:MOCHAT_TEST_STOP_STATE) -or [regex]::Matches($httpFailureOutput, 'stop --timeout 70 app').Count -lt 2) {
            throw "受控维护 $httpFailurePath 故障后未再次优雅停服并确认 stopped：`n$httpFailureOutput"
        }
        Assert-Matches $httpFailureOutput 'maintenance_cleanup=done' "受控维护 $httpFailurePath 故障后未清理临时秘密"
    }

    [System.IO.File]::WriteAllText($backupKeyFile, '', [System.Text.Encoding]::ASCII)
    Remove-Item Env:MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY -ErrorAction SilentlyContinue
    $ErrorActionPreference = 'Continue'
    try {
        $emptyBackupKeyOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $deployScript `
            -DockerCommand $fakeDocker `
            -SkipHttpCheck 2>&1 | Out-String
        $emptyBackupKeyExitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousPreference
    }
    if ($emptyBackupKeyExitCode -eq 0) {
        throw "空备份加密密钥文件必须在部署前 fail-fast：`n$emptyBackupKeyOutput"
    }
    Assert-Matches $emptyBackupKeyOutput '本地密钥文件格式无效' '空备份加密密钥未阻断部署'
} finally {
    $env:MOCHAT_DOCKER_DESKTOP_SECRET_DIR = $previousSecretDirectory
	$env:MOCHAT_TEST_COMPOSE_FILE = $previousFakeComposeFile
    $env:MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY = $previousBackupEncryptionKey
    Remove-Item Env:MOCHAT_TEST_STOP_STATE -ErrorAction SilentlyContinue
    Remove-Item Env:MOCHAT_TEST_MAINTENANCE_STATE -ErrorAction SilentlyContinue
    Remove-Item Env:MOCHAT_TEST_FAIL_STAGE -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $fakeRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host 'Docker Desktop 部署脚本测试通过'
