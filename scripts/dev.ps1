param(
    [ValidateSet('test', 'vet', 'build', 'cross', 'all')]
    [string]$Task = 'all'
)

$ErrorActionPreference = 'Stop'
$ProjectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
Push-Location $ProjectRoot
try {
    if ($Task -in @('test', 'all')) {
        go test ./...
    }
    if ($Task -in @('vet', 'all')) {
        go vet ./...
    }
    if ($Task -in @('build', 'all')) {
        New-Item -ItemType Directory -Force -Path 'bin' | Out-Null
        go build -trimpath -o 'bin/aistat.exe' ./cmd/aistat
    }
    if ($Task -in @('cross', 'all')) {
        New-Item -ItemType Directory -Force -Path 'dist' | Out-Null
        $PreviousCGO = $env:CGO_ENABLED
        $PreviousOS = $env:GOOS
        $PreviousArch = $env:GOARCH
        try {
            $env:CGO_ENABLED = '0'
            $env:GOOS = 'linux'
            $env:GOARCH = 'amd64'
            go build -trimpath -o 'dist/aistat-linux-amd64' ./cmd/aistat
            $env:GOARCH = 'arm64'
            go build -trimpath -o 'dist/aistat-linux-arm64' ./cmd/aistat
        } finally {
            $env:CGO_ENABLED = $PreviousCGO
            $env:GOOS = $PreviousOS
            $env:GOARCH = $PreviousArch
        }
    }
} finally {
    Pop-Location
}
