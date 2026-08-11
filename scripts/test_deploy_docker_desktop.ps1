$ErrorActionPreference = 'Stop'

$repositoryRoot = Split-Path -Parent $PSScriptRoot
$deployScript = Join-Path $PSScriptRoot 'deploy_docker_desktop.ps1'

if (-not (Test-Path -LiteralPath $deployScript)) {
    throw "部署脚本不存在：$deployScript"
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
$portOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $deployScript `
    -DryRun `
    -SkipHttpCheck `
    -MySQLPort 23316 `
    -RedisPort 36389 2>&1 | Out-String
if ($LASTEXITCODE -ne 0) {
    throw "数据库端口参数不可用：`n$portOutput"
}

Assert-Matches $defaultOutput '--project-name mochat-go-desktop' '默认项目名不固定'
Assert-Matches $defaultOutput 'down --remove-orphans' '默认部署未替换旧项目'
Assert-Matches $defaultOutput 'up -d --build --force-recreate --remove-orphans' '缺少强制重建参数'
Assert-Matches $defaultOutput '仅在迁移账本不存在时执行 baseline' '未声明安全的条件基线策略'
Assert-Matches $defaultOutput 'exec -T app mochat-migrate -action up -project-root /app' '未执行数据库迁移'
Assert-Matches $defaultOutput 'SaaS 管理员初始化未自动执行' '部署入口仍未声明 SaaS-only 初始化边界'
$deploySource = Get-Content -LiteralPath $deployScript -Raw
Assert-Matches $deploySource '"--database=\$database"' '迁移/管理员数据库校验未显式指定目标 database'
Assert-Matches $deploySource '12,64' '容器 ID 未执行 12-64 位长度校验'
Assert-Matches $deploySource '2> \$stderrPath' 'Docker Capture 未分离 stderr'
Assert-Matches $defaultOutput 'SaaS 身份登录：http://127\.0\.0\.1:18080/security/login' '未检查 SaaS 身份登录入口'
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
try {
@'
@echo off
setlocal
echo docker progress 1>&2
if "%1"=="info" (
  echo identity=%MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY% 1>&2
  if defined MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY echo identity_key=set 1>&2
  exit /b 0
)
if "%1"=="inspect" (
  echo running^|healthy
  exit /b 0
)
if "%9"=="-q" (
  echo 1234567890ab
  exit /b 0
)
echo 1
exit /b 0
echo docker progress 1>&2
exit /b 0
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
    if ($fakeOutput -match 'mochat-migrate -action baseline') {
        throw '已有迁移账本时仍执行 baseline，会跳过新的增量迁移'
    }
} finally {
    Remove-Item -LiteralPath $fakeRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host 'Docker Desktop 部署脚本测试通过'
