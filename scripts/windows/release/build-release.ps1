<#
.SYNOPSIS
  Builds the Portier Windows installer using Inno Setup.

.DESCRIPTION
  Reads the version from package.json (or accepts -Version), runs
  npm run package:portier to produce build/portier/, then calls ISCC.exe
  to produce build/releases/windows/Portier-Setup-<version>.exe.

  Inno Setup 6 must be installed before running this script.
  Download: https://jrsoftware.org/isinfo.php

.PARAMETER Version
  Installer version string (e.g., "1.1.0"). Defaults to the version in
  package.json.

.PARAMETER NoPackage
  Skip npm run package:portier and use the existing build/portier/ directory.
  Useful when build/portier/ is already up to date.

.PARAMETER InnoPath
  Full path to ISCC.exe. If omitted, the script searches PATH and the
  default Inno Setup 6 install location.

.EXAMPLE
  # Full build with version from package.json
  powershell -ExecutionPolicy Bypass -File build-release.ps1

.EXAMPLE
  # Skip package step, use existing build/portier/
  powershell -ExecutionPolicy Bypass -File build-release.ps1 -NoPackage

.EXAMPLE
  # Explicit version and Inno Setup path
  powershell -ExecutionPolicy Bypass -File build-release.ps1 -Version 1.1.0 -InnoPath "C:\Program Files (x86)\Inno Setup 6\ISCC.exe"
#>
param(
  [string]$Version,
  [switch]$NoPackage,
  [string]$InnoPath
)

$ErrorActionPreference = "Stop"

function Log   { param([string]$m) Write-Host "[release:windows] $m" }
function Fail  { param([string]$m) Write-Error "[release:windows] $m"; exit 1 }

# ── Locate the repo root (two directories above this script) ──────────────────
$ScriptDir = $PSScriptRoot
$RepoRoot  = Split-Path (Split-Path (Split-Path $ScriptDir -Parent) -Parent) -Parent

# ── Resolve version ───────────────────────────────────────────────────────────
if (-not $Version) {
  $pkgJsonPath = Join-Path $RepoRoot "package.json"
  if (-not (Test-Path $pkgJsonPath)) { Fail "package.json not found at $pkgJsonPath" }
  $pkg     = Get-Content $pkgJsonPath -Raw | ConvertFrom-Json
  $Version = $pkg.version
  if (-not $Version) { Fail "version field missing in package.json" }
}

# ── Locate ISCC.exe ───────────────────────────────────────────────────────────
$iscc = $null
if ($InnoPath) {
  if (-not (Test-Path $InnoPath)) { Fail "ISCC.exe not found at: $InnoPath" }
  $iscc = $InnoPath
} else {
  # Check PATH first
  $cmd = Get-Command "ISCC.exe" -ErrorAction SilentlyContinue
  if ($cmd) {
    $iscc = $cmd.Source
  } else {
    # Common Inno Setup 6 install locations
    foreach ($candidate in @(
      "C:\Program Files (x86)\Inno Setup 6\ISCC.exe",
      "C:\Program Files\Inno Setup 6\ISCC.exe"
    )) {
      if (Test-Path $candidate) { $iscc = $candidate; break }
    }
  }
}

if (-not $iscc) {
  Write-Host ""
  Write-Host "[release:windows] Inno Setup (ISCC.exe) not found." -ForegroundColor Yellow
  Write-Host "  Install Inno Setup 6 from https://jrsoftware.org/isinfo.php"
  Write-Host "  Then re-run, or pass -InnoPath 'C:\path\to\ISCC.exe'"
  Write-Host ""
  exit 1
}

Log "Inno Setup : $iscc"
Log "Version    : $Version"
Log "Repo root  : $RepoRoot"
Log ""

# ── Build package (unless skipped) ───────────────────────────────────────────
if (-not $NoPackage) {
  Log "Running npm run package:portier..."
  $npm = if (Get-Command "npm.cmd" -ErrorAction SilentlyContinue) { "npm.cmd" } else { "npm" }
  Push-Location $RepoRoot
  try {
    & $npm run package:portier
    if ($LASTEXITCODE -ne 0) { Fail "npm run package:portier failed (exit $LASTEXITCODE)." }
  } finally {
    Pop-Location
  }
  Log ""
} else {
  Log "Skipping package build (-NoPackage)."
}

# ── Verify package contents ───────────────────────────────────────────────────
$PackageDir = Join-Path $RepoRoot "build\portier"

foreach ($rel in @("service.exe", "server.js", "web\index.html", "readme.txt")) {
  $p = Join-Path $PackageDir $rel
  if (-not (Test-Path $p)) {
    Write-Host ""
    Write-Host "[release:windows] Required file missing from build/portier/: $rel" -ForegroundColor Red
    Write-Host "  Run: npm run package:portier"
    Write-Host "  Or pass -NoPackage if the directory is already built."
    exit 1
  }
}

Log "Package verified: build/portier/"

# ── Prepare output directory ──────────────────────────────────────────────────
$OutputDir = Join-Path $RepoRoot "build\releases\windows"
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

# ── Run ISCC.exe ──────────────────────────────────────────────────────────────
$IssFile = Join-Path $ScriptDir "portier.iss"

Log "Building installer (ISCC)..."
Log ""

& $iscc $IssFile `
  "/DAppVersion=$Version" `
  "/DSourceDir=$PackageDir" `
  "/DOutputDir=$OutputDir"

if ($LASTEXITCODE -ne 0) {
  Write-Host ""
  Write-Host "[release:windows] ISCC.exe failed (exit $LASTEXITCODE)." -ForegroundColor Red
  exit 1
}

# ── Report ────────────────────────────────────────────────────────────────────
$installerPath = Join-Path $OutputDir "Portier-Setup-$Version.exe"

Write-Host ""
Log "Installer built: $installerPath"
Write-Host ""
Write-Host "  NOTE: This installer is unsigned and may trigger Windows SmartScreen." -ForegroundColor Yellow
Write-Host "  For public distribution, sign with an EV certificate before releasing." -ForegroundColor Yellow
Write-Host ""
Write-Host "  Install layout:"
Write-Host "    Binaries : %ProgramFiles%\Portier\"
Write-Host "    Config   : %ProgramData%\Portier\rules.json"
Write-Host "    Logs     : %ProgramData%\Portier\logs\"
Write-Host ""
