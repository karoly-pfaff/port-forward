#!/usr/bin/env bash
set -euo pipefail

LABEL="com.portier.port-forwarding"
USER_ID="$(id -u)"

if launchctl list "$LABEL" &>/dev/null; then
  echo "Portier LaunchAgent: loaded"
  echo ""
  launchctl print "gui/$USER_ID/$LABEL" 2>/dev/null || launchctl list "$LABEL" 2>/dev/null || true
else
  echo "Portier LaunchAgent: not loaded"
  echo ""
  echo "  To start:   bash scripts/macos/start-launch-agent.sh"
  echo "  To install: bash scripts/macos/install-launch-agent.sh"
fi

echo ""
echo "Full diagnostic:"
echo "  launchctl print gui/$USER_ID/$LABEL"
