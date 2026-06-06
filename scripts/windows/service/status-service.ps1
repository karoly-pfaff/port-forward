param(
  [ValidateSet("Machine", "User")]
  [string]$Scope = "Machine",
  [string]$ServiceName = "Portier"
)

$ErrorActionPreference = "Stop"

if ($Scope -eq "Machine") {
  $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
  if (-not $svc) {
    Write-Host "Windows service '$ServiceName': not installed."
  } else {
    Write-Host "Windows service '$ServiceName': $($svc.Status)"
    Write-Host "  StartType  : $($svc.StartType)"
    $svcCim = Get-CimInstance Win32_Service -Filter "Name='$ServiceName'" -ErrorAction SilentlyContinue
    if ($svcCim) {
      Write-Host "  PathName   : $($svcCim.PathName)"
    }
  }
} else {
  $task = Get-ScheduledTask -TaskName $ServiceName -ErrorAction SilentlyContinue
  if (-not $task) {
    Write-Host "Scheduled task '$ServiceName': not installed."
  } else {
    $info = Get-ScheduledTaskInfo -TaskName $ServiceName -ErrorAction SilentlyContinue
    Write-Host "Scheduled task '$ServiceName': $($task.State)"
    if ($info) {
      Write-Host "  LastRunTime: $($info.LastRunTime)"
      Write-Host "  NextRunTime: $($info.NextRunTime)"
    }
    $action = $task.Actions[0]
    Write-Host "  Execute    : $($action.Execute)"
    Write-Host "  Arguments  : $($action.Arguments)"
  }
}
