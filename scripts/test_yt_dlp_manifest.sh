#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MANIFEST="$PROJECT_ROOT/build/resources/yt-dlp/manifest.yaml"

fail() { echo "[test_yt_dlp_manifest] FAIL: $*" >&2; exit 1; }
pass() { echo "[test_yt_dlp_manifest] PASS: $*"; }

[[ -f "$MANIFEST" ]] || fail "manifest.yaml not found"
for key in schema_version yt_dlp license platforms; do
	grep -Eq "^${key}:" "$MANIFEST" || fail "missing top-level key: $key"
done
grep -Eq '^  version: "2026\.08\.19"$' "$MANIFEST" || fail "unexpected pinned version"
grep -Eq '^  channel: "stable"$' "$MANIFEST" || fail "stable channel not pinned"

for platform in windows-x64 linux-x64 darwin-universal; do
	grep -Eq "^  ${platform}:$" "$MANIFEST" || fail "missing platform: $platform"
done
for checksum in \
	66674953fe251b89f4d08c5f0e35e0728679bd67ab3d7d05c0562af101dd3e7a \
	58162f9bfdc27458ea47bfcb311cf47028f17d8154a8bf7d689861d46399230a \
	0f192b7ec147ab6288885d6351d9ab67367640029b4377576ef46dd79cf7b202; do
	grep -q "$checksum" "$MANIFEST" || fail "missing checksum: $checksum"
done

for license in \
	"build/resources/yt-dlp/licenses/yt-dlp-UNLICENSE.txt" \
	"build/resources/yt-dlp/licenses/THIRD_PARTY_LICENSES.txt" \
	"build/resources/yt-dlp/licenses/GPL-3.0-or-later.txt"; do
	[[ -s "$PROJECT_ROOT/$license" ]] || fail "missing license material: $license"
done

[[ -x "$PROJECT_ROOT/scripts/copy_yt_dlp_to_bundle.sh" ]] || fail "bundle copy script is not executable"
grep -q 'downloads/' "$PROJECT_ROOT/scripts/copy_yt_dlp_to_bundle.sh" || fail "bundle copy must use fetched release assets"
if grep -Eq 'command -v yt-dlp|exec\.LookPath\("yt-dlp"\)' \
	"$PROJECT_ROOT/internal/ytdlp"/*.go "$PROJECT_ROOT/scripts/copy_yt_dlp_to_bundle.sh"; then
	fail "yt-dlp integration must not fall back to PATH"
fi

for platform in windows-x64 linux-x64 darwin-x64 darwin-arm64; do
	grep -q "$platform" "$PROJECT_ROOT/internal/ytdlp/binary.go" || fail "runtime resolver missing $platform"
done

pass "manifest, stable release pin, checksums, and license records are present"
