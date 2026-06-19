# Windows Packaging and Service Install

Portier supports two server runtimes on Windows:

- **Go service** (`service.exe`) — preferred native runtime, no Node.js required.
- **Node server** (`server.js`) — supported fallback runtime, requires Node.js.

Runtime config and the web UI stay external in both modes.

---

## Install Layout

```text
<install-dir>\
  service.exe
  server.js
  web\
    index.html
    assets\
```

---

## Build the Package

Cross-platform generic package (validates on any OS):

```powershell
npm run build:runtime            # builds build/portier/
npm run validate:runtime:smoke   # build, validate layout, and smoke-test
```

Windows-specific package (produces build\windows\):

```powershell
npm run build:runtime:windows
```

Output: `build\windows\` (service.exe, server.js, web\, readme.txt).

Clean and rebuild:

```powershell
npm run build:clean
npm run build:runtime:windows
```

---

## Windows Installer (WiX MSI)

The **WiX MSI is the canonical Windows installer**, built by the WiX Toolset v7
(`scripts/windows/release/`) as part of the Windows release:

```powershell
npm run build:release:current
```

A full Windows release **fails if WiX 7 is unavailable** — use `--portable-only` to build
just the portable zip. Output: `build\releases\windows\Portier-<version>.msi` (included in
`SHA256SUMS`).

**What the MSI installs (file-install):**

| Location | Contents |
|----------|----------|
| `%ProgramFiles%\Portier\` | `portier.exe`, `service.exe`, `server.js`, `web\`, `api\openapi.json`, `readme.txt` |
| `%ProgramFiles%\Portier\service\` | canonical Windows service scripts (`install-service.ps1`, …) |

- The current MSI is **file-install only**: it does **not** auto-install or start the Windows
  Service. Register the service with the bundled `service\install-service.ps1` (machine =
  Windows Service, user = Scheduled Task). Service auto-install via an MSI custom action is a
  follow-up.
- The MSI **never** creates, overwrites, or migrates `rules.json`, and does not touch
  `%ProgramData%\Portier`. Config/data stay external.
- Per-machine install requires admin/UAC; supports silent install
  (`msiexec /i Portier-<version>.msi /qn`) and standard Add/Remove Programs + repair.

**SmartScreen / signing:** the MSI is unsigned. For public distribution, sign it (and
`service.exe`) with an EV certificate.

Validate the MSI install layout and config/data boundary (no admin needed; uses
`msiexec /a` extraction non-elevated, full `/i`+`/x` when elevated):

```powershell
npm run validate:msi:install
```

See [`scripts/windows/release/readme.md`](release/readme.md) for the WiX source and install smoke.

> **Legacy:** the previous Inno Setup installer has been retired from the release flow. Its
> scripts are kept, manual-only, under `scripts/windows/legacy/` and are not built or
> validated. New Windows installer work belongs in the WiX MSI.

### Portable zip (cross-buildable)

The Windows portable `.zip` is built by `build:release:current` (bundled with the MSI). It can
also be produced from **any host** (since the Go binaries are pure Go) via the host-agnostic
portable generator:

```bash
npm run build:release:windows          # cross-compile windows/amd64 + package the zip
npm run validate:release:portable:all  # structural validation (no native runtime smoke)
```

Native runtime smoke must run on Windows (`npm run validate:runtime:smoke`).

---

## Manual Test

Go service:

```powershell
.\build\windows\service.exe --config ".\rules.json" --host 127.0.0.1 --port 47831 --static-dir ".\build\windows\web"
```

Node server (requires Node.js):

```powershell
node .\build\windows\server.js --config ".\rules.json" --host 127.0.0.1 --port 47831 --static-dir ".\build\windows\web"
```

Then open `http://127.0.0.1:47831`.

---

## Machine Install (Administrator required)

Installs to `%ProgramFiles%\Portier`. Config and logs under `%ProgramData%\Portier`. Runs as a Windows Service that starts automatically at boot.

**1. Copy the package:**

```powershell
Copy-Item -Recurse -Force .\build\windows\* "$env:ProgramFiles\Portier\"
```

