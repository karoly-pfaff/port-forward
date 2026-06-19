# macOS LaunchAgent Install

Portier runs as a user-level LaunchAgent on macOS. No `sudo` is required. The agent starts automatically when you log in and restarts if it crashes.

No `.app` bundle or Homebrew formula is included at this release. Install is script-based.

---

## Default Paths

| Item | Path |
|---|---|
| Install directory | `~/Applications/Portier` |
| Config file | `~/Library/Application Support/Portier/rules.json` |
| LaunchAgent plist | `~/Library/LaunchAgents/com.portier.port-forwarding.plist` |
| Stdout log | `~/Library/Logs/Portier/portier.out.log` |
| Stderr log | `~/Library/Logs/Portier/portier.err.log` |
| Management UI | `http://127.0.0.1:47831` |

All paths are configurable via install script parameters. `~` is expanded to the absolute `$HOME` path at install time — launchd does not expand `~` in plist values.

---

## Build and Package

Cross-platform generic package (validates on any OS):

```bash
npm run build:runtime            # builds build/portier/
npm run validate:runtime:smoke   # build, validate layout, and smoke-test
```

macOS-specific package (produces build/macos/ with darwin/amd64 binary):

```bash
npm run build:runtime:macos
```

Or build manually with:

```bash
npm run build
npm run build:service   # cross-compiles Go for current platform
```

The native `.pkg` (pkgbuild) and portable tar.gz are built and validated on a
`macos-latest` runner by the **Release MacOS** workflow
(`.github/workflows/release-macos.yml`, manual `workflow_dispatch`), which also
introspects the `.pkg` payload with `pkgutil --payload-files` and uploads
`build/releases/macos/**` (`.pkg`, portable tar.gz, `checksums.sha256`) as a workflow
artifact. The `.pkg` is unsigned. No GitHub Release or tag is created.

---

## Install (Quick Start)

Build the package and install in one step:

```bash
npm run build:runtime
bash scripts/macos/service/install-launch-agent.sh
```

The install script auto-copies `build/portier/` into `~/Applications/Portier/`, creates the config directory and `rules.json` if missing, generates the LaunchAgent plist with absolute paths, and bootstraps the agent for the current user. No `sudo` is required.

Open `http://127.0.0.1:47831` after installation.

---

## Install Script Options

```bash
bash scripts/macos/service/install-launch-agent.sh [OPTIONS]
```

| Option | Default | Description |
|---|---|---|
| `--source-dir <dir>` | `build/portier/` (auto-detected) | Copy runtime files from this directory |
| `--install-dir <dir>` | `~/Applications/Portier` | Target install directory |
| `--config-path <path>` | `~/Library/Application Support/Portier/rules.json` | Config file path |
| `--host <host>` | `127.0.0.1` | Management API bind address |
| `--port <port>` | `47831` | Management API port |
| `--runtime service\|node` | `service` | Use native service binary or Node.js fallback |
| `--node-mode` | — | Alias for `--runtime node` |
| `--no-start` | — | Generate plist but do not load the agent |
| `--static-dir <dir>` | `<install-dir>/web` | Web UI directory override |

If `build/portier/` exists at the time the script runs, it is copied automatically. Pass `--source-dir ""` (or remove `build/portier/`) to skip the copy and validate an existing install directory instead.

---

## Native Service Mode (Preferred)

The install script uses the native service binary by default. If `build/portier/` is present, it copies automatically:

```bash
bash scripts/macos/service/install-launch-agent.sh
```

With a custom install location:

```bash
bash scripts/macos/service/install-launch-agent.sh \
  --install-dir ~/Applications/Portier \
  --config-path ~/Library/Application\ Support/Portier/rules.json
```

---

## Node.js Fallback Mode

If the native binary is not available, deploy `server.js` and run with Node.js. This requires Node.js installed on the machine.

```bash
bash scripts/macos/service/install-launch-agent.sh --runtime node
```

The installer resolves the Node.js binary path at install time (using `which node` and common Homebrew locations) and writes the absolute path into the plist. This ensures the LaunchAgent can find Node.js even though launchd starts with a minimal `PATH`.

---

## Start, Stop, Status, Uninstall

```bash
# Status (and diagnostic output from launchctl)
bash scripts/macos/service/status-launch-agent.sh

# Stop (unloads the agent — prevents auto-restart)
bash scripts/macos/service/stop-launch-agent.sh

# Start (or restart) the agent
bash scripts/macos/service/start-launch-agent.sh

# Uninstall: stops agent and removes plist; preserves rules.json and logs
bash scripts/macos/service/uninstall-launch-agent.sh

# Uninstall and remove all config and logs (destructive)
bash scripts/macos/service/uninstall-launch-agent.sh --purge
```

---

## Logs

```bash
# Live stdout
tail -f ~/Library/Logs/Portier/portier.out.log

# Live stderr (startup errors, crashes)
tail -f ~/Library/Logs/Portier/portier.err.log
```

Logs persist across restarts and are not rotated automatically. Remove them manually or use `uninstall-launch-agent.sh --purge` to clean everything.

---

## Config

Rules are stored at `~/Library/Application Support/Portier/rules.json`. This path is external to the install directory and is **not** deleted by `uninstall-launch-agent.sh` (only by `--purge`). Back it up or copy it before reinstalling.

---

## Management UI

Open `http://127.0.0.1:47831` in a browser after the agent starts. The management UI binds to `127.0.0.1` by default and is not accessible from the LAN.

To change the bind address or port, pass `--host` and `--port` to `install-launch-agent.sh`. These values are written into the plist at install time.

---

## Release Artifacts

macOS release builds two artifacts under `build/releases/macos/`:

