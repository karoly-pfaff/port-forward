# Linux systemd Service Install

Portier can run as a system-level systemd service on Linux.

**Recommended:** use the helper scripts in `scripts/linux/` — they generate and install the unit file automatically.

The `deploy/systemd/` directory contains example unit files for reference and manual installs.

---

## Install Layout

```text
/opt/portier/
  service        (Go binary — preferred runtime)
  server.js      (Node/TypeScript server — fallback runtime)
  web/
    index.html
    assets/

/etc/portier/
  rules.json     (external config — never packaged)
```

---

## Build the Package

Cross-platform generic package (validates on any OS):

```bash
npm run package:portier          # builds build/portier/
npm run validate:package:smoke   # build, validate layout, and smoke-test
```

Linux-specific package (produces build/linux/):

```bash
bash scripts/linux/build-package.sh
```

## Helper Scripts (Recommended)

### Install

Go service (preferred):

```bash
sudo bash scripts/linux/install-service.sh
```

Node fallback (requires Node.js):

```bash
sudo bash scripts/linux/install-service.sh --runtime node
```

With custom paths:

```bash
sudo bash scripts/linux/install-service.sh \
  --install-dir /opt/portier \
  --config-path /etc/portier/rules.json \
  --host 127.0.0.1 \
  --port 47831
```

Supported flags:

| Flag             | Default                       | Description                              |
|------------------|-------------------------------|------------------------------------------|
| `--install-dir`  | `/opt/portier`                | Directory containing binaries and web/   |
| `--config-path`  | `/etc/portier/rules.json`     | Path to rules.json                       |
| `--host`         | `127.0.0.1`                   | Management UI/API bind address           |
| `--port`         | `47831`                       | Management UI/API port                   |
| `--static-dir`   | `<install-dir>/web`           | Path to web UI assets                    |
| `--runtime`      | `service`                     | `service` (Go binary) or `node`          |
| `--node-path`    | `/usr/bin/node`               | Path to node executable (node mode only) |
| `--no-start`     | —                             | Enable but do not start the service      |

### Manage

```bash
sudo bash scripts/linux/status-service.sh
sudo bash scripts/linux/stop-service.sh
sudo bash scripts/linux/start-service.sh
```

Or use standard systemd tools directly:

```bash
sudo systemctl status portier
sudo systemctl stop portier
sudo systemctl start portier
sudo systemctl restart portier
sudo journalctl -u portier -f
```

### Uninstall

Stops the service, disables it, and removes the unit file. Preserves config and install directory by default:

```bash
sudo bash scripts/linux/uninstall-service.sh
```

Also remove install directory (binaries and web assets):

```bash
sudo bash scripts/linux/uninstall-service.sh --remove-files
```

Also remove config directory (rules.json and logs):

```bash
sudo bash scripts/linux/uninstall-service.sh --remove-files --remove-config
```

---

## Manual Install (Example Unit Files)

The example unit files in this directory can be copied and used without the helper scripts.

Go service (preferred):

```bash
sudo cp deploy/systemd/portier.service.example /etc/systemd/system/portier.service
sudo systemctl daemon-reload
sudo systemctl enable --now portier
```

Node fallback:

```bash
sudo cp deploy/systemd/portier-node.service.example /etc/systemd/system/portier.service
sudo systemctl daemon-reload
sudo systemctl enable --now portier
```

---

## Status and Logs

```bash
sudo systemctl status portier
sudo journalctl -u portier -f
sudo journalctl -u portier -n 100
```

---

## Automated Service Validation

The systemd install flow is validated by an explicit script that uses a test-specific unit name, port, and temp directory. It never touches the production `portier.service` unit or `/opt/portier`.

```bash
sudo npm run validate:service:linux
# or
sudo bash scripts/linux/validate-systemd-service.sh
```

The script:
- Builds `build/portier/` via `npm run package:portier` (pass `--no-build` to skip)
- Writes a `portier-test.service` unit to `/etc/systemd/system/`
- Starts the service, polls `/api/health`, verifies the web UI
- Stops the service, removes the unit file, runs `daemon-reload`, verifies removal
- Cleans up all temp files

Pass `--keep-files` to preserve temp directories on failure for debugging.
Pass `--port <number>` to override the auto-detected free port.

## Important Notes

**Do not enable both runtimes on the same port.** If two services try to bind `127.0.0.1:47831`, one will fail to start. Install one unit file only.

**Management UI** defaults to `http://127.0.0.1:47831` and is local-only. No firewall rule needed for the management port.

**Forwarded ports** listening on `0.0.0.0` are LAN-visible and may require firewall rules. Portier warns when a rule uses `0.0.0.0`.

```bash
# ufw example
sudo ufw allow 48001/tcp

# firewalld example
sudo firewall-cmd --add-port=48001/tcp --permanent
sudo firewall-cmd --reload
```

**Uninstall always preserves config** (`/etc/portier/rules.json`) unless `--remove-config` is explicitly passed.

**Node fallback node path:** the system PATH is not reliably available to systemd services. Always provide a full path to `node` via `--node-path` if not using the default `/usr/bin/node`.
