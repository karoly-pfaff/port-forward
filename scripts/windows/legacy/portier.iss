; Portier Windows Installer
; Inno Setup 6 required — https://jrsoftware.org/isinfo.php
;
; Build via npm:
;   npm run release:current
;
; Build manually:
;   ISCC portier.iss /DAppVersion=1.0.0 /DSourceDir=..\..\..\build\portier /DOutputDir=..\..\..\build\releases\windows
;
; The build-release.ps1 script always passes absolute paths for SourceDir and
; OutputDir.  The defaults below are fallbacks for manual ISCC invocations from
; this directory.

#ifndef AppVersion
  #define AppVersion "1.0.0"
#endif

#ifndef SourceDir
  #define SourceDir "..\..\..\build\portier"
#endif

#ifndef OutputDir
  #define OutputDir "..\..\..\build\releases\windows"
#endif

#define AppName "Portier"
#define AppPublisher "Portier"
#define ManagementURL "http://127.0.0.1:47831"

; ============================================================
[Setup]
; AppId identifies this product across upgrades.  Do not change it.
AppId={{F3A2B5C1-4D6E-4789-8F2A-1C3D5E7B9F0A}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL={#ManagementURL}
AppSupportURL={#ManagementURL}

; Install to Program Files\Portier by default
DefaultDirName={autopf}\{#AppName}
DefaultGroupName={#AppName}
AllowNoIcons=yes

; Output
OutputDir={#OutputDir}
OutputBaseFilename=Portier-Setup-{#AppVersion}

Compression=lzma2
SolidCompression=yes
WizardStyle=modern

; Machine-wide install (Program Files + Windows Service) requires admin
PrivilegesRequired=admin

; 64-bit Windows 10+ required (service.exe is amd64)
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0

; ============================================================
[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

; ============================================================
[Messages]
FinishedLabel=Setup has finished installing [name] on your computer.%n%nOpen {#ManagementURL} in your browser to manage port forwarding rules.%n%nIf you installed Portier as a Windows Service, it starts automatically at boot.%n%nNote: this installer is unsigned and may trigger Windows SmartScreen. For public distribution, sign with an EV certificate before releasing.

; ============================================================
[Tasks]
Name: "installservice"; \
  Description: "Install Portier as a &Windows Service (starts automatically at boot)"; \
  Flags: checkedonce

; ============================================================
[Files]
; Runtime binaries
Source: "{#SourceDir}\portier.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\service.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\server.js";   DestDir: "{app}"; Flags: ignoreversion

; Web UI (React client)
Source: "{#SourceDir}\web\*"; DestDir: "{app}\web"; Flags: ignoreversion recursesubdirs createallsubdirs

; Documentation shown to the user on the ReadMe page
Source: "{#SourceDir}\readme.txt"; DestDir: "{app}"; Flags: ignoreversion isreadme

; ============================================================
; Config and log directories under ProgramData.
; rules.json is NOT packaged here — it is created empty by [Code] if absent.
; The directories are created so the service can write logs immediately.
[Dirs]
Name: "{commonappdata}\{#AppName}"
Name: "{commonappdata}\{#AppName}\logs"

; ============================================================
[Icons]
; Start Menu: open management UI in the default browser
Name: "{group}\Open {#AppName}"; \
  Filename: "{sys}\rundll32.exe"; \
  Parameters: "url.dll,FileProtocolHandler {#ManagementURL}"; \
  Comment: "Open the Portier port forwarding management interface"

; Start Menu: uninstall
Name: "{group}\{cm:UninstallProgram,{#AppName}}"; Filename: "{uninstallexe}"

; ============================================================
; Remove the logs directory on uninstall (log files, not user data).
; Config directory and rules.json are preserved by default.
[UninstallDelete]
Type: filesandordirs; Name: "{commonappdata}\{#AppName}\logs"

; ============================================================
[Code]

// Stop the service before file copy so service.exe can be overwritten on upgrade.
procedure StopServiceIfRunning;
var
  Script, TmpScript: String;
  ResultCode: Integer;
begin
  TmpScript := ExpandConstant('{tmp}') + '\portier-svc-stop.ps1';
  Script :=
    '$svc = Get-Service -Name "Portier" -ErrorAction SilentlyContinue' + #13#10 +
    'if ($svc -and $svc.Status -ne "Stopped") {' + #13#10 +
    '  Stop-Service -Name "Portier" -Force -ErrorAction SilentlyContinue' + #13#10 +
    '  Start-Sleep -Milliseconds 500' + #13#10 +
    '}';
  SaveStringToFile(TmpScript, Script, False);
  Exec('powershell.exe',
       '-ExecutionPolicy Bypass -NoProfile -NonInteractive -File "' + TmpScript + '"',
       '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  DeleteFile(TmpScript);
end;

// Create an empty rules.json only if one does not already exist.
// The service requires the file to exist at startup.
procedure CreateConfigIfMissing;
var
  ConfigPath: String;
begin
  ConfigPath := ExpandConstant('{commonappdata}') + '\Portier\rules.json';
  if not FileExists(ConfigPath) then
    SaveStringToFile(ConfigPath, '[]', False);
end;

// Register and start the Portier Windows Service.
// Uses a temp PowerShell script to avoid Inno Setup shell quoting limitations
// with paths that contain spaces (e.g., Program Files).
procedure DoInstallService;
var
  AppDir, ConfigPath, WebDir: String;
  Script, TmpScript: String;
  ResultCode: Integer;
begin
  AppDir     := ExpandConstant('{app}');
  ConfigPath := ExpandConstant('{commonappdata}') + '\Portier\rules.json';
  WebDir     := AppDir + '\web';
  TmpScript  := ExpandConstant('{tmp}') + '\portier-svc-install.ps1';

  // Embed the actual paths as PowerShell variable assignments.
  // $binPath uses backtick-escaped quotes so New-Service receives a properly
  // quoted BinaryPathName even when paths contain spaces.
  Script :=
    '$ErrorActionPreference = "Stop"' + #13#10 +
    '$binary = "' + AppDir + '\service.exe"' + #13#10 +
    '$config = "' + ConfigPath + '"' + #13#10 +
    '$web    = "' + WebDir + '"' + #13#10 +
    '$binPath = "`"$binary`" --service --config `"$config`" --host 127.0.0.1 --port 47831 --static-dir `"$web`""' + #13#10 +
    '$existing = Get-Service -Name "Portier" -ErrorAction SilentlyContinue' + #13#10 +
    'if ($existing) { Write-Host "Portier service already exists — skipping registration."; exit 0 }' + #13#10 +
    'New-Service -Name "Portier" -DisplayName "Portier Port Forwarding" -Description "TCP/UDP port forwarding service for local development." -BinaryPathName $binPath -StartupType Automatic' + #13#10 +
    'Start-Service -Name "Portier"';

  SaveStringToFile(TmpScript, Script, False);

  if not Exec('powershell.exe',
              '-ExecutionPolicy Bypass -NoProfile -NonInteractive -File "' + TmpScript + '"',
              '', SW_HIDE, ewWaitUntilTerminated, ResultCode) then begin
    MsgBox('PowerShell could not be launched for Windows Service installation.' + #13#10 +
           'Portier files were installed to ' + AppDir + '.' + #13#10 +
           'See ' + AppDir + '\readme.txt for manual service installation steps.',
           mbInformation, MB_OK);
    DeleteFile(TmpScript);
    exit;
  end;

  DeleteFile(TmpScript);

  if ResultCode <> 0 then
    MsgBox('Windows service registration returned exit code ' + IntToStr(ResultCode) + '.' + #13#10 +
           'Portier files are installed in ' + AppDir + '.' + #13#10 +
           'If the service did not start, see readme.txt for troubleshooting and manual steps.',
           mbInformation, MB_OK)
  else
    MsgBox('Portier Windows Service installed and started.' + #13#10 +
           'Open http://127.0.0.1:47831 to manage port forwarding rules.',
           mbInformation, MB_OK);
end;

// Stop and remove the Portier Windows Service before uninstall.
// Config (rules.json) is preserved — only the service registration is removed.
procedure DoUninstallService;
var
  Script, TmpScript: String;
  ResultCode: Integer;
begin
  TmpScript := ExpandConstant('{tmp}') + '\portier-svc-uninstall.ps1';
  Script :=
    '$svc = Get-Service -Name "Portier" -ErrorAction SilentlyContinue' + #13#10 +
    'if ($svc) {' + #13#10 +
    '  if ($svc.Status -ne "Stopped") { Stop-Service -Name "Portier" -Force -ErrorAction SilentlyContinue }' + #13#10 +
    '  Start-Sleep -Milliseconds 500' + #13#10 +
    '  sc.exe delete "Portier" | Out-Null' + #13#10 +
    '}';
  SaveStringToFile(TmpScript, Script, False);
  Exec('powershell.exe',
       '-ExecutionPolicy Bypass -NoProfile -NonInteractive -File "' + TmpScript + '"',
       '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  DeleteFile(TmpScript);
end;

// Called by Inno Setup before files are copied.
// Stops the running service so service.exe can be overwritten on upgrade.
function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  Result := '';
  StopServiceIfRunning;
end;

// Called after each install step.
procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then begin
    CreateConfigIfMissing;
    if WizardIsTaskSelected('installservice') then
      DoInstallService;
  end;
end;

// Called during uninstall.
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then
    DoUninstallService;
end;
