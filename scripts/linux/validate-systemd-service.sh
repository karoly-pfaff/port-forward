#!/usr/bin/env bash
# Validates Portier Linux systemd service install flow.
# Requires root (sudo). Uses test-specific unit name and paths;
# never touches production Portier installs or the portier.service unit.
#
# Usage:
#   sudo bash scripts/linux/validate-systemd-service.sh [--no-build] [--keep-files] [--port PORT]
#
# Flags:
#   --no-build    Skip npm run package:portier; use existing build/portier/.
#   --keep-files  Preserve temp directories after validation (for debugging).
#   --port PORT   Management port for the test service (default: auto-detect free port).
set -uo pipefail

TEST_SERVICE_NAME="portier-test"
TEST_UNIT_FILE="/etc/systemd/system/${TEST_SERVICE_NAME}.service"
TEST_BASE="/tmp/portier-test-$$"
TEST_INSTALL="$TEST_BASE/install"
TEST_CONFIG_DIR="$TEST_BASE/config"
TEST_CONFIG="$TEST_CONFIG_DIR/rules.json"
HOST_ADDR="127.0.0.1"
PORT=""
NO_BUILD=""
KEEP_FILES=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-build)   NO_BUILD=1;   shift ;;
    --keep-files) KEEP_FILES=1; shift ;;
    --port)       PORT="$2";    shift 2 ;;
    *) echo "[validate:linux] Unknown argument: $1" >&2; exit 1 ;;
  esac
done

log()  { echo "[validate:linux] $*"; }
pass() { echo "  [PASS] $*"; }

# Auto-detect free port
get_free_port() {
  if command -v python3 >/dev/null 2>&1; then
    python3 -c "import socket; s=socket.socket(); s.bind(('127.0.0.1',0)); p=s.getsockname()[1]; s.close(); print(p)"
  else
    echo "47842"
  fi
}

[ -z "$PORT" ] && PORT="$(get_free_port)"
HEALTH_URL="http://${HOST_ADDR}:${PORT}/api/health"
ROOT_URL="http://${HOST_ADDR}:${PORT}/"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PACKAGE_DIR="$REPO_ROOT/build/portier"

# Require root
if [ "$(id -u)" -ne 0 ]; then
  echo "[validate:linux] ERROR: This script requires root. Run with sudo." >&2
  echo "  sudo bash scripts/linux/validate-systemd-service.sh" >&2
  exit 1
fi

# Require systemd
if ! command -v systemctl >/dev/null 2>&1; then
  echo "[validate:linux] ERROR: systemctl not found — systemd is required for this validation." >&2
  exit 1
fi
if ! systemctl list-units >/dev/null 2>&1; then
  echo "[validate:linux] ERROR: systemctl is not responsive — is systemd running as PID 1?" >&2
  exit 1
fi

# Cleanup function — runs on EXIT regardless of success or failure
_CLEANUP_DONE=0
cleanup() {
  [ "$_CLEANUP_DONE" = "1" ] && return
  _CLEANUP_DONE=1
  log "Cleanup..."

  # Stop and disable service if loaded
  if systemctl list-units --full --all 2>/dev/null | grep -q "${TEST_SERVICE_NAME}.service"; then
    systemctl stop "$TEST_SERVICE_NAME" 2>/dev/null || true
    systemctl disable "$TEST_SERVICE_NAME" 2>/dev/null || true
    log "  Service '$TEST_SERVICE_NAME' stopped and disabled."
  fi

  # Remove unit file
  if [ -f "$TEST_UNIT_FILE" ]; then
    rm -f "$TEST_UNIT_FILE" || true
    systemctl daemon-reload 2>/dev/null || true
    log "  Unit file removed and daemon reloaded."
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
  local url="$1" timeout="${2:-30}"
  local deadline=$((SECONDS + timeout))
  while [ "$SECONDS" -lt "$deadline" ]; do
    if curl -sf --max-time 2 "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  return 1
}

log "Linux systemd service validation (root)"
log "  Service name : $TEST_SERVICE_NAME"
log "  Unit file    : $TEST_UNIT_FILE"
log "  Install dir  : $TEST_INSTALL"
log "  Config       : $TEST_CONFIG"
log "  Port         : $PORT"
log ""

