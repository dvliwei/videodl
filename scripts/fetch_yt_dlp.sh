#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESOURCES_DIR="$PROJECT_ROOT/build/resources/yt-dlp"
DOWNLOAD_DIR="$RESOURCES_DIR/downloads"
LICENSE_DIR="$RESOURCES_DIR/licenses"
VERSION="2026.08.19"
BASE_URL="https://github.com/yt-dlp/yt-dlp/releases/download/$VERSION"
SOURCE_BASE_URL="https://github.com/yt-dlp/yt-dlp/raw/$VERSION"

log() { echo "[fetch_yt_dlp] $*"; }
fail() { echo "[fetch_yt_dlp] ERROR: $*" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || fail "curl is required"
mkdir -p "$DOWNLOAD_DIR" "$LICENSE_DIR"

for asset in yt-dlp.exe yt-dlp_linux yt-dlp_macos SHA2-256SUMS; do
	log "fetching $asset"
	curl --fail --location --retry 5 --retry-all-errors --connect-timeout 15 --proto '=https' --tlsv1.2 \
		"$BASE_URL/$asset" -o "$DOWNLOAD_DIR/$asset"
done

for asset in THIRD_PARTY_LICENSES.txt LICENSE; do
	log "fetching source license material $asset"
	curl --fail --location --retry 5 --retry-all-errors --connect-timeout 15 --proto '=https' --tlsv1.2 \
		"$SOURCE_BASE_URL/$asset" -o "$DOWNLOAD_DIR/$asset"
done

cp "$DOWNLOAD_DIR/THIRD_PARTY_LICENSES.txt" "$LICENSE_DIR/THIRD_PARTY_LICENSES.upstream.txt"
cp "$DOWNLOAD_DIR/LICENSE" "$LICENSE_DIR/yt-dlp-UNLICENSE.upstream.txt"

bash "$SCRIPT_DIR/verify_yt_dlp.sh"
log "official yt-dlp $VERSION assets downloaded and verified"
