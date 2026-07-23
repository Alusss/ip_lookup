#!/usr/bin/env bash
set -euo pipefail

# Install Cloudflare CIDR sync cron/systemd timer
# Usage: sudo bash deploy/scripts/install-cf-sync-cron.sh [method]
#   method: cron (default) or timer

METHOD="${1:-cron}"
SCRIPT_PATH="$(cd "$(dirname "$0")/.." && pwd)/scripts/update-cloudflare-ip.sh"

if [ "$(id -u)" -ne 0 ]; then
	echo "This script must be run as root" >&2
	exit 1
fi

if [ ! -f "$SCRIPT_PATH" ]; then
	echo "Script not found: $SCRIPT_PATH" >&2
	exit 1
fi

case "$METHOD" in
	cron)
		echo "Installing cron entries..."
		cat > /etc/cron.d/cloudflare-ip-sync <<'CRON'
# Cloudflare CIDR auto-sync
# Full sync daily at 03:00
0 3 * * * root /bin/bash -c '/etc/ip-lookup/update-cloudflare-ip.sh || logger -t cf-ip-sync "FAILED"'
# Incremental check every 6 hours
0 */6 * * * root /bin/bash -c '/etc/ip-lookup/update-cloudflare-ip.sh || logger -t cf-ip-sync "FAILED"'
CRON
		cp "$SCRIPT_PATH" /etc/ip-lookup/update-cloudflare-ip.sh
		chmod +x /etc/ip-lookup/update-cloudflare-ip.sh
		echo "Cron installed at /etc/cron.d/cloudflare-ip-sync"
		;;
	timer)
		echo "Installing systemd timer..."
		TIMER_DIR="/etc/systemd/system"

		cat > "${TIMER_DIR}/cf-ip-sync.service" <<'SERVICE'
[Unit]
Description=Cloudflare IP CIDR Sync
[Service]
Type=oneshot
ExecStart=/etc/ip-lookup/update-cloudflare-ip.sh
SERVICE

		cat > "${TIMER_DIR}/cf-ip-sync.timer" <<'TIMER'
[Unit]
Description=Cloudflare IP CIDR daily sync
[Timer]
OnCalendar=daily
OnCalendar=*-*-* 03:00:00
OnUnitActiveSec=6h
Persistent=true
[Install]
WantedBy=timers.target
TIMER

		cp "$SCRIPT_PATH" /etc/ip-lookup/update-cloudflare-ip.sh
		chmod +x /etc/ip-lookup/update-cloudflare-ip.sh
		systemctl daemon-reload
		systemctl enable --now cf-ip-sync.timer
		echo "Systemd timer installed and enabled"
		;;
	*)
		echo "Unknown method: $METHOD (use: cron or timer)" >&2
		exit 1
		;;
esac

echo "Done! CF CIDR sync will run daily at 03:00 and every 6 hours."
