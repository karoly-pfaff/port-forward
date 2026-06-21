# Upgrading To Portier 2.0

This guide is for existing Portier v1.x users moving to v2.0. Portier 2.0 is a stability and
packaging milestone, not a redesign: the REST API, the `rules.json` format, and CLI behavior are
backward-compatible with v1.x. Most upgrades are a straight install-over-the-top.

For the canonical packaging layout, install paths, and release artifacts, see
[installer.md](installer.md). For startup/config recovery behavior, see [recovery.md](recovery.md).

## Who Should Read This

- Anyone running a packaged Portier install (Windows MSI, macOS `.pkg`, Linux `.deb`/`.rpm`, or a
  portable archive) from a v1.x release.
- Anyone running Portier from a service/daemon (Windows Service or scheduled task, macOS
  LaunchAgent, Linux systemd) installed from v1.x.

If you run Portier directly from the repository, just pull and rebuild (`npm install && npm run
build`); there is nothing to migrate.

## Supported Upgrade Paths

- **v1.x → v2.0** is supported as an in-place upgrade. Install the v2.0 package over the existing
  install directory, or extract the v2.0 portable archive in place.
- There is **no automatic config migration** on upgrade. `rules.json` is read as-is.
- Downgrades are not formally tested. Keeping a config backup (below) makes reverting safe.

## Compatibility Expectations

- **`rules.json`** stays a backward-compatible unversioned JSON array of rules. v2.0 reads existing
  v1.x config without conversion, and never rewrites it to a new shape on startup. The
  export/import envelope version remains `"1"`.
- **REST API** — the `/api` contract is unchanged. Existing API clients, scripts, and the CLI keep
  working.
- **CLI** — command names and the documented exit-code policy are stable (see
  [tools/cli/readme.md](../tools/cli/readme.md)).
- **OpenAPI** — only the version metadata changes between releases; the schema is otherwise stable.

## Configuration Location Per Platform

`rules.json` lives **outside** the install directory and is preserved across upgrades. Default
locations:

| Platform | Config | Install directory |
| --- | --- | --- |
| Windows (machine) | `%ProgramData%\Portier\rules.json` | `%ProgramFiles%\Portier\` |
| Windows (user) | `%APPDATA%\Portier\rules.json` | `%LOCALAPPDATA%\Portier\` |
| macOS | `~/Library/Application Support/Portier/rules.json` | `/usr/local/portier` (`.pkg`) |
| Linux | `/etc/portier/rules.json` | `/opt/portier` |

The install directory (binaries, `web/`, bundled `api/openapi.json`) is disposable and can be
replaced wholesale; user config and data are never inside it.

## What Is Deliberately Not Automatic

By safety design, installers and upgrades do not act on your system on their own:

- Installers are **file-install only** — they do not enable or start a service, create a scheduled
  task, or load a LaunchAgent. Opt in with the bundled platform service scripts.
- No config migration runs at startup. Use the offline `portier config migrate` command if you
  ever need to normalize a config file; it is dry-run by default and backs up before writing.
- No firewall rules are created or removed.
- No telemetry, remote update, or download behavior is added.

## Before You Upgrade: Back Up Config

Back up `rules.json` before upgrading. Either copy the platform file above, or export from a running
instance:

```powershell
portier config export --out portier-rules-backup.json
```

The export is a portable `{ version, exportedAt, rules }` bundle you can re-import with
`portier config import` or from the Settings view.

## Upgrade Steps

1. Stop the running Portier service/instance if one is running.
2. Install the v2.0 package (MSI / `.pkg` / `.deb` / `.rpm`) or extract the v2.0 portable archive
   over the install directory. User config is preserved.
3. Start Portier again (your existing service registration continues to point at the install
   directory).

## Verify The Installation

- **CLI version:**

  ```powershell
  portier version
  ```

- **Runtime health and version:**

  ```powershell
  portier runtime
  ```

  Or check the REST API directly — `GET /api/health` for liveness and `GET /api/runtime` for the
  reported version and recovery state.

- **UI/API:** open `http://127.0.0.1:47831`. The Dashboard should load and your existing rules
  should be listed.

If the runtime reports a recovery state (an amber banner in the UI, or `recovery.active` in
`GET /api/runtime`), your `rules.json` could not be loaded cleanly. See [recovery.md](recovery.md)
— your original config is preserved and writes are blocked until you resolve it.

## Rollback

If you need to revert:

1. Stop Portier.
2. Reinstall the previous v1.x package or restore the previous portable archive into the install
   directory.
3. Your `rules.json` is unchanged by the upgrade; if you want to restore an exported backup, use
   `portier config import` or the Settings view.

## Known Non-Blocking Deferrals

These are intentionally out of scope for v2.0 and tracked as follow-ups:

- **Code signing / notarization** — Windows builds are unsigned (SmartScreen may warn) and the
  macOS `.pkg` is unsigned/not notarized (Gatekeeper may warn). Verify downloads with the published
  `checksums.sha256`.
- **Automated GitHub Release publishing** — release artifacts are built and validated by manual
  workflows; publishing and tagging stay explicit manual steps.
- **arm64 native packages** — arm64 ships as validated portable archives; native `.pkg`/`.deb`/
  `.rpm` packages remain amd64.
- **Service auto-install from installers** — installers stay file-install by design; service
  registration is an explicit, opt-in step.
- **Remote/team/auth management** — Portier 2.0 is local-first; remote management is deferred beyond
  2.0.
