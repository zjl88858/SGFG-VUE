param([string]$OutputDirectory = "bin/licenses")
$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
if (-not [IO.Path]::IsPathRooted($OutputDirectory)) {
    $OutputDirectory = Join-Path $root $OutputDirectory
}
$omhy = Join-Path $root "third_party/oh-my-hysteria"
$files = @{
    "THIRD_PARTY_LICENSES.md" = Join-Path $root "THIRD_PARTY_LICENSES.md"
    "LICENSE-omhy.txt" = Join-Path $omhy "LICENSE.md"
    "LICENSE-omhy-third-party.md" = Join-Path $omhy "LICENSE-THIRD-PARTY.md"
    "LICENSE-quic-go.txt" = Join-Path $omhy "third_party/quic-go/LICENSE"
    "LICENSE-tquic.txt" = Join-Path $omhy "third_party/tquic/LICENSE"
    "LICENSE-geodesic-kcc.txt" = Join-Path $omhy "third_party/geodesic/LICENSE-kcc"
    "LICENSE-geodesic-ucp.txt" = Join-Path $omhy "third_party/geodesic/LICENSE-ucp"
}
foreach ($source in $files.Values) {
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) { throw "Missing required license: $source" }
}
New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
foreach ($name in $files.Keys) {
    Copy-Item -LiteralPath $files[$name] -Destination (Join-Path $OutputDirectory $name) -Force
}
Write-Host "Core license notices staged to $OutputDirectory"
