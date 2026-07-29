$ErrorActionPreference = "Stop"

Write-Host "Phase 2.1: checking functional matrix"
pnpm check:phase2.1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Phase 2.1: checking shared UI"
pnpm --filter @mochat/ui lint
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
pnpm --filter @mochat/ui typecheck
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
pnpm --filter @mochat/ui test
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

foreach ($app in @("dashboard", "sidebar", "operation")) {
    Write-Host "Phase 2.1: checking $app"
    pnpm --filter "@mochat/$app" typecheck
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    pnpm --filter "@mochat/$app" test
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    pnpm --filter "@mochat/$app" build
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host "Phase 2.1 frontend acceptance passed"
