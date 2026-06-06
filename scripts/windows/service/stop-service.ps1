param(
  [ValidateSet("Machine", "User")]
  [string]$Scope = "Machine",
  [string]$ServiceName = "Portier"
)

$ErrorActionPreference = "Stop"

if ($Scope -eq "Machine") {
  if (-not (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue)) {
    throw "Windows service '$ServiceName' not found."
  }
  Stop-Service -Name $ServiceName -Force
  Write-Host "Service '$ServiceName' stopped."
} else {
  if (-not (Get-ScheduledTask -TaskName $ServiceName -ErrorAction SilentlyContinue)) {
    throw "Scheduled task '$ServiceName' not found."
  }
  Stop-ScheduledTask -TaskName $ServiceName
  Write-Host "Scheduled task '$ServiceName' stopped."
}
