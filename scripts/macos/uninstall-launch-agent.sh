#!/usr/bin/env bash
set -euo pipefail

LABEL="com.portier.port-forwarding"
PLIST_DIR="$HOME/Library/LaunchAgents"
PLIST_PATH="$PLIST_DIR/$LABEL.plist"
USER_ID="$(id -u)"

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

CONFIG_PATH="$HOME/Library/Application Support/Portier/rules.json"
LOG_DIR="$HOME/Library/Logs/Portier"

echo ""
echo "Config preserved: $CONFIG_PATH"
echo ""
echo "To remove config and logs manually:"
echo "  rm -rf \"$HOME/Library/Application Support/Portier\""
echo "  rm -rf \"$LOG_DIR\""
