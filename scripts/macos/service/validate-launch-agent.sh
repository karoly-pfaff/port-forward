#!/usr/bin/env bash
# Validates Portier macOS LaunchAgent install flow.
# No sudo required — runs as the current user.
# Uses test-specific label and paths; never touches production Portier installs.
#
# Usage:
#   bash scripts/macos/service/validate-launch-agent.sh [--no-build] [--keep-files] [--port PORT]
#
# Flags:
#   --no-build    Skip npm run package:portier; use existing build/portier/.
#   --keep-files  Preserve temp directories after validation (for debugging).
#   --port PORT   Management port for the test service (default: auto-detect free port).
set -uo pipefail

TEST_LABEL="com.portier.test"
TEST_PLIST="$HOME/Library/LaunchAgents/${TEST_LABEL}.plist"
TMPDIR_BASE="${TMPDIR:-/tmp}"
TMPDIR_BASE="${TMPDIR_BASE%/}"
TEST_BASE="$TMPDIR_BASE/portier-test-$$"
TEST_INSTALL="$TEST_BASE/install"
TEST_CONFIG_DIR="$TEST_BASE/config"
TEST_CONFIG="$TEST_CONFIG_DIR/rules.json"
TEST_LOG_DIR="$TEST_BASE/logs"
HOST_ADDR="127.0.0.1"
PORT=""
NO_BUILD=""
KEEP_FILES=""
USER_ID="$(id -u)"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-build)   NO_BUILD=1;   shift ;;
    --keep-files) KEEP_FILES=1; shift ;;
    --port)       PORT="$2";    shift 2 ;;
    *) echo "[validate:macos] Unknown argument: $1" >&2; exit 1 ;;
  esac
done

log()  { echo "[validate:macos] $*"; }
pass() { echo "  [PASS] $*"; }

# Auto-detect free port with python3 or fall back to parameter default
get_free_port() {
  if command -v python3 >/dev/null 2>&1; then
    python3 -c "import socket; s=socket.socket(); s.bind(('127.0.0.1',0)); p=s.getsockname()[1]; s.close(); print(p)"
  else
    echo "47841"
  fi
}

[ -z "$PORT" ] && PORT="$(get_free_port)"
HEALTH_URL="http://${HOST_ADDR}:${PORT}/api/health"
ROOT_URL="http://${HOST_ADDR}:${PORT}/"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
PACKAGE_DIR="$REPO_ROOT/build/portier"

# Cleanup function — runs on EXIT regardless of success or failure
_CLEANUP_DONE=0
cleanup() {
  [ "$_CLEANUP_DONE" = "1" ] && return
  _CLEANUP_DONE=1
  log "Cleanup..."

  # Unload agent if still loaded
  if launchctl list "$TEST_LABEL" >/dev/null 2>&1; then
    launchctl bootout "gui/$USER_ID/$TEST_LABEL" 2>/dev/null \
      || launchctl unload "$TEST_PLIST" 2>/dev/null || true
    log "  LaunchAgent '$TEST_LABEL' unloaded."
  fi

  # Remove plist if it exists
  if [ -f "$TEST_PLIST" ]; then
    rm -f "$TEST_PLIST" || true
    log "  Plist removed: $TEST_PLIST"
  fi

  # Remove temp dir unless --keep-files
  if [ -n "$KEEP_FILES" ]; then
    if [ -d "$TEST_BASE" ]; then
      log "  --keep-files: temp dir preserved: $TEST_BASE"
    fi
  else
    if [ -d "$TEST_BASE" ]; then
      rm -rf "$TEST_BASE" || true
      log "  Temp dir removed."
    fi
  fi
}

trap 'LAST=$?; cleanup; exit $LAST' EXIT

wait_for_health() {
  local url="$1" timeout="${2:-25}"
  local deadline=$((SECONDS + timeout))
  while [ "$SECONDS" -lt "$deadline" ]; do
    if curl -sf --max-time 2 "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  return 1
}

log "macOS LaunchAgent validation"
log "  Label      : $TEST_LABEL"
log "  Plist      : $TEST_PLIST"
log "  Install dir: $TEST_INSTALL"
log "  Config     : $TEST_CONFIG"
log "  Port       : $PORT"
log ""

# Preflight: fail if agent already loaded from a previous run
if launchctl list "$TEST_LABEL" >/dev/null 2>&1; then
  echo "[validate:macos] ERROR: LaunchAgent '$TEST_LABEL' already loaded from a previous run." >&2
  echo "  Remove it first:" >&2
  echo "    launchctl bootout gui/$(id -u)/$TEST_LABEL" >&2
  echo "    rm -f $TEST_PLIST" >&2
  exit 1
