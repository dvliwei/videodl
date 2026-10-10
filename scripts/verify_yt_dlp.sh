#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESOURCES_DIR="$PROJECT_ROOT/build/resources/yt-dlp"
DOWNLOAD_DIR="$RESOURCES_DIR/downloads"
VERSION="2026.08.19"

log() { echo "[verify_yt_dlp] $*"; }
fail() { echo "[verify_yt_dlp] FAIL: $*" >&2; exit 1; }

command -v sha256sum >/dev/null 2>&1 || command -v shasum >/dev/null 2>&1 || fail "sha256sum or shasum is required"

case "$(uname -s)" in
	Darwin) checksum_cmd="shasum -a 256" ;;
	*) checksum_cmd="sha256sum" ;;
esac

assets="yt-dlp.exe yt-dlp_linux yt-dlp_macos"
expected_checksum() {
	case "$1" in
		yt-dlp.exe) printf '%s' '66674953fe251b89f4d08c5f0e35e0728679bd67ab3d7d05c0562af101dd3e7a' ;;
		yt-dlp_linux) printf '%s' '58162f9bfdc27458ea47bfcb311cf47028f17d8154a8bf7d689861d46399230a' ;;
		yt-dlp_macos) printf '%s' '0f192b7ec147ab6288885d6351d9ab67367640029b4377576ef46dd79cf7b202' ;;
		*) return 1 ;;
	esac
}

[[ -f "$RESOURCES_DIR/manifest.yaml" ]] || fail "manifest.yaml not found"
[[ -f "$DOWNLOAD_DIR/SHA2-256SUMS" ]] || fail "missing $DOWNLOAD_DIR/SHA2-256SUMS; run make yt-dlp-fetch first"

for asset in $assets; do
	path="$DOWNLOAD_DIR/$asset"
	[[ -f "$path" ]] || fail "missing $path; run make yt-dlp-fetch first"
	if [[ "$checksum_cmd" == "shasum -a 256" ]]; then
		actual="$(shasum -a 256 "$path" | awk '{print $1}')"
	else
		actual="$(sha256sum "$path" | awk '{print $1}')"
	fi
	expected="$(expected_checksum "$asset")"
	[[ "$actual" == "$expected" ]] || fail "$asset SHA-256 mismatch: got $actual"
	log "$asset checksum verified"
done

[[ -s "$DOWNLOAD_DIR/THIRD_PARTY_LICENSES.txt" ]] || fail "missing upstream third-party license notice"
[[ -s "$DOWNLOAD_DIR/LICENSE" ]] || fail "missing upstream source license notice"
log "yt-dlp $VERSION assets and license materials verified"
