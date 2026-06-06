#!/usr/bin/env bash
set -euo pipefail

LABEL="com.portier.port-forwarding"
PLIST_PATH="$HOME/Library/LaunchAgents/$LABEL.plist"
USER_ID="$(id -u)"

if ! launchctl list "$LABEL" &>/dev/null; then
  echo "Portier LaunchAgent is not loaded."
  exit 0
fi

# bootout stops the process and removes the registration.
# With KeepAlive=true, killing the process alone would cause launchd to restart it —
# bootout is the correct way to permanently stop a KeepAlive job.
if launchctl bootout "gui/$USER_ID/$LABEL" 2>/dev/null; then
  echo "Portier LaunchAgent stopped and unloaded."
else
  # Legacy fallback
  launchctl unload "$PLIST_PATH" 2>/dev/null || true
  echo "Portier LaunchAgent unloaded."
fi
