$ErrorActionPreference = 'Stop'
$script = Join-Path $PSScriptRoot 'phase35_acceptance_data.ps1'

function Assert-Fails([scriptblock]$Operation, [string]$Pattern) {
    try { & $Operation; throw 'expected command to fail' }
    catch { if ($_.Exception.Message -notmatch $Pattern) { throw } }
}

Assert-Fails { & $script -Action verify -BaseUrl 'http://127.0.0.1:1' -EnvironmentId 'P35-ACCEPT-TEST' -CorpId 1 } 'AllowIsolatedEnvironment'
Assert-Fails { & $script -Action verify -BaseUrl 'http://127.0.0.1:1' -EnvironmentId 'NOT-SAFE' -CorpId 1 -AllowIsolatedEnvironment } 'ValidatePattern|P35-ACCEPT'

$source = Get-Content -Raw $script
foreach ($required in @("'create'", "'verify'", "'cleanup'", 'Method Post', 'Method Get', 'Method Delete', 'X-Phase35-Acceptance-Environment')) {
    if (-not $source.Contains($required)) { throw "missing acceptance workflow: $required" }
}
Write-Host 'phase35 acceptance data guard tests passed'
