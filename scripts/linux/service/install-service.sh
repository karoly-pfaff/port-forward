#!/usr/bin/env bash
# Usage: sudo bash scripts/linux/service/install-service.sh [OPTIONS]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

SERVICE_UNIT="/etc/systemd/system/portier.service"
SERVICE_NAME="portier"

INSTALL_DIR="/opt/portier"
CONFIG_PATH="/etc/portier/rules.json"
HOST="127.0.0.1"
PORT="47831"
STATIC_DIR=""
RUNTIME="service"
NODE_PATH="/usr/bin/node"
NO_START=""
NO_ENABLE=""
SOURCE_DIR=""

# Auto-detect source from build/portier/ if present
_AUTO_SOURCE="$REPO_ROOT/build/portier"
if [ -d "$_AUTO_SOURCE" ]; then
  SOURCE_DIR="$_AUTO_SOURCE"
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-dir)  INSTALL_DIR="$2";  shift 2 ;;
    --config-path)  CONFIG_PATH="$2";  shift 2 ;;
    --host)         HOST="$2";         shift 2 ;;
    --port)         PORT="$2";         shift 2 ;;
    --static-dir)   STATIC_DIR="$2";   shift 2 ;;
    --runtime)      RUNTIME="$2";      shift 2 ;;
    --node-path)    NODE_PATH="$2";    shift 2 ;;
    --source-dir)   SOURCE_DIR="$2";   shift 2 ;;
    --no-start)     NO_START="1";      shift   ;;
    --no-enable)    NO_ENABLE="1";     shift   ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

[ -z "$STATIC_DIR" ] && STATIC_DIR="$INSTALL_DIR/web"
CONFIG_DIR="$(dirname "$CONFIG_PATH")"

echo "Portier systemd service install"
echo ""
echo "  Install dir : $INSTALL_DIR"
echo "  Config      : $CONFIG_PATH"
echo "  Runtime     : $RUNTIME"
echo "  Management  : http://$HOST:$PORT"
[ -n "$SOURCE_DIR" ] && echo "  Source      : $SOURCE_DIR"
echo ""

# --- Root check ---

if [ "$(id -u)" -ne 0 ]; then
  echo "Error: This script requires root. Run with sudo." >&2
  exit 1
fi

# --- Validate runtime ---

if [ "$RUNTIME" = "node" ]; then
  if [ ! -x "$NODE_PATH" ]; then
    echo "Error: Node.js not found at '$NODE_PATH'." >&2
    echo "  Install Node.js or pass --node-path /path/to/node." >&2
    exit 1
  fi
  SERVER_JS="$INSTALL_DIR/server.js"
  if [ ! -f "$SERVER_JS" ]; then
    echo "Error: server.js not found at '$SERVER_JS'." >&2
    echo "  Copy server.js to the install directory first." >&2
    exit 1
  fi
  EXEC_START="$NODE_PATH $INSTALL_DIR/server.js"
elif [ "$RUNTIME" = "service" ]; then
  SERVICE_BIN="$INSTALL_DIR/service"
  if [ ! -f "$SERVICE_BIN" ]; then
    echo "Error: Service binary not found at '$SERVICE_BIN'." >&2
    echo "  Copy the service binary to the install directory, or use --runtime node for Node.js fallback." >&2
    exit 1
  fi
  if [ ! -x "$SERVICE_BIN" ]; then
    chmod +x "$SERVICE_BIN"
  fi
  EXEC_START="$INSTALL_DIR/service"
else
  echo "Error: Unknown runtime '$RUNTIME'. Use 'service' or 'node'." >&2
  exit 1
fi

# --- Validate web directory ---

if [ ! -d "$STATIC_DIR" ]; then
  echo "Warning: Static client directory not found: $STATIC_DIR" >&2
  echo "  The API will work, but the web UI will not be available until web/ is present." >&2
  echo ""
fi

# --- Create directories ---

mkdir -p "$INSTALL_DIR"
mkdir -p "$CONFIG_DIR"

# --- Copy runtime files if source-dir is set ---

if [ -n "$SOURCE_DIR" ]; then
  if [ ! -d "$SOURCE_DIR" ]; then
    echo "Error: Source directory not found: $SOURCE_DIR" >&2
    exit 1
  fi
  cp -r "$SOURCE_DIR/." "$INSTALL_DIR/"
  chmod +x "$INSTALL_DIR/service" 2>/dev/null || true
  echo "Copied runtime files from $SOURCE_DIR to $INSTALL_DIR"
fi

# --- Create default rules.json if missing ---

if [ ! -f "$CONFIG_PATH" ]; then
  printf '[]' > "$CONFIG_PATH"
  echo "Created empty config: $CONFIG_PATH"
fi

if [ ! -w "$CONFIG_PATH" ]; then
  echo "Error: Config file is not writable: $CONFIG_PATH" >&2
  exit 1
fi

# --- Generate systemd unit ---

FULL_EXEC_START="$EXEC_START --service --config $CONFIG_PATH --host $HOST --port $PORT --static-dir $STATIC_DIR"

if [ -f "$SERVICE_UNIT" ]; then
  echo "Replacing existing unit: $SERVICE_UNIT"
fi

if [ "$RUNTIME" = "node" ]; then
  cat > "$SERVICE_UNIT" <<EOF
[Unit]
Description=Portier Port Forwarding
After=network.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
ExecStart=$FULL_EXEC_START
Environment=NODE_ENV=production
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
else
  cat > "$SERVICE_UNIT" <<EOF
[Unit]
Description=Portier Port Forwarding
After=network.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
ExecStart=$FULL_EXEC_START
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
fi

echo "Unit written: $SERVICE_UNIT"

# --- Enable service ---

systemctl daemon-reload

if [ -z "$NO_ENABLE" ]; then
  systemctl enable "$SERVICE_NAME"
  echo "Service enabled (starts at boot)."
fi

if [ -z "$NO_START" ]; then
  systemctl start "$SERVICE_NAME"
  echo "Service started."
fi

if [ -n "$NO_START" ] && [ -n "$NO_ENABLE" ]; then
  STATUS_MSG="installed (not enabled, not started)"
elif [ -n "$NO_START" ]; then
  STATUS_MSG="installed and enabled (not started)"
else
  STATUS_MSG="running"
fi

echo ""
echo "Portier is $STATUS_MSG."
echo ""
echo "  Management UI : http://$HOST:$PORT"
echo "  Config        : $CONFIG_PATH"
echo "  Unit file     : $SERVICE_UNIT"
echo ""
echo "Next steps:"
echo "  Status    sudo bash \"$SCRIPT_DIR/status-service.sh\""
echo "  Stop      sudo bash \"$SCRIPT_DIR/stop-service.sh\""
echo "  Uninstall sudo bash \"$SCRIPT_DIR/uninstall-service.sh\""
echo "  Logs      sudo journalctl -u $SERVICE_NAME -f"
