[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]$EvidenceDir,
  [string]$BaseUrl = "http://127.0.0.1:18080",
  [string]$Phone,
  [string]$Password,
  [string]$Jwt,
  [string]$MysqlUser,
  [string]$MysqlPassword,
  [string]$MysqlDatabase = "mochat"
)
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$script:Jwt = $Jwt
New-Item -ItemType Directory -Force -Path $EvidenceDir | Out-Null

function Write-Json([string]$Name, $Value) { $Value | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath (Join-Path $EvidenceDir $Name) -Encoding utf8 }
function Invoke-Contract([string]$Name, [string]$Path) {
  try {
    $headers = @{}; if ($script:Jwt) { $headers.Authorization = "Bearer $script:Jwt" }
    $response = Invoke-WebRequest -UseBasicParsing -Uri "$BaseUrl$Path" -Method Get -Headers $headers
    $body = $response.Content | ConvertFrom-Json
    Write-Json $Name @{ status = [int]$response.StatusCode; code = $body.code; machineCode = $body.msg; hasData = ($null -ne $body.data); contentLength = $response.Content.Length }
    return [int]$response.StatusCode
  } catch {
    $status = if ($null -ne $_.Exception.Response) { [int]$_.Exception.Response.StatusCode } else { 0 }
    Write-Json $Name @{ status = $status; error = "request failed" }
    return $status
  }
}
if (-not $Jwt -and $Phone -and $Password) {
  $login = Invoke-RestMethod -Uri "$BaseUrl/dashboard/user/auth" -Method Post -ContentType "application/json" -Body (@{ phone = $Phone; password = $Password } | ConvertTo-Json)
  $tokenValue = $login.data.token
  if (-not $tokenValue) { $tokenValue = $login.token }
  $script:Jwt = [string]$tokenValue
  if (-not $script:Jwt) { throw "dashboard/user/auth did not return a token" }
}
if (-not $Jwt -and -not ($Phone -and $Password)) { throw "provide -Jwt or -Phone and -Password; credentials are never written to evidence" }
function Snapshot-Volumes([string]$Name) {
  $names = @(docker volume ls --format "{{.Name}}" | Where-Object { $_ -like "mochat-go-desktop*" })
  if ($names.Count -lt 4) { throw "expected at least four mochat-go-desktop volumes" }
  $items = foreach ($volume in $names) { docker volume inspect $volume | ConvertFrom-Json | Select-Object Name, Mountpoint, Labels }
  Write-Json $Name $items
}

$compose = docker compose -p mochat-go-desktop ps --format json 2>$null
if ($LASTEXITCODE -ne 0 -or -not ($compose | Select-String "mochat-go-desktop")) { throw "compose project mochat-go-desktop is not available" }
Set-Content -LiteralPath (Join-Path $EvidenceDir "compose-ps.json") -Value $compose -Encoding utf8
Snapshot-Volumes "volumes-before.json"
$containerIds = @{ app = (docker compose -p mochat-go-desktop ps -q app); mysql = (docker compose -p mochat-go-desktop ps -q mysql); redis = (docker compose -p mochat-go-desktop ps -q redis) }
Write-Json "container-ids.json" $containerIds
if ($MysqlUser -and $MysqlPassword) {
  $env:MYSQL_PWD = $MysqlPassword
  try { $counts = docker compose -p mochat-go-desktop exec -T mysql mariadb -u$MysqlUser $MysqlDatabase -N -B -e "SELECT table_name,table_rows FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('mc_user','mc_rbac_role','mochat_go_dashboard_permissions','mochat_go_dashboard_user_roles','mochat_go_dashboard_user_permissions','mochat_go_dashboard_permission_audits')"; Set-Content -LiteralPath (Join-Path $EvidenceDir "table-counts-before.txt") -Value $counts -Encoding utf8 } finally { Remove-Item Env:MYSQL_PWD -ErrorAction SilentlyContinue }
}

$statuses = @{}
$statuses.readyz = Invoke-Contract "readyz.json" "/readyz"
$statuses.profile = Invoke-Contract "access-profile.json" "/dashboard/access/profile"
$statuses.catalog = Invoke-Contract "access-catalog.json" "/dashboard/access/catalog"
$statuses.users = Invoke-Contract "access-users.json" "/dashboard/access/users?page=1&perPage=50"
$statuses.roles = Invoke-Contract "access-roles.json" "/dashboard/access/roles?page=1&perPage=50"
$statuses.audits = Invoke-Contract "access-audits.json" "/dashboard/access/audits?page=1&perPage=50"

Snapshot-Volumes "volumes-after.json"
if ($MysqlUser -and $MysqlPassword) {
  $env:MYSQL_PWD = $MysqlPassword
  try { $counts = docker compose -p mochat-go-desktop exec -T mysql mariadb -u$MysqlUser $MysqlDatabase -N -B -e "SELECT table_name,table_rows FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('mc_user','mc_rbac_role','mochat_go_dashboard_permissions','mochat_go_dashboard_user_roles','mochat_go_dashboard_user_permissions','mochat_go_dashboard_permission_audits')"; Set-Content -LiteralPath (Join-Path $EvidenceDir "table-counts-after.txt") -Value $counts -Encoding utf8 } finally { Remove-Item Env:MYSQL_PWD -ErrorAction SilentlyContinue }
}
Write-Json "smoke-contract.json" @{ baseUrl = $BaseUrl; project = "mochat-go-desktop"; tenantGate = "TENANT_ACCESS_DENIED"; pageGate = "DASHBOARD_PERMISSION_DENIED"; catalog = "53 pages / 49 ordinary / 4 superadmin_only"; statuses = $statuses; destructiveOperations = @() }
