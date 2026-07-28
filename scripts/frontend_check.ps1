param(
    [Parameter(Position = 0)]
    [ValidateSet("quick", "build", "e2e")]
    [string]$Mode
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if ([string]::IsNullOrWhiteSpace($Mode)) {
    Write-Error "usage: frontend_check.ps1 quick|build|e2e"
}

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot
$corepackCommand = if ($env:MOCHAT_COREPACK_COMMAND) { $env:MOCHAT_COREPACK_COMMAND } else { "corepack" }

function Invoke-FrontendCommand {
    param([string[]]$Arguments)
    & $corepackCommand pnpm @Arguments
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}

function Assert-FrontendToolchain {
    $nodeVersion = if ($env:MOCHAT_TEST_NODE_VERSION) { $env:MOCHAT_TEST_NODE_VERSION } else { (& node --version) }
    $nodeMatch = [regex]::Match($nodeVersion.Trim(), '^v?(\d+)\.(\d+)\.(\d+)$')
    if (-not $nodeMatch.Success) {
        Write-Error "invalid Node version: $nodeVersion"
    }
    $nodeMajor = [int]$nodeMatch.Groups[1].Value
    $nodeMinor = [int]$nodeMatch.Groups[2].Value
    if ($nodeMajor -lt 22 -or ($nodeMajor -eq 22 -and $nodeMinor -lt 12) -or $nodeMajor -ge 25) {
        Write-Error "Node 22.12 through 24.x is required; found $nodeVersion"
    }

    $package = Get-Content -Raw package.json | ConvertFrom-Json
    $expectedPnpm = [string]$package.packageManager -replace '^pnpm@', ''
    $actualPnpm = if ($env:MOCHAT_TEST_PNPM_VERSION) { $env:MOCHAT_TEST_PNPM_VERSION } else { (& $corepackCommand pnpm --version) }
    if ($actualPnpm.Trim() -ne $expectedPnpm) {
        Write-Error "pnpm $expectedPnpm is required; found $actualPnpm"
    }
}

Assert-FrontendToolchain

switch ($Mode) {
    "quick" {
        Invoke-FrontendCommand @("check:audit")
        Invoke-FrontendCommand @("check:deps")
        Invoke-FrontendCommand @("lint")
        Invoke-FrontendCommand @("typecheck")
        Invoke-FrontendCommand @("test")
    }
    "build" {
        Invoke-FrontendCommand @("install", "--frozen-lockfile")
        Invoke-FrontendCommand @("build")
        $dashboardDist = if ($env:MOCHAT_TEST_DASHBOARD_DIST) { $env:MOCHAT_TEST_DASHBOARD_DIST } else { "web/apps/dashboard/dist/index.html" }
        $saasAdminDist = if ($env:MOCHAT_TEST_SAAS_ADMIN_DIST) { $env:MOCHAT_TEST_SAAS_ADMIN_DIST } else { "web/apps/saas-admin/dist/index.html" }
        foreach ($dist in @($dashboardDist, $saasAdminDist)) {
            if (-not (Test-Path -LiteralPath $dist -PathType Leaf)) {
                Write-Error "frontend build output is missing: $dist"
            }
        }
    }
    "e2e" {
        if (
            (-not $env:MOCHAT_PLAYWRIGHT_CHANNEL) -and
            (Test-Path -LiteralPath "C:\Program Files\Google\Chrome\Application\chrome.exe")
        ) {
            $env:MOCHAT_PLAYWRIGHT_CHANNEL = "chrome"
        }
        Invoke-FrontendCommand @("--filter", "@mochat/e2e", "test:e2e")
    }
}
