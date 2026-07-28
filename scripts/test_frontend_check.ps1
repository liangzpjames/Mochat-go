$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Split-Path -Parent $PSScriptRoot
$scriptPath = Join-Path $PSScriptRoot "frontend_check.ps1"
$powershell = (Get-Process -Id $PID).Path

function Invoke-ExpectedFailure {
    param(
        [string]$Name,
        [string[]]$Arguments,
        [hashtable]$Environment = @{}
    )
    $saved = @{}
    try {
        foreach ($key in $Environment.Keys) {
            $saved[$key] = [Environment]::GetEnvironmentVariable($key)
            [Environment]::SetEnvironmentVariable($key, $Environment[$key])
        }
        $previousPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        & $powershell -NoProfile -ExecutionPolicy Bypass -File $scriptPath @Arguments *> $null
        $ErrorActionPreference = $previousPreference
        if ($LASTEXITCODE -eq 0) {
            throw "$Name unexpectedly succeeded"
        }
    } finally {
        foreach ($key in $Environment.Keys) {
            [Environment]::SetEnvironmentVariable($key, $saved[$key])
        }
    }
}

Set-Location $repoRoot
Invoke-ExpectedFailure "missing mode" @()
Invoke-ExpectedFailure "invalid mode" @("invalid")
Invoke-ExpectedFailure "old Node" @("quick") @{ MOCHAT_TEST_NODE_VERSION = "20.0.0" }
Invoke-ExpectedFailure "future Node" @("quick") @{ MOCHAT_TEST_NODE_VERSION = "25.0.0" }
Invoke-ExpectedFailure "wrong pnpm" @("quick") @{ MOCHAT_TEST_PNPM_VERSION = "1.0.0" }

$mockDirectory = Join-Path ([System.IO.Path]::GetTempPath()) ("mochat-frontend-check-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $mockDirectory | Out-Null
$mockCorepack = Join-Path $mockDirectory "corepack.cmd"
Set-Content -LiteralPath $mockCorepack -Encoding Ascii -Value @'
@echo off
if "%3"=="--version" (
  echo 11.17.0
  exit /b 0
)
echo %* | findstr /C:"%MOCHAT_MOCK_FAIL_ON%" >nul
if not errorlevel 1 exit /b 17
exit /b 0
'@
try {
    $common = @{
        MOCHAT_COREPACK_COMMAND = $mockCorepack
        MOCHAT_TEST_NODE_VERSION = "22.12.0"
        MOCHAT_TEST_PNPM_VERSION = "11.17.0"
    }
    Invoke-ExpectedFailure "lockfile drift" @("build") ($common + @{ MOCHAT_MOCK_FAIL_ON = "install --frozen-lockfile" })
    Invoke-ExpectedFailure "subcommand failure propagation" @("quick") ($common + @{ MOCHAT_MOCK_FAIL_ON = "check:deps" })
    Invoke-ExpectedFailure "missing app dist" @("build") ($common + @{
        MOCHAT_MOCK_FAIL_ON = "never-match"
        MOCHAT_TEST_DASHBOARD_DIST = (Join-Path $mockDirectory "missing-dashboard.html")
        MOCHAT_TEST_SAAS_ADMIN_DIST = (Join-Path $mockDirectory "missing-saas.html")
    })
} finally {
    Remove-Item -LiteralPath $mockDirectory -Recurse -Force
}

Write-Output "frontend-check-tests: ok"
