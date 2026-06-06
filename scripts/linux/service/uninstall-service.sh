#!/usr/bin/env bash
set -euo pipefail

SERVICE_UNIT="/etc/systemd/system/portier.service"
SERVICE_NAME="portier"

INSTALL_DIR="/opt/portier"
CONFIG_PATH="/etc/portier/rules.json"
REMOVE_FILES=""
REMOVE_CONFIG=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-dir)    INSTALL_DIR="$2";  shift 2 ;;
    --config-path)    CONFIG_PATH="$2";  shift 2 ;;
    --remove-files)   REMOVE_FILES="1";  shift   ;;
    --remove-config)  REMOVE_CONFIG="1"; shift   ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

CONFIG_DIR="$(dirname "$CONFIG_PATH")"

if [ "$(id -u)" -ne 0 ]; then
  echo "Error: This script requires root. Run with sudo." >&2
  exit 1
fi

echo "Uninstalling Portier systemd service..."

# --- Stop and disable ---

if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
  echo "Stopping $SERVICE_NAME..."
  systemctl stop "$SERVICE_NAME"
fi

if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
  systemctl disable "$SERVICE_NAME"
  echo "Service disabled."
fi

# --- Remove unit file ---

if [ -f "$SERVICE_UNIT" ]; then
  rm "$SERVICE_UNIT"
  echo "Removed unit: $SERVICE_UNIT"
else
  echo "Unit not found (already removed): $SERVICE_UNIT"
fi

systemctl daemon-reload
echo "daemon-reload complete."

# --- Optionally remove install directory ---

if [ -n "$REMOVE_FILES" ]; then
  if [ -d "$INSTALL_DIR" ]; then
    rm -rf "$INSTALL_DIR"
    echo "Removed install directory: $INSTALL_DIR"
  else
    echo "Install directory not found: $INSTALL_DIR"
  fi
fi

# --- Optionally remove config directory ---

if [ -n "$REMOVE_CONFIG" ]; then
  if [ -d "$CONFIG_DIR" ]; then
    rm -rf "$CONFIG_DIR"
    echo "Removed config directory: $CONFIG_DIR"
  else
    echo "Config directory not found: $CONFIG_DIR"
  fi
fi

echo ""
if [ -z "$REMOVE_CONFIG" ]; then
  echo "Config preserved: $CONFIG_PATH"
  echo "  Pass --remove-config to also delete rules.json and the config directory."
fi
if [ -z "$REMOVE_FILES" ]; then
  echo "Install directory preserved: $INSTALL_DIR"
  echo "  Pass --remove-files to remove binaries and web assets."
fi
