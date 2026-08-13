param(
    [ValidateSet('test', 'vet', 'build', 'cross', 'all')]
    [string]$Task = 'all'
)

$ErrorActionPreference = 'Stop'
$ProjectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$Commit = (& git rev-parse --short HEAD 2>$null)
if (-not $Commit) { $Commit = 'unknown' }
$Version = (& git describe --tags --always --dirty 2>$null)
if (-not $Version) { $Version = 'dev' }
$BuildDate = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
$Ldflags = "-s -w -X github.com/arpingblue/AIStat/internal/version.Version=$Version -X github.com/arpingblue/AIStat/internal/version.Commit=$Commit -X github.com/arpingblue/AIStat/internal/version.Date=$BuildDate"
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
		go build -trimpath -ldflags $Ldflags -o 'bin/aistat.exe' ./cmd/aistat
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
			go build -trimpath -ldflags $Ldflags -o 'dist/aistat-linux-amd64' ./cmd/aistat
            $env:GOARCH = 'arm64'
			go build -trimpath -ldflags $Ldflags -o 'dist/aistat-linux-arm64' ./cmd/aistat
        } finally {
            $env:CGO_ENABLED = $PreviousCGO
            $env:GOOS = $PreviousOS
            $env:GOARCH = $PreviousArch
        }
    }
} finally {
    Pop-Location
}