- `portier-portable-macos-<version>.tar.gz` — the portable archive (universal baseline).
- `Portier-<version>.pkg` — the native installer (built on macOS when `pkgbuild` is
  available).

```bash
npm run build:release:current            # tar.gz + .pkg (on macOS)
npm run build:release:portable           # tar.gz only (skip the .pkg)
npm run build:release:current -- --no-build   # reuse an existing build/portier/
```

Both contain the full runtime layout:

```text
portier
service
server.js
web/
  index.html
  assets/
api/
  openapi.json
readme.txt
```

Extract and install from the portable archive on a target machine:

```bash
tar -xzf portier-portable-macos-<version>.tar.gz -C ~/Applications/Portier/
bash scripts/macos/service/install-launch-agent.sh --source-dir ~/Applications/Portier
```

The macOS **portable tar.gz** can be **cross-built from another host** (e.g. Windows), since
the Go binaries are pure Go (`darwin/amd64`):

```bash
npm run build:release:macos            # cross-compile darwin/amd64 + package tar.gz
npm run validate:release:portable:all  # structural validation (no native runtime smoke)
```

Cross-built validation is structural only — native runtime smoke must run on macOS. The
native **`.pkg`** still requires macOS (`pkgbuild`).

### Native .pkg installer

The `.pkg` (built by `scripts/macos/release/build-release.sh` via `pkgbuild`) is the v1.18
macOS native installer track. Current status — a **file-install** package:

- Installs the runtime layout to **`/usr/local/portier`** and bundles the canonical
  LaunchAgent scripts under `/usr/local/portier/service-scripts/` (the runtime binary is
  named `service`, so the scripts use a `service-scripts/` subdir).
- Does **not** auto-install or start the LaunchAgent yet — run
  `service-scripts/install-launch-agent.sh` (LaunchAgent setup via the `.pkg` is a follow-up).
- **Never** creates, overwrites, or migrates `rules.json`, and does not touch
  `~/Library/Application Support/Portier` or logs. Config/data stay external.
- **Unsigned / not notarized.** Gatekeeper will warn on download; sign + notarize for public
  distribution (see below). Install requires admin (`sudo installer -pkg Portier-<version>.pkg
  -target /`).

The `.pkg` is included in `checksums.sha256` and reported by `npm run validate:release:current` on
macOS. Payload introspection (`pkgutil --payload-files`) and a `.pkg` install smoke are the
next macOS validation follow-ups.

---

## macOS Firewall

Forwarded listen ports are separate from the management UI. If a rule listens on a LAN-visible address such as `0.0.0.0`, macOS Firewall may block inbound traffic to that port. macOS may prompt you to allow or block the Portier process on first use. If a forwarding rule appears to start but remote machines cannot connect, check:

- System Settings → Privacy & Security → Firewall → Firewall Options
- The rule's `listenHost` in the Portier UI — confirm it is `0.0.0.0`, not `127.0.0.1`

---

## Signing and Notarization

### Local development builds

Unsigned builds are fully supported for local testing. macOS Gatekeeper may quarantine downloaded binaries. To clear the quarantine flag:

```bash
xattr -d com.apple.quarantine ~/Applications/Portier/service
# or allow all attributes:
xattr -cr ~/Applications/Portier/
```

Alternatively, open Finder → right-click → Open on the binary the first time.

### Public distribution

For public distribution without user intervention, the `service` binary and any installer must be:

1. **Signed** with a Developer ID Application certificate (`codesign --sign "Developer ID Application: ..."`)
2. **Notarized** via Apple's notarization service (`xcrun notarytool submit ...`)
3. **Stapled** so Gatekeeper can verify offline (`xcrun stapler staple ...`)

### Credential requirements

Developer ID signing requires an active Apple Developer Program membership and a Developer ID certificate in the macOS Keychain. These credentials should **never** be committed to the repository. Use environment variables or a CI secrets store when building in automated pipelines.

### Not implemented in v1.1

Signing and notarization are documented here as the required path for public distribution. They are not automated in v1.1. The build scripts produce unsigned local artifacts suitable for development and internal testing. Signing automation is planned for a future release.

---

## Direct launchctl Reference

```bash
# Show full service state
launchctl print gui/$(id -u)/com.portier.port-forwarding

# Bootstrap manually (equivalent to install script step)
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.portier.port-forwarding.plist

# Stop and remove
launchctl bootout gui/$(id -u)/com.portier.port-forwarding
```

---

## Plist Template

`scripts/macos/service/com.portier.plist.example` is a hand-edited reference plist. Use the install script instead for normal installs, as the script expands `$HOME` to an absolute path and sets all flags from parameters.

---

## Automated LaunchAgent Validation

The LaunchAgent install flow is validated by an explicit script that uses a test-specific label, port, and temp directory. It never touches your real Portier LaunchAgent.

```bash
npm run validate:service:macos
# or
bash scripts/macos/service/validate-launch-agent.sh
```

The script:
- Builds `build/portier/` via `npm run build:runtime` (pass `--no-build` to skip)
- Installs the test LaunchAgent (`com.portier.test`), polls `/api/health`, verifies the web UI
- Stops and removes the test LaunchAgent, verifies it is unloaded
- Cleans up all temp files

Pass `--keep-files` to preserve temp directories on failure for debugging.
Pass `--port <number>` to override the auto-detected free port.

---

## Not Included

- macOS `.app` bundle — not implemented
- Homebrew formula — deferred to a future release
- System-level LaunchDaemon — documented as future work; requires `sudo` and a dedicated service user
- Auto-update

The native `.pkg` installer track has started (see "Native .pkg installer" above): a
file-install `.pkg` is built on macOS. LaunchAgent auto-install from the `.pkg`, signing/
notarization, and a `.pkg` install smoke are follow-ups.