**2. Install the service (run as Administrator):**

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\windows\service\install-service.ps1
```

Default parameter values when `-Scope Machine`:

| Parameter     | Value                                   |
|---------------|-----------------------------------------|
| `InstallDir`  | `%ProgramFiles%\Portier`                |
| `ConfigDir`   | `%ProgramData%\Portier`                 |
| `ConfigPath`  | `%ProgramData%\Portier\rules.json`      |
| `LogsDir`     | `%ProgramData%\Portier\logs`            |
| `StaticDir`   | `%ProgramFiles%\Portier\web`            |

The installer registers and starts a service with a command line equivalent to:

```text
"C:\Program Files\Portier\service.exe" --service --config "C:\ProgramData\Portier\rules.json" --host 127.0.0.1 --port 47831 --static-dir "C:\Program Files\Portier\web"
```

Node fallback (requires Node.js; provide the full path to `node.exe` for a service):

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\windows\service\install-service.ps1 `
  -UseNode -NodePath "C:\Program Files\nodejs\node.exe"
```

**Manage the service:**

```powershell
.\scripts\windows\service\start-service.ps1
.\scripts\windows\service\stop-service.ps1
.\scripts\windows\service\status-service.ps1
```

Or use standard Windows tools:

```powershell
Start-Service Portier
Stop-Service Portier
Get-Service Portier
```

**Uninstall (preserves rules.json by default):**

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\windows\service\uninstall-service.ps1
```

Pass `-RemoveConfig` to also delete `%ProgramData%\Portier` including `rules.json` and logs.

---

## User Install (no Administrator required)

Installs to `%LOCALAPPDATA%\Portier`. Config and logs under `%APPDATA%\Portier`. Runs as a Windows Task Scheduler task that starts automatically at user logon.

**1. Copy the package:**

```powershell
Copy-Item -Recurse -Force .\build\windows\* "$env:LOCALAPPDATA\Portier\"
```

**2. Install the scheduled task:**

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\windows\service\install-service.ps1 -Scope User
```

Default parameter values when `-Scope User`:

| Parameter     | Value                                   |
|---------------|-----------------------------------------|
| `InstallDir`  | `%LOCALAPPDATA%\Portier`                |
| `ConfigDir`   | `%APPDATA%\Portier`                     |
| `ConfigPath`  | `%APPDATA%\Portier\rules.json`          |
| `LogsDir`     | `%APPDATA%\Portier\logs`                |
| `StaticDir`   | `%LOCALAPPDATA%\Portier\web`            |

The scheduler task runs at user logon with a command equivalent to:

```text
"C:\Users\<user>\AppData\Local\Portier\service.exe" --service --config "C:\Users\<user>\AppData\Roaming\Portier\rules.json" --host 127.0.0.1 --port 47831 --static-dir "C:\Users\<user>\AppData\Local\Portier\web"
```

Node fallback for user install (node must be on PATH or provide full path):

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\windows\service\install-service.ps1 -Scope User `
  -UseNode -NodePath "C:\Program Files\nodejs\node.exe"
```

**Manage the task:**

```powershell
.\scripts\windows\service\start-service.ps1  -Scope User
.\scripts\windows\service\stop-service.ps1   -Scope User
.\scripts\windows\service\status-service.ps1 -Scope User
```

**Uninstall (preserves rules.json by default):**

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\windows\service\uninstall-service.ps1 -Scope User
```

Pass `-RemoveConfig` to also delete `%APPDATA%\Portier` including `rules.json` and logs.

---

## Windows Service Mode

When `service.exe` is started by the Windows Service Control Manager (SCM), it
automatically registers with SCM using `golang.org/x/sys/windows/svc`. It
reports `StartPending`, starts the HTTP/forwarding runtime, then reports
`Running`. On Stop or Shutdown, it signals shutdown, waits for the HTTP server
to drain (up to 10 s), and reports `StopPending` / `Stopped`.

When started from a terminal (not SCM), `service.exe` runs as a normal console
process and responds to Ctrl+C / SIGTERM. The `--service` flag is still
accepted (it is logged and passed by the installer for documentation clarity)
but is not what triggers SCM registration — that is detected automatically via
the Windows API.

**Console mode test** (same arguments the installer uses, run from a terminal):

```powershell
.\build\portier\service.exe --service --config ".\rules.json" --host 127.0.0.1 --port 47831 --static-dir ".\build\portier\web"
```

If this starts and responds on `/api/health`, the binary is functional. Press
Ctrl+C to stop it, then test the Windows Service path.

---

## Troubleshooting Error 1053

> Error 1053: The service did not respond to the start or control request in a
> timely fashion.

**Cause before this fix**: service.exe did not call
`StartServiceCtrlDispatcher`; SCM timed out waiting for registration.

