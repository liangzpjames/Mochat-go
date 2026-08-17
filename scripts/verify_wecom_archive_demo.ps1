param([string]$BaseUrl = 'http://139.196.34.133:19090')
$ErrorActionPreference = 'Stop'
$health = Invoke-RestMethod -Uri ($BaseUrl.TrimEnd('/') + '/healthz') -TimeoutSec 10
if ($health.status -ne 'ok') { throw 'public health check failed' }
$adminOnPublic = Invoke-WebRequest -Uri ($BaseUrl.TrimEnd('/') + '/admin/status') -SkipHttpErrorCheck -TimeoutSec 10
if ($adminOnPublic.StatusCode -ne 404) { throw "admin route is exposed on public listener: $($adminOnPublic.StatusCode)" }
$base = [Uri]$BaseUrl
$adminReachable = $false
try {
    $null = Invoke-WebRequest -Uri ("http://{0}:19091/admin/status" -f $base.Host) -SkipHttpErrorCheck -TimeoutSec 3
    $adminReachable = $true
} catch {}
if ($adminReachable) { throw 'admin HTTP endpoint on port 19091 is reachable from the public network' }
$health | ConvertTo-Json -Depth 5
