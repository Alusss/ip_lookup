#!/usr/bin/env bash
set -euo pipefail

# GeoLite2 GeoIP Database Auto-Update Script
# Downloads the free GeoLite2 City + ASN databases from MaxMind.
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
#
# City is required; ASN is optional -- if the ASN download fails the script
# only warns and the Go backend runs without ASN (graceful degradation).

LICENSE_KEY="${1:-${MAXMIND_LICENSE_KEY:-}}"
DOWNLOAD_URL="https://download.maxmind.com/app/geoip_download"
DEST_DIR="/var/lib/ip-lookup"
MAX_RETRIES=3
RETRY_DELAY=10
TEMP_DIR=$(mktemp -d)

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

# download_and_install EDITION
# Echoes status lines; returns 0 on success/unchanged, 1 on failure.
download_and_install() {
	local edition="$1"
	local dest_file="${DEST_DIR}/${edition}.mmdb"
	local work="${TEMP_DIR}/${edition}"
	mkdir -p "$work"

	echo "[INFO] Downloading ${edition} database..."
	local dl_ok=""
	local retry_delay=$RETRY_DELAY
	for attempt in $(seq 1 "${MAX_RETRIES}"); do
		if curl -sS -L --fail --max-time 120 \
			-o "${work}/${edition}.tar.gz" \
			"${DOWNLOAD_URL}?edition_id=${edition}&license_key=${LICENSE_KEY}&suffix=tar.gz"; then
			dl_ok=1
			break
		fi
		echo "[WARN] ${edition}: download attempt ${attempt}/${MAX_RETRIES} failed, retrying in ${retry_delay}s..." >&2
		sleep "${retry_delay}"
		retry_delay=$((retry_delay * 2))
	done

	if [ -z "${dl_ok}" ]; then
		echo "[ERROR] ${edition}: download failed after ${MAX_RETRIES} attempts" >&2
		return 1
	fi

	echo "[INFO] ${edition}: extracting..."
	if ! tar xzf "${work}/${edition}.tar.gz" -C "${work}" --strip-components=1; then
		echo "[ERROR] ${edition}: extraction failed" >&2
		return 1
	fi

	local mmdb_file
	mmdb_file=$(find "${work}" -name '*.mmdb' 2>/dev/null | head -1 || true)
	if [ -z "$mmdb_file" ]; then
		echo "[ERROR] ${edition}: no .mmdb file found in archive" >&2
		return 1
	fi

	if [ -f "${dest_file}" ] && cmp -s "$mmdb_file" "$dest_file"; then
		echo "[INFO] ${edition}: unchanged, skipping"
		return 0
	fi

	cp "$mmdb_file" "${dest_file}.tmp" || return 1
	mv "${dest_file}.tmp" "${dest_file}" || return 1
	echo "[INFO] ${edition}: updated -> ${dest_file} ($(du -h "$dest_file" | cut -f1))"
	return 0
}

CITY_OK=0
ASN_OK=0

download_and_install "GeoLite2-City" || CITY_OK=1
download_and_install "GeoLite2-ASN" || ASN_OK=1

if [ "$CITY_OK" -ne 0 ]; then
	echo "[ERROR] City database (required) failed to update" >&2
	exit 1
fi

if [ "$ASN_OK" -ne 0 ]; then
	echo "[WARN] ASN database (optional) failed to update; backend will run without ASN" >&2
fi

if systemctl is-active --quiet ip-lookup 2>/dev/null; then
	echo "[INFO] Go backend will detect changes via fsnotify (no restart needed)"
fi

echo "[INFO] Done"
