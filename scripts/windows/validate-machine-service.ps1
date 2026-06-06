<#
.SYNOPSIS
  Validates Portier Windows machine-scope install flow (Windows Service).
  Requires Administrator. Uses test-specific names and paths.
  Never touches production Portier installs, config, or service name.

.PARAMETER NoBuild
  Skip npm run package:portier and use the existing build/portier/ directory.

.PARAMETER KeepFiles
  Preserve temp directories after validation (useful for debugging failures).

.PARAMETER Port
  Management port for the test service. 0 = auto-detect a free port (default).
#>
param(
  [switch]$NoBuild,
  [switch]$KeepFiles,
  [int]$Port = 0
)

$ErrorActionPreference = "Stop"

$TEST_SERVICE_NAME = "PortierTestMachine"
$TEST_DISPLAY_NAME = "Portier Test Machine Service"
$TEST_BASE         = Join-Path $env:TEMP "PortierTestMachine"
$TEST_INSTALL      = Join-Path $TEST_BASE "install"
$TEST_CONFIG_DIR   = Join-Path $TEST_BASE "config"
$TEST_CONFIG       = Join-Path $TEST_CONFIG_DIR "rules.json"
$HOST_ADDR         = "127.0.0.1"

$SCRIPT_DIR  = $PSScriptRoot
$REPO_ROOT   = Split-Path (Split-Path $SCRIPT_DIR -Parent) -Parent
$PACKAGE_DIR = Join-Path (Join-Path $REPO_ROOT "build") "portier"

function Log  { param([string]$m) Write-Host "[validate:machine] $m" }
function Pass { param([string]$m) Write-Host "  [PASS] $m" }

function Test-IsAdministrator {
  $p = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
  return $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-FreePort {
  $l = New-Object System.Net.Sockets.TcpListener([System.Net.IPAddress]::Loopback, 0)
  $l.Start()
  $p = $l.LocalEndpoint.Port
  $l.Stop()
  return $p
}

function Wait-ForHealth {
  param([string]$Url, [int]$TimeoutSec = 30)
  $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSec)
  while ([DateTime]::UtcNow -lt $deadline) {
    try {
      $r = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 2 -ErrorAction Stop
      if ($r.StatusCode -eq 200) { return $true }
    } catch { }
    Start-Sleep -Milliseconds 500
  }
  return $false
}

# Require Administrator up front
if (-not (Test-IsAdministrator)) {
  Write-Error "[validate:machine] This script requires Administrator. Re-run from an elevated PowerShell prompt."
  exit 1
}

$testPort  = if ($Port -ne 0) { $Port } else { Get-FreePort }
$healthUrl = "http://${HOST_ADDR}:${testPort}/api/health"
$rootUrl   = "http://${HOST_ADDR}:${testPort}/"
$exitCode  = 0

