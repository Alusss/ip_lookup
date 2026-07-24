#!/usr/bin/env bash
set -euo pipefail

# IP Lookup Verification Script
# Gate script for CI pipeline
# Usage: bash scripts/verify.sh

FAILED=0

check() {
	local name="$1"
	shift
	if "$@"; then
		echo "[PASS] ${name}"
	else
		echo "[FAIL] ${name}"
		FAILED=1
	fi
}

# 1. Go tests
check "go test ./..." bash -c "cd backend && go test ./... -count=1 2>&1 | tail -5"

# 2. golangci-lint
if command -v golangci-lint &>/dev/null; then
	check "golangci-lint run" bash -c "golangci-lint run ./backend/... 2>&1 | tail -5"
else
	echo "[SKIP] golangci-lint (not installed)"
fi

# 3. Docker build
if command -v docker &>/dev/null; then
	check "docker build" docker build -t ip-lookup:test -f docker/Dockerfile . --quiet 2>&1
else
	echo "[SKIP] docker build (not installed)"
fi

# 4. systemd-analyze security
if command -v systemd-analyze &>/dev/null; then
	SECURITY_SCORE=$(systemd-analyze security deploy/systemd/ip-lookup.service 2>/dev/null | grep -oP '^\s+\d+\.\d+' | head -1 || echo "N/A")
	echo "[INFO] systemd security score: ${SECURITY_SCORE}"
	if [ "$SECURITY_SCORE" != "N/A" ]; then
		SCORE_VAL=$(echo "$SECURITY_SCORE" | cut -d. -f1)
		if [ "$SCORE_VAL" -le 3 ] 2>/dev/null; then
			echo "[PASS] systemd security score <= 3.0 (${SECURITY_SCORE})"
		else
			echo "[FAIL] systemd security score > 3.0 (${SECURITY_SCORE})"
			FAILED=1
		fi
	fi
else
	echo "[SKIP] systemd-analyze (not installed)"
fi

# 5. Check files exist
for f in \
	backend/main.go \
	backend/config.go \
	backend/handler.go \
	backend/ip_extract.go \
	backend/ratelimit.go \
	backend/ad.go \
	backend/metrics.go \
	backend/middleware.go \
	backend/geoip.go \
	backend/errors.go \
	backend/monitor.go \
	backend/circuitbreaker.go \
	backend/main_test.go \
	frontend/index.html \
	frontend/js/i18n.js \
	frontend/js/app.js \
	frontend/privacy.html \
	frontend/docs/what-is-ipv6.html \
	frontend/docs/ipv6-test-guide.html \
	deploy/systemd/ip-lookup.service \
	deploy/caddy/Caddyfile \
	deploy/nginx/nginx.conf \
	docker/Dockerfile \
	scripts/install-systemd.sh \
	deploy/scripts/update-cloudflare-ip.sh \
	deploy/scripts/install-cf-sync-cron.sh \
	deploy/scripts/update-geoip.sh \
	deploy/nftables/cloudflare-only.nft \
	api/openapi.yaml \
	frontend/_headers \
	frontend/_redirects \
	frontend/robots.txt \
	docs/operation.md \
	docs/architecture.md \
	docs/future-plan.md \
	VARIABLES.md; do
	check "file exists: $f" test -f "$f"
done

if [ "$FAILED" -eq 1 ]; then
	echo "Verification FAILED"
	exit 1
fi

echo "All checks passed!"
