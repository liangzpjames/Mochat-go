[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]$EvidenceDir,
  [string]$BaseUrl = "http://localhost:18080",
  [string]$ComposeFile = "",
  [int]$CrossTenantUserId = 0,
  [string]$CredentialSecretFile = "",
  [switch]$ReadOnly,
  [switch]$ExerciseFixtureWrites
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$composeProject = "mochat-go-desktop"
$script:DashboardAuthorization = ""
$script:Evidence = @{}

if ([string]::IsNullOrWhiteSpace($ComposeFile)) {
  $ComposeFile = Join-Path -Path $PSScriptRoot -ChildPath "..\deploy\standalone\docker-compose.yml"
}

function Write-SafeJson([string]$Name, [object]$Value) {
  $Value | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath (Join-Path $EvidenceDir $Name) -Encoding utf8
}

function Get-Property([object]$Object, [string]$Name) {
  if ($null -eq $Object) { return $null }
  $property = $Object.PSObject.Properties[$Name]
  if ($null -eq $property) { return $null }
  return $property.Value
}

function Get-ResponseData([object]$Response) {
  $body = Get-Property $Response "Json"
  $data = Get-Property $body "data"
  if ($null -ne $data) { return $data }
  return $body
}

function Get-MachineCode([object]$Response) {
  $body = Get-Property $Response "Json"
  $errorCode = Get-Property $body "errorCode"
  if ($null -ne $errorCode -and [string]$errorCode -ne "") { return [string]$errorCode }
  $machineCode = Get-Property $body "machineCode"
  if ($null -ne $machineCode -and [string]$machineCode -ne "") { return [string]$machineCode }
  $code = Get-Property $body "code"
  if ($null -ne $code) { return [string]$code }
  return ""
}

function Add-ResponseEvidence([string]$Name, [object]$Response) {
  $script:Evidence[$Name] = @{
    status = [int](Get-Property $Response "StatusCode")
    machineCode = Get-MachineCode $Response
    contentLength = [int64](Get-Property $Response "ContentLength")
  }
}

function Invoke-Api([string]$Name, [string]$Path, [string]$Method = "GET", [object]$Body = $null) {
  $headers = @{ Accept = "application/json" }
  if (-not [string]::IsNullOrWhiteSpace($script:DashboardAuthorization)) {
    $headers.Authorization = $script:DashboardAuthorization
  }
  $request = @{
    UseBasicParsing = $true
    Uri = "$BaseUrl$Path"
    Method = $Method
    Headers = $headers
    ErrorAction = "Stop"
  }
  if ($null -ne $Body) {
    $request.ContentType = "application/json"
    $request.Body = ($Body | ConvertTo-Json -Depth 12 -Compress)
  }
  $status = 0
  $content = ""
  try {
    $response = Invoke-WebRequest @request
    $status = [int]$response.StatusCode
    $content = [string]$response.Content
  } catch {
    $responseProperty = $_.Exception.PSObject.Properties["Response"]
    if ($null -ne $responseProperty -and $null -ne $responseProperty.Value) {
      $errorResponse = $responseProperty.Value
      $status = [int]$errorResponse.StatusCode
      try {
        $reader = New-Object System.IO.StreamReader($errorResponse.GetResponseStream())
        try { $content = $reader.ReadToEnd() } finally { $reader.Dispose() }
      } catch { $content = "" }
    }
  }
  $json = $null
  if (-not [string]::IsNullOrWhiteSpace($content)) {
    try { $json = $content | ConvertFrom-Json } catch { $json = $null }
  }
  return [pscustomobject]@{
    Name = $Name
    StatusCode = $status
    Json = $json
    Content = $content
    ContentLength = $content.Length
  }
}

function Assert-Status([object]$Response, [int]$Expected, [string]$Description) {
  if ([int](Get-Property $Response "StatusCode") -ne $Expected) {
    throw "$Description returned unexpected HTTP status"
  }
}

function Assert-NoCredentialMaterial([object]$Response, [object[]]$SensitiveValues) {
  $content = [string](Get-Property $Response "Content")
  foreach ($value in $SensitiveValues) {
    if ($null -ne $value -and [string]$value -ne "" -and $content.Contains([string]$value)) {
      throw "protected API response contained credential material"
    }
  }
  if ($content -match '"(?:password|jwt|secret|token|privateKey)"\s*:') {
    throw "protected API response exposed a forbidden credential field"
  }
}