fi
if [ -f "$TEST_PLIST" ]; then
  echo "[validate:macos] ERROR: Plist already exists: $TEST_PLIST" >&2
  echo "  Remove it first: rm -f $TEST_PLIST" >&2
  exit 1
fi

# Build package if requested
if [ -z "$NO_BUILD" ]; then
  log "Running npm run package:portier..."
  npm run package:portier
  log ""
else
  log "Skipping package build (--no-build)."
fi

# Verify package contents
SERVICE_BIN="$PACKAGE_DIR/service"
if [ ! -f "$SERVICE_BIN" ]; then
  echo "[validate:macos] ERROR: service binary not found: $SERVICE_BIN" >&2
  echo "  Run: npm run package:portier" >&2
  exit 1
fi
if [ ! -d "$PACKAGE_DIR/web" ]; then
  echo "[validate:macos] ERROR: web/ not found in build/portier/" >&2
  exit 1
fi

# Set up test install dir
log "Creating test install directory..."
mkdir -p "$TEST_INSTALL" "$TEST_CONFIG_DIR" "$TEST_LOG_DIR"
cp -r "$PACKAGE_DIR/"* "$TEST_INSTALL/"
chmod +x "$TEST_INSTALL/service"
printf '[]' > "$TEST_CONFIG"
pass "Test install directory ready."

# Generate LaunchAgent plist with test label and isolated paths
log "Writing LaunchAgent plist: $TEST_PLIST"
mkdir -p "$(dirname "$TEST_PLIST")"
STATIC_DIR="$TEST_INSTALL/web"
cat > "$TEST_PLIST" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>${TEST_LABEL}</string>
	<key>ProgramArguments</key>
	<array>
		<string>${TEST_INSTALL}/service</string>
		<string>--service</string>
		<string>--config</string>
		<string>${TEST_CONFIG}</string>
		<string>--host</string>
		<string>${HOST_ADDR}</string>
		<string>--port</string>
		<string>${PORT}</string>
		<string>--static-dir</string>
		<string>${STATIC_DIR}</string>
	</array>
	<key>WorkingDirectory</key>
	<string>${TEST_INSTALL}</string>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
	<key>StandardOutPath</key>
	<string>${TEST_LOG_DIR}/portier.out.log</string>
	<key>StandardErrorPath</key>
	<string>${TEST_LOG_DIR}/portier.err.log</string>
</dict>
</plist>
PLIST
pass "Plist written."

# Bootstrap LaunchAgent
log "Bootstrapping LaunchAgent '$TEST_LABEL'..."
if launchctl bootstrap "gui/$USER_ID" "$TEST_PLIST" 2>/dev/null; then
  pass "LaunchAgent bootstrapped."
else
  launchctl load "$TEST_PLIST"
  pass "LaunchAgent loaded (legacy launchctl load)."
fi

# Poll /api/health
log "Polling $HEALTH_URL (up to 25s)..."
if ! wait_for_health "$HEALTH_URL" 25; then
  echo "[validate:macos] Diagnostics:" >&2
  launchctl print "gui/$USER_ID/$TEST_LABEL" 2>&1 | head -20 >&2 || true
  if [ -f "$TEST_LOG_DIR/portier.err.log" ]; then
    echo "--- stderr log ---" >&2
    tail -20 "$TEST_LOG_DIR/portier.err.log" >&2
  fi
  echo "[validate:macos] FAILED: service did not respond on $HEALTH_URL within 25s." >&2
  exit 1
fi
pass "/api/health responded OK."

# Verify web UI
log "Checking web UI at $ROOT_URL..."
UI_BODY="$(curl -sf --max-time 5 "$ROOT_URL" 2>/dev/null || true)"
if echo "$UI_BODY" | grep -qi "<html"; then
  pass "Web UI served at /."
else
  echo "[validate:macos] FAILED: web UI not served at / or does not contain <html." >&2
  exit 1
fi

# Stop agent
log "Stopping LaunchAgent '$TEST_LABEL'..."
launchctl bootout "gui/$USER_ID/$TEST_LABEL" 2>/dev/null \
  || launchctl unload "$TEST_PLIST" 2>/dev/null || true
sleep 1

# Verify unloaded
if launchctl list "$TEST_LABEL" >/dev/null 2>&1; then
  echo "[validate:macos] FAILED: LaunchAgent '$TEST_LABEL' still loaded after bootout." >&2
  exit 1
fi
pass "LaunchAgent '$TEST_LABEL' verified unloaded."

# Remove plist
rm -f "$TEST_PLIST"
pass "Plist removed."

log ""
log "macOS LaunchAgent validation PASSED."
