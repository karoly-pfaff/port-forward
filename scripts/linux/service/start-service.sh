#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="portier"

if [ "$(id -u)" -ne 0 ]; then
  echo "Error: This script requires root. Run with sudo." >&2
  exit 1
fi

if ! [ -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
  echo "Error: $SERVICE_NAME.service not found." >&2
  echo "  Install it first with scripts/linux/service/install-service.sh." >&2
  exit 1
fi

systemctl start "$SERVICE_NAME"
echo "Service '$SERVICE_NAME' started."
echo "  Status: sudo systemctl status $SERVICE_NAME"
echo "  Logs  : sudo journalctl -u $SERVICE_NAME -f"