function Resolve-DashboardAuthorization {
  $environmentToken = [string]$env:MOCHAT_IDENTITY_SINGLE_CORP_DASHBOARD_TOKEN
  if (-not [string]::IsNullOrWhiteSpace($environmentToken)) {
    if ($environmentToken -match '^Bearer\s+') { return $environmentToken }
    return "Bearer $environmentToken"
  }
  if ($ReadOnly) {
    throw "ReadOnly requires MOCHAT_IDENTITY_SINGLE_CORP_DASHBOARD_TOKEN so no login mutation is performed"
  }
  $login = [string]$env:MOCHAT_IDENTITY_SINGLE_CORP_DASHBOARD_LOGIN
  $password = [string]$env:MOCHAT_IDENTITY_SINGLE_CORP_DASHBOARD_PASSWORD
  if ([string]::IsNullOrWhiteSpace($login) -or [string]::IsNullOrWhiteSpace($password)) {
    throw "fixture-write mode requires dashboard credentials from protected environment variables"
  }
  $loginResponse = Invoke-Api "dashboard-login" "/dashboard/user/auth" "POST" @{ phone = $login; password = $password }
  Assert-Status $loginResponse 200 "Dashboard login"
  $loginData = Get-ResponseData $loginResponse
  $token = Get-Property $loginData "token"
  if ($null -eq $token -or [string]$token -eq "") { throw "Dashboard login did not return a token" }
  if ([string]$token -match '^Bearer\s+') { return [string]$token }
  return "Bearer $token"
}

function Get-ContainerIds([string]$Name) {
  $ids = @{}
  foreach ($service in @("app", "mysql", "redis")) {
    $value = (docker compose @script:ComposeArgs ps -q $service | Out-String).Trim()
    if ([string]::IsNullOrWhiteSpace($value)) { throw "compose service $service is not running" }
    $ids[$service] = $value
  }
  Write-SafeJson $Name $ids
  return $ids
}

function Get-VolumeSnapshot([string]$Name) {
  $expectedVolumes = @(
    "mochat-go-desktop_app-storage",
    "mochat-go-desktop_audit-anchor-storage",
    "mochat-go-desktop_mysql-data",
    "mochat-go-desktop_redis-data"
  )
  $actualNames = @(docker volume ls --format "{{.Name}}" | Where-Object { $expectedVolumes -contains $_ })
  if ($actualNames.Count -ne $expectedVolumes.Count) { throw "expected exactly four mochat-go-desktop volumes" }
  $snapshot = @()
  foreach ($volume in $expectedVolumes) {
    $inspected = @(docker volume inspect $volume | ConvertFrom-Json)
    if ($inspected.Count -ne 1) { throw "volume inspection did not return one object" }
    $snapshot += [pscustomobject]@{
      Name = [string]$inspected[0].Name
      Mountpoint = [string]$inspected[0].Mountpoint
    }
  }
  Write-SafeJson $Name $snapshot
  return $snapshot
}

$composePath = (Resolve-Path -LiteralPath $ComposeFile -ErrorAction Stop).Path
$script:ComposeArgs = @("-f", $composePath, "-p", $composeProject)
if ($ReadOnly -and $ExerciseFixtureWrites) { throw "-ReadOnly and -ExerciseFixtureWrites are mutually exclusive" }
if (-not $ReadOnly -and -not $ExerciseFixtureWrites) { throw "choose exactly one of -ReadOnly or -ExerciseFixtureWrites" }
if ($ExerciseFixtureWrites -and $CrossTenantUserId -le 0) { throw "-ExerciseFixtureWrites requires -CrossTenantUserId" }

New-Item -ItemType Directory -Force -Path $EvidenceDir | Out-Null
$composeStatus = docker compose @script:ComposeArgs ps --format json 2>$null
if ($LASTEXITCODE -ne 0) { throw "compose project mochat-go-desktop is not available" }
$script:DashboardAuthorization = Resolve-DashboardAuthorization
$beforeVolumes = @(Get-VolumeSnapshot "volumes-before.json")
$beforeContainers = Get-ContainerIds "container-ids-before.json"

