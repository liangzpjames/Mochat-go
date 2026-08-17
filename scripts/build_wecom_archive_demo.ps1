param(
    [string]$OutputDirectory = 'output/wecom-archive-demo-release',
    [string]$PublicUrl = 'http://139.196.34.133:19090'
)
$ErrorActionPreference = 'Stop'

function Assert-NativeSuccess([string]$Operation) {
    if ($LASTEXITCODE -ne 0) { throw "$Operation failed with exit code $LASTEXITCODE" }
}

$repo = Split-Path -Parent $PSScriptRoot
$release = [System.IO.Path]::GetFullPath((Join-Path $repo $OutputDirectory))
$allowedRoot = [System.IO.Path]::GetFullPath((Join-Path $repo 'output')) + [System.IO.Path]::DirectorySeparatorChar
if (-not $release.StartsWith($allowedRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw 'release directory must be inside repository output/'
}
if (Test-Path -LiteralPath $release) {
    Remove-Item -Recurse -Force -LiteralPath $release
}
New-Item -ItemType Directory -Force -Path $release | Out-Null
$buildDirectory = Join-Path $repo '.tmp-wecom-archive-demo-build'
if (Test-Path -LiteralPath $buildDirectory) {
    Remove-Item -Recurse -Force -LiteralPath $buildDirectory
}
New-Item -ItemType Directory -Force -Path $buildDirectory | Out-Null

Push-Location $repo
try {
    go test ./internal/wecomarchivedemo ./cmd/wecom-archive-demo -count=1
    Assert-NativeSuccess 'Go tests'
    $gitSha = (git rev-parse --short=12 HEAD)
    Assert-NativeSuccess 'Git revision lookup'
    $gitSha = $gitSha.Trim()
    $image = "mochat/wecom-archive-demo:$gitSha"
    $oldCGO = $env:CGO_ENABLED
    $oldGOOS = $env:GOOS
    $oldGOARCH = $env:GOARCH
    try {
        $env:CGO_ENABLED = '0'
        $env:GOOS = 'linux'
        $env:GOARCH = 'amd64'
        go build -trimpath -ldflags '-s -w' -o (Join-Path $buildDirectory 'wecom-archive-demo') ./cmd/wecom-archive-demo
        Assert-NativeSuccess 'Linux amd64 cross compilation'
    } finally {
        $env:CGO_ENABLED = $oldCGO
        $env:GOOS = $oldGOOS
        $env:GOARCH = $oldGOARCH
    }
    docker build --platform linux/amd64 -f deploy/wecom-archive-demo/Dockerfile -t $image .
    Assert-NativeSuccess 'Docker image build'
    docker run --rm $image sdkcheck
    Assert-NativeSuccess 'Official WeCom Finance SDK load check'
    go run ./cmd/wecom-archive-demo keygen --output (Join-Path $release 'secrets') --public-url $PublicUrl
    Assert-NativeSuccess 'Demo configuration generation'

    $data = Join-Path $release 'smoke-data'
    New-Item -ItemType Directory -Force -Path $data | Out-Null
    $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    $listener.Start()
    $port = ([System.Net.IPEndPoint]$listener.LocalEndpoint).Port
    $listener.Stop()
    $smokeName = "wecom-archive-demo-smoke-$PID"
    try {
        docker run -d --name $smokeName -p "127.0.0.1:${port}:8080" --mount "type=bind,src=$(Join-Path $release 'secrets/config.json'),dst=/config/config.json,readonly" --mount "type=bind,src=$data,dst=/data" $image serve --config /config/config.json | Out-Null
        Assert-NativeSuccess 'Demo container smoke start'
        $healthy = $false
        for ($attempt = 0; $attempt -lt 30; $attempt++) {
            try {
                $health = Invoke-RestMethod -Uri "http://127.0.0.1:$port/healthz" -TimeoutSec 2
                if ($health.status -eq 'ok' -and $health.sdk_loaded -eq $false) { $healthy = $true; break }
            } catch { Start-Sleep -Milliseconds 500 }
        }
        if (-not $healthy) { throw 'container smoke test failed' }
    } finally {
        docker rm -f $smokeName 2>$null | Out-Null
    }

    docker save -o (Join-Path $release 'image.tar') $image
    Assert-NativeSuccess 'Docker image export'
    Set-Content -LiteralPath (Join-Path $release 'image-name.txt') -Value $image -NoNewline
    Copy-Item scripts/deploy_wecom_archive_demo.sh,scripts/configure_wecom_archive_demo.sh,scripts/run_wecom_archive_demo.sh -Destination $release
    $imageHash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $release 'image.tar')).Hash.ToLowerInvariant()
    $nameHash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $release 'image-name.txt')).Hash.ToLowerInvariant()
    Set-Content -LiteralPath (Join-Path $release 'checksums.sha256') -Value @("$imageHash  image.tar", "$nameHash  image-name.txt")
    Write-Output "release=$release"
    Write-Output "image=$image"
} finally {
    if (Test-Path -LiteralPath $buildDirectory) {
        Remove-Item -Recurse -Force -LiteralPath $buildDirectory
    }
    Pop-Location
}
