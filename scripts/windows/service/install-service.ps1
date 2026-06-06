param(
  [ValidateSet("Machine", "User")]
  [string]$Scope = "Machine",
  [string]$InstallDir,
  [string]$ConfigDir,
  [string]$HostAddress = "127.0.0.1",
  [int]$Port = 47831,
  [string]$StaticDir,
  [string]$ServiceName = "Portier",
  [string]$DisplayName = "Portier Port Forwarding",
  [string]$Description = "TCP/UDP port forwarding service for local development.",
  [switch]$UseNode,
  [string]$NodePath = "node"
)

$ErrorActionPreference = "Stop"

function Test-IsAdministrator {
  $p = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
  return $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Format-Argument {
  param([string]$Value)
  return '"' + ($Value -replace '"', '\"') + '"'
}

function Get-InstallPaths {
  param([string]$Scope, [string]$InstallDir, [string]$ConfigDir, [string]$StaticDir)

  $defaultInstall = if ($Scope -eq "Machine") {
    Join-Path $env:ProgramFiles "Portier"
  } else {
    Join-Path $env:LOCALAPPDATA "Portier"
  }

  $defaultConfig = if ($Scope -eq "Machine") {
    Join-Path $env:ProgramData "Portier"
  } else {
    Join-Path $env:APPDATA "Portier"
  }

  $resolvedInstall = [System.IO.Path]::GetFullPath($(if ($InstallDir) { $InstallDir } else { $defaultInstall }))
  $resolvedConfig  = [System.IO.Path]::GetFullPath($(if ($ConfigDir) { $ConfigDir } else { $defaultConfig }))
  $resolvedStatic  = if ($StaticDir) {
    [System.IO.Path]::GetFullPath($StaticDir)
  } else {
    Join-Path $resolvedInstall "web"
  }

  return [PSCustomObject]@{
    InstallDir = $resolvedInstall
    ConfigDir  = $resolvedConfig
    ConfigPath = Join-Path $resolvedConfig "rules.json"
    LogsDir    = Join-Path $resolvedConfig "logs"
    StaticDir  = $resolvedStatic
  }
}

function New-ServiceArguments {
  param([PSCustomObject]$Paths, [string]$HostAddress, [int]$Port)
  return "--service --config $(Format-Argument $Paths.ConfigPath) --host $HostAddress --port $Port --static-dir $(Format-Argument $Paths.StaticDir)"
}

if ($Scope -eq "Machine" -and -not (Test-IsAdministrator)) {
  throw "Machine-scope install requires Administrator. Re-run as Administrator, or use -Scope User for a per-user install."
}

$paths = Get-InstallPaths -Scope $Scope -InstallDir $InstallDir -ConfigDir $ConfigDir -StaticDir $StaticDir

if ($UseNode) {
  $binary    = $NodePath
  $serverJs  = Join-Path $paths.InstallDir "server.js"
  if (-not (Test-Path $serverJs)) {
    throw "Node fallback expected server.js at '$serverJs'. Run 'npm run build:native:windows' first."
  }
  $serviceArgs = "$(Format-Argument $serverJs) $(New-ServiceArguments -Paths $paths -HostAddress $HostAddress -Port $Port)"
} else {
  $binary = Join-Path $paths.InstallDir "service.exe"
  if (-not (Test-Path $binary)) {
    throw "Go service binary not found at '$binary'. Run 'npm run build:native:windows' first, or use -UseNode for Node.js fallback."
  }
  $serviceArgs = New-ServiceArguments -Paths $paths -HostAddress $HostAddress -Port $Port
}

New-Item -ItemType Directory -Force -Path $paths.InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $paths.ConfigDir  | Out-Null
New-Item -ItemType Directory -Force -Path $paths.LogsDir    | Out-Null

if (-not (Test-Path $paths.StaticDir)) {
  Write-Warning "Static dir '$($paths.StaticDir)' not found. Web UI will be unavailable until web\ is present in the install directory."
}

if ($Scope -eq "Machine") {
  if (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) {
    throw "Windows service '$ServiceName' already exists. Run uninstall-service.ps1 before reinstalling."
  }

  $binaryPathName = "$(Format-Argument $binary) $serviceArgs"
  New-Service `
    -Name        $ServiceName `
    -DisplayName $DisplayName `
    -Description $Description `
    -BinaryPathName $binaryPathName `
    -StartupType Automatic | Out-Null

  Start-Service -Name $ServiceName
  Write-Host "Installed and started Windows service '$ServiceName'."
} else {
  if (Get-ScheduledTask -TaskName $ServiceName -ErrorAction SilentlyContinue) {
    throw "Scheduled task '$ServiceName' already exists. Run uninstall-service.ps1 -Scope User before reinstalling."
  }

  $action   = New-ScheduledTaskAction -Execute $binary -Argument $serviceArgs -WorkingDirectory $paths.InstallDir
  $trigger  = New-ScheduledTaskTrigger -AtLogon
  $settings = New-ScheduledTaskSettingsSet `
    -ExecutionTimeLimit ([TimeSpan]::Zero) `
    -RestartCount 3 `
    -RestartInterval (New-TimeSpan -Minutes 1) `
    -MultipleInstances IgnoreNew

  Register-ScheduledTask `
    -TaskName    $ServiceName `
    -Action      $action `
    -Trigger     $trigger `
    -Settings    $settings `
    -Description $Description | Out-Null

  Start-ScheduledTask -TaskName $ServiceName
  Write-Host "Registered and started scheduled task '$ServiceName' (auto-starts at user logon)."
}

Write-Host ""
Write-Host "Scope      : $Scope"
Write-Host "InstallDir : $($paths.InstallDir)"
Write-Host "ConfigPath : $($paths.ConfigPath)"
Write-Host "LogsDir    : $($paths.LogsDir)"
Write-Host "StaticDir  : $($paths.StaticDir)"
Write-Host "UI         : http://${HostAddress}:${Port}"