# Preflight: fail if test unit already exists
if [ -f "$TEST_UNIT_FILE" ]; then
  echo "[validate:linux] ERROR: Unit file already exists: $TEST_UNIT_FILE" >&2
  echo "  Remove it first:" >&2
  echo "    systemctl stop $TEST_SERVICE_NAME 2>/dev/null || true" >&2
  echo "    rm -f $TEST_UNIT_FILE && systemctl daemon-reload" >&2
  exit 1
fi
if systemctl is-active "$TEST_SERVICE_NAME" >/dev/null 2>&1; then
  echo "[validate:linux] ERROR: Service '$TEST_SERVICE_NAME' is already active." >&2
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
  echo "[validate:linux] ERROR: service binary not found: $SERVICE_BIN" >&2
  echo "  Run: npm run package:portier" >&2
  exit 1
fi
if [ ! -d "$PACKAGE_DIR/web" ]; then
  echo "[validate:linux] ERROR: web/ not found in build/portier/" >&2
  exit 1
fi

# Set up test install dir
log "Creating test install directory..."
mkdir -p "$TEST_INSTALL" "$TEST_CONFIG_DIR"
cp -r "$PACKAGE_DIR/"* "$TEST_INSTALL/"
chmod +x "$TEST_INSTALL/service"
printf '[]' > "$TEST_CONFIG"
pass "Test install directory ready."

# Write systemd unit file
log "Writing unit file: $TEST_UNIT_FILE"
STATIC_DIR="$TEST_INSTALL/web"
EXEC_START="$TEST_INSTALL/service --service --config $TEST_CONFIG --host $HOST_ADDR --port $PORT --static-dir $STATIC_DIR"

cat > "$TEST_UNIT_FILE" <<UNIT
[Unit]
Description=Portier systemd service validation (test — safe to remove)
After=network.target

[Service]
Type=simple
WorkingDirectory=${TEST_INSTALL}
ExecStart=${EXEC_START}
Restart=no
UNIT

pass "Unit file written."

# Reload systemd
log "Running systemctl daemon-reload..."
systemctl daemon-reload
pass "daemon-reload complete."

# Start service
log "Starting service '$TEST_SERVICE_NAME'..."
systemctl start "$TEST_SERVICE_NAME"
pass "Service started."

# Poll /api/health
log "Polling $HEALTH_URL (up to 30s)..."
if ! wait_for_health "$HEALTH_URL" 30; then
  echo "[validate:linux] Diagnostics:" >&2
  systemctl status "$TEST_SERVICE_NAME" --no-pager 2>&1 | head -20 >&2 || true
  journalctl -u "$TEST_SERVICE_NAME" -n 30 --no-pager 2>&1 >&2 || true
  echo "[validate:linux] FAILED: service did not respond on $HEALTH_URL within 30s." >&2
  exit 1
fi
pass "/api/health responded OK."

# Verify web UI
log "Checking web UI at $ROOT_URL..."
UI_BODY="$(curl -sf --max-time 5 "$ROOT_URL" 2>/dev/null || true)"
if echo "$UI_BODY" | grep -qi "<html"; then
  pass "Web UI served at /."
else
  echo "[validate:linux] FAILED: web UI not served at / or does not contain <html." >&2
  exit 1
fi

# Stop service
log "Stopping service '$TEST_SERVICE_NAME'..."
systemctl stop "$TEST_SERVICE_NAME"
pass "Service stopped."

# Remove unit file
log "Removing unit file: $TEST_UNIT_FILE"
rm -f "$TEST_UNIT_FILE"
systemctl daemon-reload
pass "Unit file removed and daemon reloaded."

# Verify service no longer active
if systemctl is-active "$TEST_SERVICE_NAME" >/dev/null 2>&1; then
  echo "[validate:linux] FAILED: Service '$TEST_SERVICE_NAME' still active after stop and unit removal." >&2
  exit 1
fi
pass "Service '$TEST_SERVICE_NAME' verified stopped and removed."

log ""
log "Linux systemd service validation PASSED."
