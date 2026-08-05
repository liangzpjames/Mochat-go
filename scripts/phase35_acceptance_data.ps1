[CmdletBinding()]
param(
    [ValidateSet('create', 'verify', 'cleanup')][string]$Action = 'verify',
    [Parameter(Mandatory = $true)][string]$BaseUrl,
    [Parameter(Mandatory = $true)][ValidatePattern('^P35-ACCEPT-[A-Za-z0-9-]+$')][string]$EnvironmentId,
    [Parameter(Mandatory = $true)][long]$CorpId,
    [string]$Token,
    [switch]$AllowIsolatedEnvironment
)

$ErrorActionPreference = 'Stop'
$prefix = 'P35-ACCEPT-'

if (-not $AllowIsolatedEnvironment) {
    throw 'Refused: -AllowIsolatedEnvironment is required for an isolated acceptance environment.'
}
if ($CorpId -le 0) { throw 'CorpId must be greater than zero.' }
if (-not $EnvironmentId.StartsWith($prefix, [System.StringComparison]::Ordinal)) {
    throw "EnvironmentId must start with $prefix."
}

$uri = [Uri]$BaseUrl
if ($uri.Scheme -notin @('http', 'https')) { throw 'BaseUrl must use http or https.' }
$headers = @{ 'X-Phase35-Acceptance-Environment' = $EnvironmentId }
if ($Token) { $headers.Authorization = "Bearer $Token" }
$endpoint = "{0}/dashboard/acceptance/phase35?corpId={1}&prefix={2}" -f $BaseUrl.TrimEnd('/'), $CorpId, [Uri]::EscapeDataString($prefix)

$invoke = @{
    Uri = $endpoint
    Headers = $headers
    ContentType = 'application/json; charset=utf-8'
    TimeoutSec = 20
}

switch ($Action) {
    'create' {
        $body = @{ environmentId = $EnvironmentId; prefix = $prefix } | ConvertTo-Json
        $result = Invoke-RestMethod @invoke -Method Post -Body $body
    }
    'verify' {
        $result = Invoke-RestMethod @invoke -Method Get
    }
    'cleanup' {
        # The server must validate the prefix again; no broader cleanup scope is sent.
        $body = @{ environmentId = $EnvironmentId; prefix = $prefix } | ConvertTo-Json
        $result = Invoke-RestMethod @invoke -Method Delete -Body $body
    }
}

if ($null -eq $result -or $result.prefix -ne $prefix) {
    throw 'The acceptance API did not echo the controlled prefix.'
}
$result | ConvertTo-Json -Depth 10
