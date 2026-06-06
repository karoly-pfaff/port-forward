# macOS LaunchAgent Install

Portier runs as a user-level LaunchAgent on macOS. No `sudo` is required. The agent starts automatically when you log in and restarts if it crashes.

No `.app` bundle or Homebrew formula is included at this release. Install is manual: copy the files and run `scripts/macos/install-launch-agent.sh`.

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
npm run package:portier          # builds build/portier/
npm run validate:package:smoke   # build, validate layout, and smoke-test
```

macOS-specific package (produces build/macos/):

```bash
npm install
bash scripts/macos/build-package.sh
```

The package is created under `build/macos/`:

```text
build/macos/
  service        (Go binary, darwin/amd64)
  server.js      (Node/TypeScript server bundle)
  web/
    index.html
    assets/
  readme.txt
```

Or build manually with:

```bash
npm run build
npm run build:service   # cross-compiles Go for current platform
```

---

## Go Service Mode (Preferred)

If a `service` binary is available (cross-compiled or built on macOS), copy it to the install directory:

```bash
mkdir -p ~/Applications/Portier
cp build/macos/service ~/Applications/Portier/service
chmod +x ~/Applications/Portier/service
cp -r build/macos/web ~/Applications/Portier/web
```

Then install the LaunchAgent:

```bash
bash scripts/macos/install-launch-agent.sh \
  --install-dir ~/Applications/Portier \
  --config-path ~/Library/Application\ Support/Portier/rules.json
```

The installer validates the binary, creates the config directory, writes `rules.json` if missing, generates the plist with absolute paths, and bootstraps the LaunchAgent.

---

## Node.js Fallback Mode

If no packaged binary is available, deploy `server.js` and run with Node.js. This requires Node.js installed on the machine.

```bash
mkdir -p ~/Applications/Portier
cp build/macos/server.js ~/Applications/Portier/server.js
cp -r build/macos/web ~/Applications/Portier/web
```

Install with `--node-mode`:

```bash
bash scripts/macos/install-launch-agent.sh \
  --install-dir ~/Applications/Portier \
  --config-path ~/Library/Application\ Support/Portier/rules.json \
  --node-mode
```

The installer resolves the Node.js binary path at install time (using `which node` and common Homebrew locations) and writes the absolute path into the plist. This ensures the LaunchAgent can find Node.js even though launchd starts with a minimal `PATH`.

---

## Start, Stop, Status, Uninstall

```bash
# Check whether the LaunchAgent is loaded and running
bash scripts/macos/status-launch-agent.sh

# Stop the LaunchAgent (unloads it — prevents auto-restart)
bash scripts/macos/stop-launch-agent.sh

# Start (or restart) the LaunchAgent
bash scripts/macos/start-launch-agent.sh

# Uninstall: stops agent and removes plist; preserves rules.json
bash scripts/macos/uninstall-launch-agent.sh
```

---

## Logs

```bash
# Live stdout
tail -f ~/Library/Logs/Portier/portier.out.log

# Live stderr (startup errors, crashes)
tail -f ~/Library/Logs/Portier/portier.err.log
```

Logs persist across restarts and are not rotated automatically. Remove them manually if they grow large.

---

## Config

Rules are stored at `~/Library/Application Support/Portier/rules.json`. This path is external to the install directory and is **not** deleted by `uninstall-launch-agent.sh`. Back it up or copy it before reinstalling.

---

## Management UI

Open `http://127.0.0.1:47831` in a browser after the agent starts. The management UI binds to `127.0.0.1` by default and is not accessible from the LAN.

To change the bind address or port, pass `--host` and `--port` to `install-launch-agent.sh`. These values are written into the plist at install time.

---

## macOS Firewall

Forwarded listen ports are separate from the management UI. If a rule listens on a LAN-visible address such as `0.0.0.0`, macOS Firewall may block inbound traffic to that port. macOS may prompt you to allow or block the Portier process on first use. If a forwarding rule appears to start but remote machines cannot connect, check:

- System Settings → Privacy & Security → Firewall → Firewall Options
- The rule's `listenHost` in the Portier UI — confirm it is `0.0.0.0`, not `127.0.0.1`

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

`deploy/macos/com.portier.plist.example` is a hand-edited reference plist. Use the install script instead for normal installs, as the script expands `$HOME` to an absolute path.

---

## Automated LaunchAgent Validation

The LaunchAgent install flow is validated by an explicit script that uses a test-specific label, port, and temp directory. It never touches your real Portier LaunchAgent.

```bash
npm run validate:service:macos
# or
bash scripts/macos/validate-launch-agent.sh
```

The script:
- Builds `build/portier/` via `npm run package:portier` (pass `--no-build` to skip)
- Installs the test LaunchAgent (`com.portier.test`), polls `/api/health`, verifies the web UI
- Stops and removes the test LaunchAgent, verifies it is unloaded
- Cleans up all temp files

Pass `--keep-files` to preserve temp directories on failure for debugging.
Pass `--port <number>` to override the auto-detected free port.

## Not Included

- macOS `.app` bundle — not implemented yet
- Homebrew formula — deferred to a future release
- System-level LaunchDaemon — documented as future work; requires `sudo` and a dedicated service user
- Auto-update
