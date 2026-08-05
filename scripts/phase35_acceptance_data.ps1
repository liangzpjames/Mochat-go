[CmdletBinding()]
param([ValidateSet('create','verify','cleanup')][string]$Action='verify',[string]$BaseUrl='http://127.0.0.1:18080',[string]$Token)
$ErrorActionPreference='Stop'
if ($Action -eq 'cleanup') { throw '本命令仅允许通过真实受控 API 清理 P35-ACCEPT- 数据；未提供 API 清理实现，拒绝执行。' }
if ($Action -eq 'create') { throw '验收数据必须通过订单/设置真实 API 创建；请使用隔离环境运行，脚本不直接写数据库。' }
$headers=@{}; if($Token){$headers.Authorization="Bearer $Token"}
$url="$BaseUrl/readyz"; $response=Invoke-WebRequest -UseBasicParsing -Headers $headers -Uri $url -TimeoutSec 10
if($response.StatusCode -lt 200 -or $response.StatusCode -ge 400){throw "应用不可用：HTTP $($response.StatusCode)"}
Write-Host 'Phase 3.5 验收环境可用；数据创建/清理需通过真实 API 与隔离库执行。'
