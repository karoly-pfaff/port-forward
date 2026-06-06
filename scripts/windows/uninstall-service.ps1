param(
  [ValidateSet("Machine", "User")]
  [string]$Scope = "Machine",
  [string]$ServiceName = "Portier",
  [string]$InstallDir,
  [string]$ConfigDir,
  [switch]$RemoveConfig
)

$ErrorActionPreference = "Stop"

function Test-IsAdministrator {
  $p = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
  return $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-DefaultInstallDir {
  param([string]$Scope)
  if ($Scope -eq "Machine") { Join-Path $env:ProgramFiles "Portier" }
  else { Join-Path $env:LOCALAPPDATA "Portier" }
}

function Get-DefaultConfigDir {
  param([string]$Scope)
  if ($Scope -eq "Machine") { Join-Path $env:ProgramData "Portier" }
  else { Join-Path $env:APPDATA "Portier" }
}

if ($Scope -eq "Machine" -and -not (Test-IsAdministrator)) {
  throw "Machine-scope uninstall requires Administrator."
}

if ($Scope -eq "Machine") {
  $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
  if (-not $svc) {
    Write-Warning "Windows service '$ServiceName' not found. Skipping service removal."
  } else {
    if ($svc.Status -ne "Stopped") {
      Write-Host "Stopping service '$ServiceName'..."
      Stop-Service -Name $ServiceName -Force
    }
    Write-Host "Removing service '$ServiceName'..."
    $result = sc.exe delete $ServiceName
    if ($LASTEXITCODE -ne 0) {
      throw "sc.exe delete failed (exit $LASTEXITCODE): $result"
    }
    Write-Host "Windows service '$ServiceName' removed."
  }
} else {
  $task = Get-ScheduledTask -TaskName $ServiceName -ErrorAction SilentlyContinue
  if (-not $task) {
    Write-Warning "Scheduled task '$ServiceName' not found. Skipping task removal."
  } else {
    Stop-ScheduledTask -TaskName $ServiceName -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $ServiceName -Confirm:$false
    Write-Host "Scheduled task '$ServiceName' removed."
  }
}

$resolvedInstallDir = if ($InstallDir) {
  [System.IO.Path]::GetFullPath($InstallDir)
} else {
  Get-DefaultInstallDir -Scope $Scope
}

$resolvedConfigDir = if ($ConfigDir) {
  [System.IO.Path]::GetFullPath($ConfigDir)
} else {
  Get-DefaultConfigDir -Scope $Scope
}

if (Test-Path $resolvedInstallDir) {
  Write-Host "Removing install directory '$resolvedInstallDir'..."
  Remove-Item -LiteralPath $resolvedInstallDir -Recurse -Force
  Write-Host "Install directory removed."
}

if ($RemoveConfig) {
  if (Test-Path $resolvedConfigDir) {
    Write-Host "Removing config directory '$resolvedConfigDir'..."
    Remove-Item -LiteralPath $resolvedConfigDir -Recurse -Force
    Write-Host "Config directory removed."
  }
} else {
  Write-Host "Config preserved: $resolvedConfigDir"
  Write-Host "  Pass -RemoveConfig to also delete rules.json and logs."
}
