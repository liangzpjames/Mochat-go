param([string]$BaseUrl = 'http://139.196.34.133:19090')
$ErrorActionPreference = 'Stop'
$health = Invoke-RestMethod -Uri ($BaseUrl.TrimEnd('/') + '/healthz') -TimeoutSec 10
if ($health.status -ne 'ok') { throw 'public health check failed' }
$health | ConvertTo-Json -Depth 5
