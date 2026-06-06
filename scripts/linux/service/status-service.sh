#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="portier"

if ! [ -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
  echo "Portier systemd service: not installed."
  echo "  Install with: sudo bash scripts/linux/service/install-service.sh"
  exit 0
fi

systemctl status "$SERVICE_NAME" --no-pager || true

echo ""
echo "Live logs:"
echo "  sudo journalctl -u $SERVICE_NAME -f"
