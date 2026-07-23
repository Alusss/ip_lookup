#!/usr/bin/env bash
set -euo pipefail

# GeoLite2 GeoIP Database Auto-Update Script
# Downloads the free GeoLite2 City database from MaxMind.
#
# Prerequisites:
#   1. Sign up for a free MaxMind account at https://www.maxmind.com/en/geolite2/signup
#   2. Generate a license key at https://www.maxmind.com/en/accounts/current/license-key
#   3. Set MAXMIND_LICENSE_KEY environment variable or pass as first argument
#
# Usage:
#   sudo bash deploy/scripts/update-geoip.sh [license_key]
#
# Auto-update (cron):
#   0 4 * * 0 root /etc/ip-lookup/update-geoip.sh  # weekly on Sunday

LICENSE_KEY="${1:-${MAXMIND_LICENSE_KEY:-}}"
DOWNLOAD_URL="https://download.maxmind.com/app/geoip_download"
EDITION="GeoLite2-City"
TEMP_DIR=$(mktemp -d)
DEST_DIR="/var/lib/ip-lookup"
DEST_FILE="${DEST_DIR}/${EDITION}.mmdb"
MAX_RETRIES=3
RETRY_DELAY=10

cleanup() {
	rm -rf "${TEMP_DIR}"
}
trap cleanup EXIT

if [ -z "$LICENSE_KEY" ]; then
	echo "ERROR: MaxMind license key required." >&2
	echo "Set MAXMIND_LICENSE_KEY env var or pass as argument." >&2
	exit 1
fi

mkdir -p "${DEST_DIR}"

echo "[INFO] Downloading ${EDITION} database..."
STATUS=""
for attempt in $(seq 1 "${MAX_RETRIES}"); do
	STATUS=$(curl -sS -L --fail --max-time 120 \
		-o "${TEMP_DIR}/${EDITION}.tar.gz" \
		-w "%{http_code}" \
		"${DOWNLOAD_URL}?edition_id=${EDITION}&license_key=${LICENSE_KEY}&suffix=tar.gz") && {
		DOWNLOAD_OK=1
		break
	}
	echo "[WARN] Download attempt ${attempt}/${MAX_RETRIES} failed (HTTP ${STATUS:-timeout}), retrying in ${RETRY_DELAY}s..." >&2
	sleep "${RETRY_DELAY}"
	RETRY_DELAY=$((RETRY_DELAY * 2))
done
if [ -z "${DOWNLOAD_OK:-}" ]; then
	echo "[ERROR] Download failed after ${MAX_RETRIES} attempts" >&2
	exit 1
fi

echo "[INFO] Extracting..."
tar xzf "${TEMP_DIR}/${EDITION}.tar.gz" -C "${TEMP_DIR}" --strip-components=1

MMDB_FILE=$(find "${TEMP_DIR}" -name '*.mmdb' | head -1)
if [ -z "$MMDB_FILE" ]; then
	echo "[ERROR] No .mmdb file found in archive" >&2
	exit 1
fi

if [ -f "${DEST_FILE}" ]; then
	if cmp -s "$MMDB_FILE" "$DEST_FILE"; then
		echo "[INFO] Database unchanged, skipping update"
		exit 0
	fi
fi

cp "$MMDB_FILE" "${DEST_FILE}.tmp"
mv "${DEST_FILE}.tmp" "${DEST_FILE}"

echo "[INFO] Database updated: ${DEST_FILE} ($(du -h "$DEST_FILE" | cut -f1))"

if systemctl is-active --quiet ip-lookup 2>/dev/null; then
	echo "[INFO] Go backend will detect the change via fsnotify (no restart needed)"
fi

echo "[INFO] Done"
