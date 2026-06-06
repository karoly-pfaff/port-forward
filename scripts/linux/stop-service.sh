#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="portier"

if [ "$(id -u)" -ne 0 ]; then
  echo "Error: This script requires root. Run with sudo." >&2
  exit 1
fi

if ! systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
  echo "Service '$SERVICE_NAME' is not running."
  exit 0
fi

systemctl stop "$SERVICE_NAME"
echo "Service '$SERVICE_NAME' stopped."
