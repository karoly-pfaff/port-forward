#!/usr/bin/env bash
set -euo pipefail

LABEL="com.portier.port-forwarding"
PLIST_DIR="$HOME/Library/LaunchAgents"
PLIST_PATH="$PLIST_DIR/$LABEL.plist"
USER_ID="$(id -u)"

if [ ! -f "$PLIST_PATH" ]; then
  echo "Error: LaunchAgent plist not found: $PLIST_PATH" >&2
  echo "  Run install-launch-agent.sh first." >&2
  exit 1
fi

if launchctl list "$LABEL" &>/dev/null; then
  # Already registered — use kickstart to ensure the process is running.
  launchctl kickstart "gui/$USER_ID/$LABEL"
  echo "Portier kickstarted."
else
  # Not registered — bootstrap from plist.
  if launchctl bootstrap "gui/$USER_ID" "$PLIST_PATH" 2>/dev/null; then
    echo "Portier LaunchAgent bootstrapped."
  else
    # Legacy fallback
    launchctl load "$PLIST_PATH"
    echo "Portier LaunchAgent loaded."
  fi
fi
