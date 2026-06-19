<#
.SYNOPSIS
  Build the Portier Windows MSI with the WiX Toolset (WiX 7 / v4 schema).

.DESCRIPTION
  Initial v1.18 MSI spike (enterprise/admin track). Packages build\portier\ into
  %ProgramFiles%\Portier and bundles the canonical service scripts. Does not touch
  user config/data. The Inno installer remains the default consumer installer.

  Resolves the `wix` tool from PATH, then from the dotnet global tools location
  (%USERPROFILE%\.dotnet\tools\wix.exe). Exits non-zero (and prints the exact
  error) if WiX is unavailable — callers treat that as a non-fatal skip.

.PARAMETER Version
  Version string for the MSI (default: root package.json version).

.PARAMETER SourceDir
  Packaged runtime dir to harvest (default: <repo>\build\portier).

.PARAMETER OutputDir
  Output dir for the MSI (default: <repo>\build\releases\windows).

.PARAMETER WixPath
  Explicit path to wix(.exe) if not on PATH or in the dotnet tools dir.
#>
param(
  [string]$Version,
  [string]$SourceDir,
  [string]$OutputDir,
  [string]$WixPath
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = (Resolve-Path (Join-Path $scriptDir "..\..\..")).Path

if (-not $Version) {
  $pkg = Get-Content (Join-Path $repoRoot "package.json") -Raw | ConvertFrom-Json
  $Version = $pkg.version
}
if (-not $SourceDir) { $SourceDir = Join-Path $repoRoot "build\portier" }
if (-not $OutputDir) { $OutputDir = Join-Path $repoRoot "build\releases\windows" }

function Resolve-Wix {
  param([string]$Explicit)
  if ($Explicit) { return $Explicit }
  $cmd = Get-Command wix -ErrorAction SilentlyContinue
  if ($cmd) { return $cmd.Source }
  $candidate = Join-Path $env:USERPROFILE ".dotnet\tools\wix.exe"
  if (Test-Path $candidate) { return $candidate }
  return $null
}

$wix = Resolve-Wix -Explicit $WixPath
if (-not $wix) {
  Write-Error "WiX Toolset 'wix' not found on PATH or in %USERPROFILE%\.dotnet\tools. Install with: dotnet tool install --global wix"
  exit 1
}

if (-not (Test-Path $SourceDir)) {
  Write-Error "Packaged runtime dir not found: $SourceDir. Run 'npm run build:runtime' first."
  exit 1
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$wxs = Join-Path $scriptDir "portier.wxs"
$msi = Join-Path $OutputDir "Portier-$Version.msi"

Write-Host "[wix] Tool    : $wix"
Write-Host "[wix] Version : $Version"
Write-Host "[wix] Source  : $SourceDir"
Write-Host "[wix] Output  : $msi"

# "-acceptEula wix7" accepts FireGiant's WiX v7 Open Source Maintenance Fee (OSMF)
# EULA (free for open-source use) so `wix build` runs non-interactively (error
# WIX7015 otherwise). The value is the EULA major-version id. See
# https://docs.firegiant.com/wix/osmf/.
& $wix build $wxs -arch x64 -acceptEula wix7 `
  -d "Version=$Version" `
  -d "PackageDir=$SourceDir" `
  -d "RepoRoot=$repoRoot" `
  -o $msi

if ($LASTEXITCODE -ne 0) {
  Write-Error "wix build failed with exit code $LASTEXITCODE"
  exit $LASTEXITCODE
}

Write-Host "[wix] MSI built: $msi"
