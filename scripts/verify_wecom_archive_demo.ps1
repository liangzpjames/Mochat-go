param([string]$BaseUrl = 'http://139.196.34.133:19090')
$ErrorActionPreference = 'Stop'
$health = Invoke-RestMethod -Uri ($BaseUrl.TrimEnd('/') + '/healthz') -TimeoutSec 10
if ($health.status -ne 'ok') { throw 'public health check failed' }
$adminOnPublic = Invoke-WebRequest -Uri ($BaseUrl.TrimEnd('/') + '/admin/status') -SkipHttpErrorCheck -TimeoutSec 10
if ($adminOnPublic.StatusCode -ne 404) { throw "admin route is exposed on public listener: $($adminOnPublic.StatusCode)" }
$base = [Uri]$BaseUrl
$client = [Net.Sockets.TcpClient]::new()
$adminReachable = $false
try {
    $connect = $client.ConnectAsync($base.Host, 19091)
    if ($connect.Wait(2000)) {
        try {
            $connect.GetAwaiter().GetResult()
            $adminReachable = $client.Connected
        } catch {
            $adminReachable = $false
        }
    }
} finally {
    $client.Dispose()
}
if ($adminReachable) { throw 'admin port 19091 is reachable from the public network' }
$health | ConvertTo-Json -Depth 5
