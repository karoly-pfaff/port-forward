#!/usr/bin/env bash
set -euo pipefail

LABEL="com.portier.port-forwarding"
PLIST_DIR="$HOME/Library/LaunchAgents"
PLIST_PATH="$PLIST_DIR/$LABEL.plist"
LOG_DIR="$HOME/Library/Logs/Portier"
USER_ID="$(id -u)"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

INSTALL_DIR="$HOME/Applications/Portier"
CONFIG_PATH="$HOME/Library/Application Support/Portier/rules.json"
HOST="127.0.0.1"
PORT="47831"
STATIC_DIR=""
NODE_MODE=""
EXECUTABLE=""
NO_START=""
SOURCE_DIR=""

# Auto-detect source from repo build/portier/ if it exists alongside the script.
_AUTO_SOURCE="$REPO_ROOT/build/portier"
if [ -d "$_AUTO_SOURCE" ]; then
  SOURCE_DIR="$_AUTO_SOURCE"
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-dir)  INSTALL_DIR="$2";  shift 2 ;;
    --config-path)  CONFIG_PATH="$2";  shift 2 ;;
    --source-dir)   SOURCE_DIR="$2";   shift 2 ;;
    --host)         HOST="$2";         shift 2 ;;
    --port)         PORT="$2";         shift 2 ;;
    --static-dir)   STATIC_DIR="$2";   shift 2 ;;
    --no-start)     NO_START="1";      shift   ;;
    --node-mode)    NODE_MODE="1";     shift   ;;
    --runtime)
      case "$2" in
        node)    NODE_MODE="1" ;;
        service) NODE_MODE=""  ;;
        *) echo "Unknown --runtime value: $2 (expected 'service' or 'node')" >&2; exit 1 ;;
      esac
      shift 2 ;;
    --executable)   EXECUTABLE="$2";   shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

[ -z "$STATIC_DIR" ]  && STATIC_DIR="$INSTALL_DIR/web"
[ -z "$EXECUTABLE" ]  && EXECUTABLE="$INSTALL_DIR/service"

CONFIG_DIR="$(dirname "$CONFIG_PATH")"

echo "Portier LaunchAgent install"
echo ""
echo "  Install dir : $INSTALL_DIR"
echo "  Config      : $CONFIG_PATH"
echo "  Management  : http://$HOST:$PORT"
echo "  Mode        : ${NODE_MODE:+node (fallback)}${NODE_MODE:-native service}"
echo ""

# --- Copy runtime files from source dir ---
# Copies build/portier/ (or --source-dir) into the install directory.
# Skip only if SOURCE_DIR is empty (manual layout already in place).

if [ -n "$SOURCE_DIR" ]; then
  if [ ! -d "$SOURCE_DIR" ]; then
    echo "Error: source directory not found: $SOURCE_DIR" >&2
    echo "  Run 'npm run build:runtime' or pass --source-dir <dir>." >&2
    exit 1
  fi
  echo "Copying runtime files: $SOURCE_DIR -> $INSTALL_DIR"
  mkdir -p "$INSTALL_DIR"
  cp -r "$SOURCE_DIR/." "$INSTALL_DIR/"
  if [ -f "$INSTALL_DIR/service" ]; then
    chmod +x "$INSTALL_DIR/service"
  fi
  echo ""
fi

# --- Validate installed files ---

if [ -n "$NODE_MODE" ]; then
  # Resolve node binary at install time so launchd can find it without PATH.
  NODE_BIN="$(which node 2>/dev/null || true)"
  if [ -z "$NODE_BIN" ]; then
    for CANDIDATE in /usr/local/bin/node /opt/homebrew/bin/node; do
      if [ -x "$CANDIDATE" ]; then
        NODE_BIN="$CANDIDATE"
        break
      fi
    done
  fi
  if [ -z "$NODE_BIN" ]; then
    echo "Error: node not found in PATH or common locations." >&2
    echo "  Install Node.js and ensure it is on PATH before running this script." >&2
    exit 1
  fi
  NODE_ENTRY="$INSTALL_DIR/server.js"
  if [ ! -f "$NODE_ENTRY" ]; then
    echo "Error: Node entry point not found: $NODE_ENTRY" >&2
    echo "  Copy server.js to the install directory first." >&2
    exit 1
  fi
