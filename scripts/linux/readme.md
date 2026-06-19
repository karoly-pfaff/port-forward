# Linux systemd Service Install

Portier can run as a system-level systemd service on Linux.

**Recommended:** use the helper scripts in `scripts/linux/` — they generate and install the unit file automatically.

The example unit files in `scripts/linux/service/` can be used for manual installs.

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
npm run build:runtime            # builds build/portier/
npm run validate:runtime:smoke   # build, validate layout, and smoke-test
```

Linux-specific package (produces build/linux/ with linux/amd64 binary):

```bash
bash scripts/linux/build-runtime.sh
```

## Release Artifact

The **portable tar.gz artifacts (amd64 + arm64)** are versioned, checksummed in
`checksums.sha256`, and GitHub-Release-ready. The native `.deb` (below) is the package; systemd
is the canonical service layer.

```bash
npm run build:release:current            # builds both portable arches + .deb + checksums.sha256
npm run build:release:current -- --no-build   # reuse an existing build/portier/
npm run validate:release:current         # validate host-arch layout/OpenAPI/version + checksum
```

Output: `build/releases/linux/portier-<version>-linux-amd64.tar.gz` and
`build/releases/linux/portier-<version>-linux-arm64.tar.gz` (architecture in the name).

Each archive contains the full runtime layout:

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

Extract and install from the archive on a target machine:

```bash
# x86-64: use the amd64 archive (ARM64: the arm64 archive).
tar -xzf portier-<version>-linux-amd64.tar.gz -C /opt/portier/
sudo bash scripts/linux/service/install-service.sh --source-dir /opt/portier
```

Preview the systemd install plan without root (paths, `ExecStart`, unit location):

```bash
bash scripts/linux/service/install-service.sh --dry-run
```

The Linux tar.gz artifacts can also be **cross-built from another host** (e.g. Windows), since
the Go binaries are pure Go:

```bash
npm run build:release:linux               # cross-compile linux/amd64 + linux/arm64 tar.gz
npm run validate:release:portable:linux   # structural validation of both arches (no native smoke)
```

Cross-built validation is structural only (it does verify each binary's machine type matches
the named arch) — the native runtime smoke (`validate:runtime:smoke`) must run on Linux, and
arm64 binaries are not executed on an amd64 host.

## Native `.deb` package

`build:release:current` on Linux also builds a native Debian package alongside the
portable tar.gz:

```bash
npm run build:release:current     # tar.gz + portier_<version>_amd64.deb + checksums.sha256
# or directly (reuses an existing build/portier/):
bash scripts/linux/release/build-release.sh --version <version>
```

Output: `build/releases/linux/portier_<version>_amd64.deb`

It is a **file-install** package (`dpkg-deb`): it lays the runtime under `/opt/portier`
and installs a systemd unit at `/lib/systemd/system/portier.service` that is left
**disabled**. The maintainer scripts only run `systemctl daemon-reload` (so the unit is
visible) and print how to opt in; on package removal they stop/disable the unit. The
package **never enables or starts the service** and never creates, overwrites, or
migrates user config (`/etc/portier/rules.json`). The Go service binary is static (CGO
disabled), so the package has no shared-library dependency. Opt in with:

```bash
sudo apt install ./portier_<version>_amd64.deb
sudo systemctl enable --now portier
```

`build-release.sh` requires `dpkg-deb` (Debian/Ubuntu); on other hosts the `.deb` step is
skipped with a notice and the portable tar.gz remains the baseline.

The **Release Linux** workflow (`.github/workflows/release-linux.yml`, manual
`workflow_dispatch`) runs the native path on `ubuntu-latest`: installs `rpm` tooling,
`build:release:current`, `validate:release:current`, `validate:release:checksums`,
`validate:runtime:smoke`, `validate:upgrade:current`, `.deb` and `.rpm` payload introspection,
and the `.deb` and `.rpm` install/remove smokes (below), then uploads `build/releases/linux/**`
(`.deb`, `.rpm`, portable tar.gz ×2, `checksums.sha256`) as the `portier-release-linux`
workflow artifact.

### `.deb` install/remove smoke

```bash
npm run validate:deb:install     # Linux only; needs sudo (passwordless on hosted runners)
```

Installs `build/releases/linux/portier_<version>_amd64.deb` with `apt-get install`, asserts
the installed layout (`/opt/portier/...` + `/lib/systemd/system/portier.service`) and the CLI
version, that the systemd unit is **disabled and inactive** (the package never enables or
starts it), and that a seeded `/etc/portier/rules.json` sentinel is untouched. It then
`apt-get remove`s the package and asserts the runtime files + unit are gone, the service is
not running, and user config is preserved. Runs on Linux only (exits 0 with a skip notice on
other platforms). This is a package-lifecycle smoke; full systemd service-install validation
(`validate:service:linux`) stays a separate manual/native check.

## Native `.rpm` package

`build:release:current` on Linux also builds a native RPM (rpmbuild) that mirrors the `.deb`:

```bash
npm run build:release:current     # also builds portier-<version>-1.x86_64.rpm
# or directly (reuses an existing build/portier/):
bash scripts/linux/release/build-rpm.sh --version <version>
```

Output: `build/releases/linux/portier-<version>-1.x86_64.rpm`

Same **file-install** behavior as the `.deb`: runtime under `/opt/portier`, a **disabled**
systemd unit, scriptlets that only `systemctl daemon-reload` + print opt-in guidance (and
stop/disable on erasure), no service enable/start, no config creation/overwrite. `AutoReqProv`
is off (the Go binary is static), so the package declares no dependencies. `build-rpm.sh`
requires `rpmbuild` (the `rpm` package on Debian/Ubuntu, native on Fedora/RHEL); on a host
without it the `.rpm` step is skipped with a notice. Opt in with:

```bash
sudo dnf install ./portier-<version>-1.x86_64.rpm   # or: sudo rpm -i ./portier-<version>-1.x86_64.rpm
sudo systemctl enable --now portier
```

### `.rpm` payload + install/remove smoke

```bash
npm run validate:rpm:payload     # rpm -qlp layout + forbidden-content check (needs the rpm CLI)
npm run validate:rpm:install     # rpm -i → assert → rpm -e (Linux only; needs sudo + rpm)
```

`validate:rpm:payload` lists the package (`rpm -qlp`) and asserts the `/opt/portier` layout +
systemd unit, the package metadata version, and that no `rules.json`/`docs/private` is shipped.
`validate:rpm:install` installs with `rpm -i`, asserts the layout + CLI version, the unit is
**disabled and inactive**, and a seeded `/etc/portier/rules.json` is untouched, then `rpm -e`s
and asserts clean removal with user config preserved. The rpm database is independent of dpkg,
so this runs on the Ubuntu CI runner (systemd is PID 1 there, giving real disabled/inactive
assertions). Both run on Linux only.
Full systemd service validation (`validate:service:linux`) needs root/systemd and is
**not** run in CI — it stays a manual/native check. No GitHub Release or tag is created.

> **`.rpm`:** still a **planned package-manager track for a later slice** (built and
> validated on a native `fedora`/RHEL-family runner). The portable tar.gz + `.deb` +
> systemd scripts are the current Linux install story.

## Helper Scripts (Recommended)

### Install

Build the package and install in one step:

```bash
npm run build:runtime
sudo bash scripts/linux/service/install-service.sh
```

The install script auto-copies `build/portier/` into `/opt/portier/`, creates the config directory and `rules.json` if missing, generates the systemd unit file, and enables and starts the service.

Node fallback (requires Node.js):

```bash
sudo bash scripts/linux/service/install-service.sh --runtime node
```

With custom paths:

```bash
sudo bash scripts/linux/service/install-service.sh \
  --install-dir /opt/portier \
  --config-path /etc/portier/rules.json \
  --host 127.0.0.1 \
  --port 47831
```

Supported flags:

| Flag             | Default                       | Description                                        |
|------------------|-------------------------------|----------------------------------------------------|
| `--source-dir`   | `build/portier/` (auto)       | Copy runtime files from this directory             |
| `--install-dir`  | `/opt/portier`                | Target directory for binaries and web/             |
| `--config-path`  | `/etc/portier/rules.json`     | Path to rules.json                                 |
| `--host`         | `127.0.0.1`                   | Management UI/API bind address                     |
| `--port`         | `47831`                       | Management UI/API port                             |
| `--static-dir`   | `<install-dir>/web`           | Path to web UI assets                              |
| `--runtime`      | `service`                     | `service` (Go binary) or `node`                    |
| `--node-path`    | `/usr/bin/node`               | Path to node executable (node mode only)           |
| `--no-enable`    | —                             | Skip `systemctl enable`; do not auto-start at boot |
| `--no-start`     | —                             | Enable but do not start the service immediately    |

If `build/portier/` exists at the time the script runs, it is copied automatically. Pass `--source-dir ""` (or remove `build/portier/`) to skip the copy and use an existing install directory instead.

### Manage

```bash
sudo bash scripts/linux/service/status-service.sh
sudo bash scripts/linux/service/stop-service.sh
sudo bash scripts/linux/service/start-service.sh
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
sudo bash scripts/linux/service/uninstall-service.sh
```

Also remove install directory (binaries and web assets):

```bash
sudo bash scripts/linux/service/uninstall-service.sh --remove-files
```

Also remove config directory (rules.json and logs):

```bash
sudo bash scripts/linux/service/uninstall-service.sh --remove-files --remove-config
```

---

## Manual Install (Example Unit Files)

The example unit files in `scripts/linux/service/` can be copied and used without the helper scripts.

Go service (preferred):

```bash
sudo cp scripts/linux/service/portier.service.example /etc/systemd/system/portier.service
sudo systemctl daemon-reload
sudo systemctl enable --now portier
```

Node fallback:

```bash
sudo cp scripts/linux/service/portier-node.service.example /etc/systemd/system/portier.service
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
sudo bash scripts/linux/service/validate-systemd-service.sh
```

The script:
- Builds `build/portier/` via `npm run build:runtime` (pass `--no-build` to skip)
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
