<#
.SYNOPSIS
  Validates Portier Windows user-scope install flow (scheduled task).
  Does not require Administrator. Uses test-specific names and paths.
  Never touches production Portier installs, config, or task names.

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

$TEST_TASK_NAME  = "PortierTestUser"
$TEST_BASE       = Join-Path $env:TEMP "PortierTestUser"
$TEST_INSTALL    = Join-Path $TEST_BASE "install"
$TEST_CONFIG_DIR = Join-Path $TEST_BASE "config"
$TEST_CONFIG     = Join-Path $TEST_CONFIG_DIR "rules.json"
$HOST_ADDR       = "127.0.0.1"

$SCRIPT_DIR  = $PSScriptRoot
$REPO_ROOT   = Split-Path (Split-Path $SCRIPT_DIR -Parent) -Parent
$PACKAGE_DIR = Join-Path (Join-Path $REPO_ROOT "build") "portier"

function Log  { param([string]$m) Write-Host "[validate:user] $m" }
function Pass { param([string]$m) Write-Host "  [PASS] $m" }

function Get-FreePort {
  $l = New-Object System.Net.Sockets.TcpListener([System.Net.IPAddress]::Loopback, 0)
  $l.Start()
  $p = $l.LocalEndpoint.Port
  $l.Stop()
  return $p
}

function Wait-ForHealth {
  param([string]$Url, [int]$TimeoutSec = 25)
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

$testPort  = if ($Port -ne 0) { $Port } else { Get-FreePort }
$healthUrl = "http://${HOST_ADDR}:${testPort}/api/health"
$rootUrl   = "http://${HOST_ADDR}:${testPort}/"
$exitCode  = 0

try {
  Log "Windows user-scope install validation"
  Log "  Task name   : $TEST_TASK_NAME"
  Log "  Install dir : $TEST_INSTALL"
  Log "  Config      : $TEST_CONFIG"
  Log "  Port        : $testPort"
  Log ""

  # Preflight: fail if a leftover test task exists
  if (Get-ScheduledTask -TaskName $TEST_TASK_NAME -ErrorAction SilentlyContinue) {
    $msg = "Scheduled task '$TEST_TASK_NAME' already exists from a previous run. "
    $msg += "Remove it first: Unregister-ScheduledTask -TaskName '$TEST_TASK_NAME' -Confirm:`$false"
    throw $msg
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

  # Register scheduled task with test name and isolated paths
  Log "Registering scheduled task '$TEST_TASK_NAME'..."
  $testBin   = Join-Path $TEST_INSTALL "service.exe"
  $staticDir = Join-Path $TEST_INSTALL "web"
  $taskArgs  = "--service --config `"$TEST_CONFIG`" --host $HOST_ADDR --port $testPort --static-dir `"$staticDir`""
  $action    = New-ScheduledTaskAction -Execute $testBin -Argument $taskArgs -WorkingDirectory $TEST_INSTALL
  $trigger   = New-ScheduledTaskTrigger -AtLogon
  $settings  = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew
  $taskDesc  = "Portier user-scope install validation (test, safe to remove)"
  $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent().Name
  $principal   = New-ScheduledTaskPrincipal -UserId $currentUser -LogonType Interactive -RunLevel Limited
  Register-ScheduledTask -TaskName $TEST_TASK_NAME -Action $action -Trigger $trigger `
    -Settings $settings -Description $taskDesc -Principal $principal | Out-Null
  Pass "Scheduled task '$TEST_TASK_NAME' registered."

  # Start task on demand
  Log "Starting scheduled task..."
  Start-ScheduledTask -TaskName $TEST_TASK_NAME
  Pass "Scheduled task started."

  # Poll /api/health
  Log "Polling $healthUrl (up to 25s)..."
  if (-not (Wait-ForHealth -Url $healthUrl -TimeoutSec 25)) {
    $state = (Get-ScheduledTask -TaskName $TEST_TASK_NAME -ErrorAction SilentlyContinue).State
    $info  = Get-ScheduledTaskInfo -TaskName $TEST_TASK_NAME -ErrorAction SilentlyContinue
    Log "  Diagnostics:"
    Log "    Task state     : $state"
    if ($null -ne $info) {
      Log "    LastRunTime    : $($info.LastRunTime)"
      Log "    LastTaskResult : $($info.LastTaskResult)"
    }
    throw "Service did not respond on $healthUrl within 25s."
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

  # Stop task
  Log "Stopping scheduled task '$TEST_TASK_NAME'..."
  Stop-ScheduledTask -TaskName $TEST_TASK_NAME -ErrorAction SilentlyContinue
  Start-Sleep -Milliseconds 1000

  # Unregister task
  Log "Unregistering scheduled task '$TEST_TASK_NAME'..."
  Unregister-ScheduledTask -TaskName $TEST_TASK_NAME -Confirm:$false
  if (Get-ScheduledTask -TaskName $TEST_TASK_NAME -ErrorAction SilentlyContinue) {
    throw "Scheduled task '$TEST_TASK_NAME' still present after Unregister-ScheduledTask."
  }
  Pass "Scheduled task '$TEST_TASK_NAME' verified removed."

  Log ""
  Log "User-scope install validation PASSED."

} catch {
  $exitCode = 1
  Write-Host ""
  Write-Host "[validate:user] FAILED: $_" -ForegroundColor Red
  if ($_ -match "Access is denied" -or $_ -match "0x80070005") {
    Write-Host ""
    Write-Host "[validate:user] NOTE: 'Access is denied' when creating a scheduled task usually means:" -ForegroundColor Yellow
    Write-Host "  - A domain Group Policy restricts scheduled task creation to administrators on this machine." -ForegroundColor Yellow
    Write-Host "  - The script is running in a non-interactive session without the required token." -ForegroundColor Yellow
    Write-Host "  Run this validation from an interactive PowerShell terminal on a non-domain-restricted machine." -ForegroundColor Yellow
  }
} finally {
  # Always clean up the task regardless of success or failure
  $leftover = Get-ScheduledTask -TaskName $TEST_TASK_NAME -ErrorAction SilentlyContinue
  if ($leftover) {
    Stop-ScheduledTask -TaskName $TEST_TASK_NAME -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $TEST_TASK_NAME -Confirm:$false -ErrorAction SilentlyContinue
    Log "Cleanup: scheduled task '$TEST_TASK_NAME' removed."
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
