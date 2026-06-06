param(
  [ValidateSet("Machine", "User")]
  [string]$Scope = "Machine",
  [string]$ServiceName = "Portier"
)

$ErrorActionPreference = "Stop"

if ($Scope -eq "Machine") {
  if (-not (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue)) {
    throw "Windows service '$ServiceName' not found. Install it first with install-service.ps1."
  }
  Start-Service -Name $ServiceName
  Write-Host "Service '$ServiceName' started."
} else {
  if (-not (Get-ScheduledTask -TaskName $ServiceName -ErrorAction SilentlyContinue)) {
    throw "Scheduled task '$ServiceName' not found. Install it first with install-service.ps1 -Scope User."
  }
  Start-ScheduledTask -TaskName $ServiceName
  Write-Host "Scheduled task '$ServiceName' started."
}