$countQuery = @"
SELECT 'mc_user', COUNT(*) FROM mc_user
UNION ALL SELECT 'mc_corp', COUNT(*) FROM mc_corp
UNION ALL SELECT 'tenant_corp_bindings', COUNT(*) FROM mochat_go_tenant_corp_bindings
UNION ALL SELECT 'dashboard_identities', COUNT(*) FROM mochat_go_dashboard_identities
UNION ALL SELECT 'saas_admin_users', COUNT(*) FROM mochat_go_saas_admin_users
UNION ALL SELECT 'identity_activations', COUNT(*) FROM mochat_go_dashboard_identity_activations
"@

function Get-TableCounts {
  $raw = $countQuery | docker compose @script:ComposeArgs exec -T mysql sh -lc 'export MYSQL_PWD="$MARIADB_PASSWORD"; mariadb --batch --skip-column-names -u"$MARIADB_USER" "$MARIADB_DATABASE"'
  if ($LASTEXITCODE -ne 0) { throw "failed to read protected table counts" }
  return [string]($raw | Out-String)
}

function Convert-TableCounts([string]$Raw) {
  $result = @{}
  foreach ($line in ($Raw -split "`r?`n")) {
    $trimmed = $line.Trim()
    if ($trimmed -eq "") { continue }
    $parts = $trimmed -split "`t"
    if ($parts.Count -lt 2) { $parts = $trimmed -split "\s+" }
    if ($parts.Count -lt 2) { throw "table count output is malformed" }
    $result[$parts[0]] = [int64]$parts[1]
  }
  return $result
}

$beforeCountsRaw = Get-TableCounts
Set-Content -LiteralPath (Join-Path $EvidenceDir "table-counts-before.txt") -Value $beforeCountsRaw -Encoding utf8
$beforeCounts = Convert-TableCounts $beforeCountsRaw

function Invoke-ReadOnlyCheck([string]$Name, [string]$Path) {
  $response = Invoke-Api $Name $Path "GET"
  Add-ResponseEvidence $Name $response
  Assert-Status $response 200 "read-only endpoint $Path"
  return $response
}

