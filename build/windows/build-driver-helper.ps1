param(
    [string]$OutputDirectory = "bin",
    [string]$Architecture = "amd64"
)
$ErrorActionPreference = "Stop"
if ($Architecture -ne "amd64") {
    Write-Host "Process proxy helper is only available for Windows x64."
    exit 0
}
$root = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
if (-not [IO.Path]::IsPathRooted($OutputDirectory)) {
    $OutputDirectory = Join-Path $root $OutputDirectory
}
$resource = Join-Path $root "cmd/sgfg-driver-helper/helper_windows_amd64.syso"
if (Test-Path -LiteralPath $resource) { throw "Resource already exists: $resource" }
New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$oldGOOS = $env:GOOS
$oldGOARCH = $env:GOARCH
$oldCGO = $env:CGO_ENABLED
Push-Location $root
try {
    & wails3 generate syso -arch amd64 -icon build/windows/icon.ico -manifest build/windows/helper.exe.manifest -info build/windows/helper-info.json -out $resource
    if ($LASTEXITCODE -ne 0) { throw "Helper resource generation failed" }
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"
    & go build -trimpath -buildvcs=false -ldflags='-w -s -H windowsgui' -o (Join-Path $OutputDirectory "sgfg-driver-helper.exe") ./cmd/sgfg-driver-helper
    if ($LASTEXITCODE -ne 0) { throw "Helper build failed" }
} finally {
    if (Test-Path -LiteralPath $resource) { Remove-Item -LiteralPath $resource }
    $env:GOOS = $oldGOOS
    $env:GOARCH = $oldGOARCH
    $env:CGO_ENABLED = $oldCGO
    Pop-Location
}
