param(
    [string]$OutputDirectory = "bin/sockfilter",
    [string]$Architecture = "amd64"
)
$ErrorActionPreference = "Stop"
if ($Architecture -ne "amd64") {
    Write-Host "SockFilter integration is x64 only; normal proxy build is unchanged."
    exit 0
}
$root = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
if (-not [IO.Path]::IsPathRooted($OutputDirectory)) {
    $OutputDirectory = Join-Path $root $OutputDirectory
}
$sdk = Join-Path $root "nfsdk/sockfilter/bin"
# Stage only. Never register/start drivers or change Windows security settings.
$files = @{
    "nfapi.dll" = Join-Path $sdk "release_c_api/x64/nfapi.dll"
    "driver/x64/sockfilter.sys" = Join-Path $sdk "driver/x64/sockfilter.sys"
    "license.rtf" = Join-Path $root "nfsdk/license.rtf"
}
foreach ($source in $files.Values) {
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
        Write-Warning "SockFilter assets unavailable ($source). Normal proxy build remains usable; process proxy requires the SDK runtime."
        exit 0
    }
}
# Internal documentation is optional in the public source distribution.
$driverReadme = Join-Path $root "README.sockfilter.md"
if (Test-Path -LiteralPath $driverReadme -PathType Leaf) {
    $files["README.sockfilter.md"] = $driverReadme
}
foreach ($relative in $files.Keys) {
    $destination = Join-Path $OutputDirectory $relative
    New-Item -ItemType Directory -Force -Path (Split-Path $destination) | Out-Null
    Copy-Item -LiteralPath $files[$relative] -Destination $destination -Force
}
Write-Host "SockFilter x64 runtime staged to $OutputDirectory (driver NOT installed)."