if ($ReadOnly) {
  [void](Invoke-ReadOnlyCheck "readyz" "/readyz")
  [void](Invoke-ReadOnlyCheck "dashboard-access-profile" "/dashboard/access/profile")
  $profileResponse = Invoke-ReadOnlyCheck "company-profile" "/dashboard/company/profile"
  [void](Invoke-ReadOnlyCheck "company-sync-status" "/dashboard/company/sync-status")
  [void](Invoke-ReadOnlyCheck "company-audits" "/dashboard/company/audits?page=1&perPage=20")
} else {
  $crossTenantResponse = Invoke-Api "cross-tenant-resource" "/dashboard/access/users/$CrossTenantUserId" "GET"
  Add-ResponseEvidence "cross-tenant-resource" $crossTenantResponse
  Assert-Status $crossTenantResponse 404 "cross-tenant protected resource"

  $profileResponse = Invoke-Api "company-profile-before-write" "/dashboard/company/profile" "GET"
  Add-ResponseEvidence "company-profile-before-write" $profileResponse
  Assert-Status $profileResponse 200 "company profile"
  $profile = Get-ResponseData $profileResponse
  $initialVersion = [int64](Get-Property $profile "bindingVersion")
  if ($initialVersion -le 0) { throw "company profile did not return a binding version" }

  $displayName = "identity-single-corp-smoke-$initialVersion"
  $profileWriteBody = @{ displayName = $displayName; expectedVersion = $initialVersion; requestId = "identity-single-corp-smoke-profile-$initialVersion" }
  $profileWrite = Invoke-Api "company-profile-write" "/dashboard/company/profile" "PUT" $profileWriteBody
  Add-ResponseEvidence "company-profile-write" $profileWrite
  Assert-Status $profileWrite 200 "company profile protected write"
  $profileAfterWrite = Get-ResponseData $profileWrite
  $nextVersion = [int64](Get-Property $profileAfterWrite "bindingVersion")
  if ($nextVersion -le $initialVersion) { throw "company profile write did not advance its version" }

  $staleWrite = Invoke-Api "stale-version" "/dashboard/company/profile" "PUT" $profileWriteBody
  Add-ResponseEvidence "stale-version" $staleWrite
  Assert-Status $staleWrite 409 "stale expectedVersion protected write"

  $secretPath = $CredentialSecretFile
  if ([string]::IsNullOrWhiteSpace($secretPath)) { $secretPath = [string]$env:MOCHAT_IDENTITY_SINGLE_CORP_CREDENTIAL_FILE }
  if ([string]::IsNullOrWhiteSpace($secretPath)) { throw "fixture-write mode requires a protected credential secret file" }
  $credentialFilePath = (Resolve-Path -LiteralPath $secretPath -ErrorAction Stop).Path
  $credentialDocument = Get-Content -LiteralPath $credentialFilePath -Raw -Encoding utf8 | ConvertFrom-Json
  $wecom = Get-Property $credentialDocument "wecom"
  if ($null -eq $wecom) { throw "credential secret file must contain a wecom object" }
  $credentialBody = @{ expectedVersion = $nextVersion; requestId = "identity-single-corp-smoke-wecom-$nextVersion" }
  $secretValues = @()
  foreach ($field in @("employeeSecret", "contactSecret", "callbackToken", "encodingAESKey", "chatSecret")) {
    $value = Get-Property $wecom $field
    if ($null -ne $value -and [string]$value -ne "") {
      $credentialBody[$field] = [string]$value
      $secretValues += [string]$value
    }
  }
  if ($secretValues.Count -eq 0) { throw "credential secret file did not provide a WeCom credential value" }
  $credentialWrite = Invoke-Api "wecom-credential-metadata" "/dashboard/company/wecom-credentials" "PUT" $credentialBody
  Add-ResponseEvidence "wecom-credential-metadata" $credentialWrite
  Assert-Status $credentialWrite 200 "WeCom credential metadata protected write"
  Assert-NoCredentialMaterial $credentialWrite $secretValues
  $credentialProfile = Get-ResponseData $credentialWrite
  $credentialMetadata = Get-Property $credentialProfile "credentials"
  Write-SafeJson "credential-metadata.json" @{
    wecom = Get-Property (Get-Property $credentialMetadata "wecom") "configured"
    keyId = Get-Property (Get-Property $credentialMetadata "wecom") "keyId"
    updatedAt = Get-Property (Get-Property $credentialMetadata "wecom") "updatedAt"
  }

  $auditResponse = Invoke-Api "company-audits-after-write" "/dashboard/company/audits?page=1&perPage=20" "GET"
  Add-ResponseEvidence "company-audits-after-write" $auditResponse
  Assert-Status $auditResponse 200 "company audit read"
  Assert-NoCredentialMaterial $auditResponse $secretValues
}

$afterCountsRaw = Get-TableCounts
Set-Content -LiteralPath (Join-Path $EvidenceDir "table-counts-after.txt") -Value $afterCountsRaw -Encoding utf8
$afterCounts = Convert-TableCounts $afterCountsRaw
$countDelta = @{}
foreach ($key in $beforeCounts.Keys) { $countDelta[$key] = $afterCounts[$key] - $beforeCounts[$key] }
Write-SafeJson "table-count-delta.json" $countDelta
if ($ReadOnly) {
  foreach ($key in $beforeCounts.Keys) {
    if ($beforeCounts[$key] -ne $afterCounts[$key]) { throw "read-only smoke changed table counts" }
  }
}

$afterVolumes = @(Get-VolumeSnapshot "volumes-after.json")
if ((ConvertTo-Json $beforeVolumes -Compress) -ne (ConvertTo-Json $afterVolumes -Compress)) { throw "volume name or mountpoint changed during smoke" }
$afterContainers = Get-ContainerIds "container-ids-after.json"
foreach ($service in @("mysql", "redis")) {
  if ($beforeContainers[$service] -ne $afterContainers[$service]) { throw "$service container ID changed during smoke" }
}

Write-SafeJson "smoke-contract.json" @{
  mode = if ($ReadOnly) { "ReadOnly" } else { "ExerciseFixtureWrites" }
  baseUrl = $BaseUrl
  composeProject = $composeProject
  composeFile = $composePath
  mysqlRedisContainerRetention = $true
  appContainerMayChange = $true
  expectedVolumes = @("app-storage", "audit-anchor-storage", "mysql-data", "redis-data")
  statuses = $script:Evidence
  destructiveOperations = @()
}