try {
  Log "Windows machine-scope install validation (Administrator)"
  Log "  Service name : $TEST_SERVICE_NAME"
  Log "  Install dir  : $TEST_INSTALL"
  Log "  Config       : $TEST_CONFIG"
  Log "  Port         : $testPort"
  Log ""

  # Preflight: fail if a leftover test service exists
  if (Get-Service -Name $TEST_SERVICE_NAME -ErrorAction SilentlyContinue) {
    throw "Windows service '$TEST_SERVICE_NAME' already exists from a previous run. Remove it: sc.exe delete $TEST_SERVICE_NAME"
  }

  # Build package if requested
  if (-not $NoBuild) {
    Log "Running npm run package:portier..."
    $npm = if (Get-Command "npm.cmd" -ErrorAction SilentlyContinue) { "npm.cmd" } else { "npm" }
    & $npm run package:portier
    if ($LASTEXITCODE -ne 0) { throw "npm run package:portier failed (exit $LASTEXITCODE)." }
    Log ""
  } else {
    Log "Skipping package build (-NoBuild)."
  }

  # Verify package contents
  if (-not (Test-Path (Join-Path $PACKAGE_DIR "service.exe"))) {
    throw "service.exe not found in build/portier/. Run: npm run package:portier"
  }
  if (-not (Test-Path (Join-Path $PACKAGE_DIR "web"))) {
    throw "web/ not found in build/portier/. Run: npm run package:portier"
  }

  # Set up test install dir
  Log "Creating test install directory..."
  New-Item -ItemType Directory -Force -Path $TEST_INSTALL    | Out-Null
  New-Item -ItemType Directory -Force -Path $TEST_CONFIG_DIR | Out-Null
  Get-ChildItem -Path $PACKAGE_DIR | ForEach-Object {
    Copy-Item -Path $_.FullName -Destination $TEST_INSTALL -Recurse -Force
  }
  Set-Content -Path $TEST_CONFIG -Value "[]" -Encoding UTF8
  Pass "Test install directory ready."

  # Register Windows Service with test name and isolated paths
  Log "Creating Windows service '$TEST_SERVICE_NAME'..."
  $testBin   = Join-Path $TEST_INSTALL "service.exe"
  $staticDir = Join-Path $TEST_INSTALL "web"
  $svcDesc   = "Portier machine-scope install validation (test, safe to remove)"

  # BinaryPathName must include the quoted binary and all CLI arguments
  $q = '"'
  $binPath = "$q$testBin$q --service --config $q$TEST_CONFIG$q --host $HOST_ADDR --port $testPort --static-dir $q$staticDir$q"

  New-Service `
    -Name        $TEST_SERVICE_NAME `
    -DisplayName $TEST_DISPLAY_NAME `
    -Description $svcDesc `
    -BinaryPathName $binPath `
    -StartupType Manual | Out-Null
  Pass "Windows service '$TEST_SERVICE_NAME' created."

  # Start service
  Log "Starting service '$TEST_SERVICE_NAME'..."
  Start-Service -Name $TEST_SERVICE_NAME
  Pass "Service started."

  # Poll /api/health
  Log "Polling $healthUrl (up to 30s)..."
  if (-not (Wait-ForHealth -Url $healthUrl -TimeoutSec 30)) {
    $svc = Get-Service -Name $TEST_SERVICE_NAME -ErrorAction SilentlyContinue
    Log "  Diagnostics:"
    Log "    Service status : $($svc.Status)"
    throw "Service did not respond on $healthUrl within 30s."
  }
  Pass "/api/health responded OK."

  # Verify web UI
  Log "Checking web UI at $rootUrl..."
  $ui = Invoke-WebRequest -Uri $rootUrl -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop
  $htmlFound = $ui.Content -match "(?i)html"
  if ($ui.StatusCode -eq 200 -and $htmlFound) {
    Pass "Web UI served at /."
  } else {
    throw "Web UI not served at / (status=$($ui.StatusCode))."
  }

  # Stop service
  Log "Stopping service '$TEST_SERVICE_NAME'..."
  Stop-Service -Name $TEST_SERVICE_NAME -Force -ErrorAction SilentlyContinue
  Start-Sleep -Milliseconds 1000
  Pass "Service stopped."

  # Delete service
  Log "Deleting Windows service '$TEST_SERVICE_NAME'..."
  $scResult = sc.exe delete $TEST_SERVICE_NAME
  if ($LASTEXITCODE -ne 0) {
    throw "sc.exe delete failed (exit $LASTEXITCODE): $scResult"
  }
  Start-Sleep -Milliseconds 500
  if (Get-Service -Name $TEST_SERVICE_NAME -ErrorAction SilentlyContinue) {
    throw "Windows service '$TEST_SERVICE_NAME' still present after sc.exe delete."
  }
  Pass "Windows service '$TEST_SERVICE_NAME' verified removed."

  Log ""
  Log "Machine-scope install validation PASSED."

} catch {
  $exitCode = 1
  Write-Host ""
  Write-Host "[validate:machine] FAILED: $_" -ForegroundColor Red
} finally {
  # Always clean up the service regardless of success or failure
  $leftover = Get-Service -Name $TEST_SERVICE_NAME -ErrorAction SilentlyContinue
  if ($leftover) {
    if ($leftover.Status -ne "Stopped") {
      Stop-Service -Name $TEST_SERVICE_NAME -Force -ErrorAction SilentlyContinue
      Start-Sleep -Milliseconds 500
    }
    sc.exe delete $TEST_SERVICE_NAME 2>&1 | Out-Null
    Log "Cleanup: Windows service '$TEST_SERVICE_NAME' removed."
  }
  if ($KeepFiles) {
    if (Test-Path $TEST_BASE) { Log "Cleanup: -KeepFiles set; temp dir preserved: $TEST_BASE" }
  } else {
    if (Test-Path $TEST_BASE) {
      Remove-Item -LiteralPath $TEST_BASE -Recurse -Force -ErrorAction SilentlyContinue
      Log "Cleanup: temp dir removed."
    }
  }
}

exit $exitCode
