$ErrorActionPreference = 'Stop'

$repo = Split-Path -Parent $PSScriptRoot
$build = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'build_wecom_archive_demo.ps1')
$deploy = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'deploy_wecom_archive_demo.sh')
$runner = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'run_wecom_archive_demo.sh')
$configure = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'configure_wecom_archive_demo.sh')
$dockerfile = Get-Content -Raw -LiteralPath (Join-Path $repo 'deploy/wecom-archive-demo/Dockerfile')

$requiredBuild = @('--platform linux/amd64', 'docker save', 'go test ./internal/wecomarchivedemo ./cmd/wecom-archive-demo', "`$env:GOOS = 'linux'", "`$env:CGO_ENABLED = '0'", 'sdkcheck')
foreach ($value in $requiredBuild) {
    if (-not $build.Contains($value)) { throw "build script is missing: $value" }
}
if (-not $build.Contains('$LASTEXITCODE')) { throw 'build script must stop when a native command fails' }

$deploymentSurface = $deploy + "`n" + $runner
$requiredDeploy = @('/opt/wecom-archive-demo', 'docker load', '--pull=never', '0.0.0.0:19090:8080', '127.0.0.1:19091:9091', 'ss -lnt')
foreach ($value in $requiredDeploy) {
    if (-not $deploymentSurface.Contains($value)) { throw "deployment scripts are missing: $value" }
}

$forbiddenDeploy = @('docker build', 'system prune', 'volume prune', 'down -v', 'rm -rf /opt', 'rm -rf $')
foreach ($value in $forbiddenDeploy) {
    if ($deploymentSurface.Contains($value)) { throw "deployment scripts contain forbidden operation: $value" }
}

if (-not $configure.Contains('read -r -s')) { throw 'configuration script must hide the archive Secret while typing' }
if (-not $dockerfile.Contains('afa8c017da2994ad2215933f2fcc6042d40d935663ad42d6e1e9d7716652f0d8')) { throw 'Dockerfile must pin the official SDK checksum' }

Write-Output 'wecom archive demo build/deploy contract: PASS'
