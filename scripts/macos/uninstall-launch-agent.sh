#!/usr/bin/env bash
set -euo pipefail

LABEL="com.portier.port-forwarding"
PLIST_DIR="$HOME/Library/LaunchAgents"
PLIST_PATH="$PLIST_DIR/$LABEL.plist"
USER_ID="$(id -u)"
PURGE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --purge) PURGE="1"; shift ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

echo "Uninstalling Portier LaunchAgent..."

if launchctl list "$LABEL" &>/dev/null; then
  if launchctl bootout "gui/$USER_ID/$LABEL" 2>/dev/null; then
    echo "LaunchAgent stopped and unloaded."
  else
    # Legacy fallback for older macOS
    launchctl unload "$PLIST_PATH" 2>/dev/null || true
    echo "LaunchAgent unloaded."
  fi
else
  echo "LaunchAgent was not loaded."
fi

if [ -f "$PLIST_PATH" ]; then
  rm "$PLIST_PATH"
  echo "Removed plist: $PLIST_PATH"
else
  echo "Plist not found (already removed): $PLIST_PATH"
fi

CONFIG_DIR="$HOME/Library/Application Support/Portier"
CONFIG_PATH="$CONFIG_DIR/rules.json"
LOG_DIR="$HOME/Library/Logs/Portier"

if [ -n "$PURGE" ]; then
  echo ""
  echo "Removing config and logs (--purge)..."
  if [ -d "$CONFIG_DIR" ]; then
    rm -rf "$CONFIG_DIR"
    echo "  Removed: $CONFIG_DIR"
  fi
  if [ -d "$LOG_DIR" ]; then
    rm -rf "$LOG_DIR"
    echo "  Removed: $LOG_DIR"
  fi
  echo "Done."
else
  echo ""
  echo "Config preserved: $CONFIG_PATH"
  echo ""
  echo "To remove config and logs permanently:"
  echo "  bash \"$(dirname "$0")/uninstall-launch-agent.sh\" --purge"
  echo ""
  echo "Or manually:"
  echo "  rm -rf \"$CONFIG_DIR\""
  echo "  rm -rf \"$LOG_DIR\""
fi