else
  if [ ! -f "$EXECUTABLE" ]; then
    echo "Error: Executable not found: $EXECUTABLE" >&2
    echo "  Run 'npm run build:runtime' and then re-run this script," >&2
    echo "  or use --runtime node for Node.js fallback." >&2
    exit 1
  fi
fi

if [ ! -d "$STATIC_DIR" ]; then
  echo "Warning: Static client directory not found: $STATIC_DIR" >&2
  echo "  The API will work, but the web UI will not be available until web/ files are copied." >&2
  echo ""
fi

# --- Create directories ---

mkdir -p "$CONFIG_DIR"
mkdir -p "$PLIST_DIR"
mkdir -p "$LOG_DIR"

# --- Create default rules.json if missing ---

if [ ! -f "$CONFIG_PATH" ]; then
  printf '[]' > "$CONFIG_PATH"
  echo "Created empty config: $CONFIG_PATH"
fi

if [ ! -w "$CONFIG_PATH" ]; then
  echo "Error: Config file is not writable: $CONFIG_PATH" >&2
  exit 1
fi

# --- Generate plist ---
# Paths must be absolute — launchd does not expand ~ in plist values.

{
  echo '<?xml version="1.0" encoding="UTF-8"?>'
  echo '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"'
  echo '  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">'
  echo '<plist version="1.0">'
  echo '<dict>'
  echo '	<key>Label</key>'
  echo "	<string>$LABEL</string>"
  echo '	<key>ProgramArguments</key>'
  echo '	<array>'
  if [ -n "$NODE_MODE" ]; then
    echo "		<string>$NODE_BIN</string>"
    echo "		<string>$INSTALL_DIR/server.js</string>"
  else
    echo "		<string>$EXECUTABLE</string>"
  fi
  echo '		<string>--service</string>'
  echo '		<string>--config</string>'
  echo "		<string>$CONFIG_PATH</string>"
  echo '		<string>--host</string>'
  echo "		<string>$HOST</string>"
  echo '		<string>--port</string>'
  echo "		<string>$PORT</string>"
  echo '		<string>--static-dir</string>'
  echo "		<string>$STATIC_DIR</string>"
  echo '	</array>'
  echo '	<key>WorkingDirectory</key>'
  echo "	<string>$INSTALL_DIR</string>"
  echo '	<key>RunAtLoad</key>'
  echo '	<true/>'
  echo '	<key>KeepAlive</key>'
  echo '	<true/>'
  echo '	<key>StandardOutPath</key>'
  echo "	<string>$LOG_DIR/portier.out.log</string>"
  echo '	<key>StandardErrorPath</key>'
  echo "	<string>$LOG_DIR/portier.err.log</string>"
  echo '</dict>'
  echo '</plist>'
} > "$PLIST_PATH"

echo "Plist written: $PLIST_PATH"

# --- Load or skip the LaunchAgent ---

if [ -n "$NO_START" ]; then
  echo ""
  echo "Skipping LaunchAgent load (--no-start)."
  echo ""
  echo "To start Portier later:"
  echo "  bash \"$(dirname "$0")/start-launch-agent.sh\""
  echo "  or: launchctl bootstrap gui/$USER_ID \"$PLIST_PATH\""
  exit 0
fi

# Unload first if already registered to avoid a stale registration on reinstall.
if launchctl list "$LABEL" &>/dev/null; then
  echo "LaunchAgent already loaded — unloading before reinstall..."
  launchctl bootout "gui/$USER_ID/$LABEL" 2>/dev/null \
    || launchctl unload "$PLIST_PATH" 2>/dev/null \
    || true
fi

# Prefer modern launchctl bootstrap (macOS 10.10+).
# Fall back to legacy launchctl load for older systems.
if launchctl bootstrap "gui/$USER_ID" "$PLIST_PATH" 2>/dev/null; then
  echo "LaunchAgent bootstrapped."
else
  launchctl load "$PLIST_PATH"
  echo "LaunchAgent loaded (legacy launchctl load)."
fi

echo ""
echo "Portier is starting."
echo ""
echo "  Management UI : http://$HOST:$PORT"
echo "  Logs          : $LOG_DIR/"
echo ""
echo "Next steps:"
echo "  Status    bash \"$(dirname "$0")/status-launch-agent.sh\""
echo "  Stop      bash \"$(dirname "$0")/stop-launch-agent.sh\""
echo "  Uninstall bash \"$(dirname "$0")/uninstall-launch-agent.sh\""
echo "  Logs      tail -f \"$LOG_DIR/portier.out.log\""