**Cause after this fix**: almost always a startup crash or misconfiguration.
Check the following:

1. **Run service.exe manually** from an elevated terminal with the exact
   command line SCM uses:

   ```powershell
   & "C:\Program Files\Portier\service.exe" --service `
     --config "C:\ProgramData\Portier\rules.json" `
     --host 127.0.0.1 --port 47831 `
     --static-dir "C:\Program Files\Portier\web"
   ```

   If it fails, fix the error reported there. If it succeeds, the binary is
   fine and the issue is SCM-specific (path quoting, permissions, etc.).

2. **Check Windows Event Viewer**:
   - `Event Viewer → Windows Logs → Application`
   - `Event Viewer → Windows Logs → System`
   - Filter for Source = `Portier` or look for error events near the start time.

3. **Query service state**:

   ```powershell
   Get-Service Portier
   sc.exe query Portier
   ```

4. **Check BinaryPathName quoting** with `sc.exe qc Portier`. Paths with
   spaces must be double-quoted. `install-service.ps1` quotes them correctly,
   but manual `sc.exe create` commands often have quoting errors.

5. **Check config path**: `rules.json` must exist at the configured path.
   Create an empty file if needed:

   ```powershell
   New-Item -ItemType File -Force "C:\ProgramData\Portier\rules.json"
   Set-Content "C:\ProgramData\Portier\rules.json" "[]"
   ```

6. **Check port availability**: `127.0.0.1:47831` must be free.

   ```powershell
   netstat -ano | findstr ":47831"
   ```

**Service stdout/stderr**: When running under SCM, stdout/stderr are not
connected to a terminal. The service logs to stderr (structured text via
`slog`). If you need to capture service output, redirect stdout/stderr in the
BinaryPathName or consult Windows Event Log. For diagnostic purposes, running
the same command line from a terminal is the easiest way to see logs.

---

## Direct sc.exe and New-Service Examples

`sc.exe` is a built-in Windows tool, not a download.

Go service via `sc.exe` (Command Prompt, run as Administrator):

```cmd
sc.exe create Portier binPath= "\"C:\Program Files\Portier\service.exe\" --service --config \"C:\ProgramData\Portier\rules.json\" --host 127.0.0.1 --port 47831 --static-dir \"C:\Program Files\Portier\web\"" start= auto DisplayName= "Portier Port Forwarding"
```

Go service via PowerShell `New-Service` (run as Administrator):

```powershell
New-Service `
  -Name "Portier" `
  -DisplayName "Portier Port Forwarding" `
  -Description "TCP/UDP port forwarding service for local development." `
  -BinaryPathName '"C:\Program Files\Portier\service.exe" --service --config "C:\ProgramData\Portier\rules.json" --host 127.0.0.1 --port 47831 --static-dir "C:\Program Files\Portier\web"' `
  -StartupType Automatic
```

---

## Automated Service Install Validation

The user-scope and machine-scope install flows are validated by explicit scripts that use test-specific names, ports, and temp directories. They never touch production Portier installs.

```powershell
# User-scope (scheduled task) — no Administrator required
npm run validate:service:windows:user

# Machine-scope (Windows Service) — Administrator required
npm run validate:service:windows:machine

# Current platform (user-scope on Windows)
npm run validate:service:current
```

Both scripts:
- Build `build/portier/` via `npm run build:runtime` (pass `-NoBuild` to skip)
- Install the test service/task, poll `/api/health`, verify the web UI
- Stop and unregister the test service/task, verify removal
- Clean up all temp files

Pass `-KeepFiles` to preserve temp directories on failure for debugging.
Pass `-Port <number>` to override the auto-detected free port.

## Important Notes

**Do not run both Machine and User installs on the same port.** If a machine-scope Windows Service and a user-scope scheduled task both try to bind `127.0.0.1:47831`, one will fail to start.

**Management UI** defaults to `http://127.0.0.1:47831` and is local-only. No firewall rule needed for the management port.

**Forwarded ports** listening on `0.0.0.0` are LAN-visible and may require Windows Firewall inbound rules. Portier warns when a rule uses `0.0.0.0`.

**Uninstall always preserves `rules.json`** unless `-RemoveConfig` is explicitly passed.

**Node fallback for machine-scope service:** Node.js must be available at the path given to `-NodePath` on the target machine. The system PATH is not reliably available to Windows Services, so always provide a full path to `node.exe`.
