[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]$EvidenceDir,
  [string]$BaseUrl = "http://127.0.0.1:18080",
  [string]$Phone,
  [string]$Password,
  [string]$Jwt,
  [string]$MysqlDatabase = "mochat",
  [int]$TargetUserId,
  [int]$CrossTenantUserId,
  [Parameter(Mandatory = $false)][string]$MutationJson,
  [switch]$ReadOnly
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
if (-not $ReadOnly -and ($TargetUserId -le 0 -or $CrossTenantUserId -le 0 -or -not $MutationJson)) { throw "full smoke requires TargetUserId, CrossTenantUserId, and MutationJson; use -ReadOnly explicitly for diagnostics" }
function Snapshot-Volumes([string]$Name) {
  $names = @(docker volume ls --format "{{.Name}}" | Where-Object { $_ -like "mochat-go-desktop*" })
  if ($names.Count -lt 4) { throw "expected at least four mochat-go-desktop volumes" }
  $items = foreach ($volume in $names) { docker volume inspect $volume | ConvertFrom-Json | Select-Object Name, Mountpoint, Labels }
  Write-Json $Name $items
  return $items
}

$compose = docker compose -p mochat-go-desktop ps --format json 2>$null
if ($LASTEXITCODE -ne 0 -or -not ($compose | Select-String "mochat-go-desktop")) { throw "compose project mochat-go-desktop is not available" }
Set-Content -LiteralPath (Join-Path $EvidenceDir "compose-ps.json") -Value $compose -Encoding utf8
$beforeVolumes = @(Snapshot-Volumes "volumes-before.json")
$containerIds = @{ app = (docker compose -p mochat-go-desktop ps -q app); mysql = (docker compose -p mochat-go-desktop ps -q mysql); redis = (docker compose -p mochat-go-desktop ps -q redis) }
Write-Json "container-ids.json" $containerIds
$countQuery = "SELECT 'mc_user',COUNT(*) FROM mc_user UNION ALL SELECT 'mc_rbac_role',COUNT(*) FROM mc_rbac_role UNION ALL SELECT 'permissions',COUNT(*) FROM mochat_go_dashboard_permissions UNION ALL SELECT 'user_roles',COUNT(*) FROM mochat_go_dashboard_user_roles UNION ALL SELECT 'user_permissions',COUNT(*) FROM mochat_go_dashboard_user_permissions UNION ALL SELECT 'audits',COUNT(*) FROM mochat_go_dashboard_permission_audits"
$counts = docker compose -p mochat-go-desktop exec -T mysql sh -lc "mariadb -u\"`$MYSQL_USER\" -p\"`$MYSQL_PASSWORD\" \"`$MYSQL_DATABASE\" -N -B -e '$countQuery'"; Set-Content -LiteralPath (Join-Path $EvidenceDir "table-counts-before.txt") -Value $counts -Encoding utf8
function Convert-TableCounts([string]$Raw) { $result = @{}; foreach ($line in ($Raw -split "`r?`n")) { $parts = $line -split "`t"; if ($parts.Count -ge 2) { $result[$parts[0]] = [int64]$parts[1] } }; return $result }
$beforeCountMap = Convert-TableCounts ([string]$counts)

$statuses = @{}
$statuses.readyz = Invoke-Contract "readyz.json" "/readyz"
$statuses.profile = Invoke-Contract "access-profile.json" "/dashboard/access/profile"
$statuses.catalog = Invoke-Contract "access-catalog.json" "/dashboard/access/catalog"
$statuses.users = Invoke-Contract "access-users.json" "/dashboard/access/users?page=1&perPage=50"
$statuses.roles = Invoke-Contract "access-roles.json" "/dashboard/access/roles?page=1&perPage=50"
$statuses.audits = Invoke-Contract "access-audits.json" "/dashboard/access/audits?page=1&perPage=50"
$statuses.unknown = Invoke-Contract "unknown-api.json" "/dashboard/access/not-registered"
if ($statuses.unknown -lt 400) { throw "unknown dashboard API was not denied" }
if ($CrossTenantUserId -gt 0) { $statuses.crossTenant = Invoke-Contract "cross-tenant-user.json" "/dashboard/access/users/$CrossTenantUserId"; if ($statuses.crossTenant -ne 404) { throw "cross-tenant user did not return 404" } }
if (-not $ReadOnly -and $TargetUserId -gt 0 -and $MutationJson) {
  $headers = @{ Authorization = "Bearer $script:Jwt" }
  $mutation = Invoke-RestMethod -Uri "$BaseUrl/dashboard/access/users/$TargetUserId" -Method Put -Headers $headers -ContentType "application/json" -Body $MutationJson
  Write-Json "mutation-result.json" @{ code = $mutation.code; hasData = ($null -ne $mutation.data) }
  $statuses.auditAfterMutation = Invoke-Contract "audits-after-mutation.json" "/dashboard/access/audits?page=1&perPage=50"
  try { Invoke-WebRequest -UseBasicParsing -Uri "$BaseUrl/dashboard/access/users/$TargetUserId" -Method Put -Headers $headers -ContentType "application/json" -Body $MutationJson | Out-Null; $staleStatus = 200 } catch { $staleStatus = if ($_.Exception.Response) { [int]$_.Exception.Response.StatusCode } else { 0 } }
  Write-Json "stale-version-result.json" @{ status = $staleStatus; conflict = ($staleStatus -eq 409) }
  if ($staleStatus -ne 409) { throw "expected stale expectedVersion mutation to return 409" }
}

$afterVolumes = @(Snapshot-Volumes "volumes-after.json")
if ((ConvertTo-Json $beforeVolumes) -ne (ConvertTo-Json $afterVolumes)) { throw "volume name/mountpoint changed during smoke" }
$containerIdsAfter = @{ app = (docker compose -p mochat-go-desktop ps -q app); mysql = (docker compose -p mochat-go-desktop ps -q mysql); redis = (docker compose -p mochat-go-desktop ps -q redis) }
Write-Json "container-ids-after.json" $containerIdsAfter
if ((ConvertTo-Json $containerIds) -ne (ConvertTo-Json $containerIdsAfter)) { throw "app/mysql/redis container IDs changed during smoke" }
$afterCountsRaw = docker compose -p mochat-go-desktop exec -T mysql sh -lc "mariadb -u\"`$MYSQL_USER\" -p\"`$MYSQL_PASSWORD\" \"`$MYSQL_DATABASE\" -N -B -e '$countQuery'"; Set-Content -LiteralPath (Join-Path $EvidenceDir "table-counts-after.txt") -Value $afterCountsRaw -Encoding utf8
$afterCountMap = Convert-TableCounts ([string]$afterCountsRaw)
$countDelta = @{}; foreach ($name in $beforeCountMap.Keys) { $countDelta[$name] = $afterCountMap[$name] - $beforeCountMap[$name] }
Write-Json "table-count-delta.json" $countDelta
if ($ReadOnly) { if ((ConvertTo-Json $beforeCountMap -Compress) -ne (ConvertTo-Json $afterCountMap -Compress)) { throw "read-only smoke changed table counts" } }
else { if (-not $statuses.auditAfterMutation -or $countDelta.audits -ne 1) { throw "full smoke expected exactly one audit row after protected mutation" } }
Write-Json "smoke-contract.json" @{ baseUrl = $BaseUrl; project = "mochat-go-desktop"; tenantGate = "TENANT_ACCESS_DENIED"; pageGate = "DASHBOARD_PERMISSION_DENIED"; catalog = "53 pages / 49 ordinary / 4 superadmin_only"; statuses = $statuses; destructiveOperations = @() }
