#!/usr/bin/env bash
set -euo pipefail

# IP Lookup Systemd Installation Script
# Usage: sudo bash scripts/install-systemd.sh [binary_path]

BINARY="${1:-./backend/ip-lookup}"
SERVICE_NAME="ip-lookup"
SERVICE_FILE="deploy/systemd/${SERVICE_NAME}.service"
CONFIG_SRC="backend/config.yaml"
CONFIG_DST="/etc/ip-lookup/config.yaml"
BINARY_DST="/usr/local/bin/${SERVICE_NAME}"
DATA_DIR="/var/lib/ip-lookup"
LOG_DIR="/var/log/ip-lookup"
USER="iplookup"
GROUP="iplookup"

if [ "$(id -u)" -ne 0 ]; then
	echo "This script must be run as root" >&2
	exit 1
fi

echo "[1/6] Creating system user and group..."
if ! getent group "${GROUP}" >/dev/null 2>&1; then
	groupadd --system "${GROUP}"
fi
if ! getent passwd "${USER}" >/dev/null 2>&1; then
	useradd --system --gid "${GROUP}" --no-create-home --shell /usr/sbin/nologin "${USER}"
fi

echo "[2/6] Creating directories..."
install -d -m 0755 -o "${USER}" -g "${GROUP}" "${DATA_DIR}"
install -d -m 0755 -o "${USER}" -g "${GROUP}" "${LOG_DIR}"
install -d -m 0755 -o root -g root /etc/ip-lookup

echo "[3/6] Installing binary..."
install -m 0755 -o root -g root "${BINARY}" "${BINARY_DST}"

echo "[4/6] Installing configuration..."
if [ ! -f "${CONFIG_DST}" ]; then
	install -m 0644 -o root -g root "${CONFIG_SRC}" "${CONFIG_DST}"
	echo "  (default config installed)"
else
	echo "  (existing config preserved)"
fi

echo "[5/6] Setting capabilities (fallback for AmbientCapabilities)..."
setcap cap_net_bind_service=+ep "${BINARY_DST}" 2>/dev/null || true

echo "[6/6] Installing and enabling systemd service..."
install -m 0644 -o root -g root "${SERVICE_FILE}" "/etc/systemd/system/${SERVICE_NAME}.service"
systemctl daemon-reload
systemctl enable --now "${SERVICE_NAME}"

echo "Done! Check status: systemctl status ${SERVICE_NAME}"
