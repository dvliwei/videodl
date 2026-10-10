#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESOURCES_SRC="$PROJECT_ROOT/build/resources/yt-dlp"
MANIFEST="$RESOURCES_SRC/manifest.yaml"

TARGET="${1:-}"
BUNDLE_PLATFORM="${2:-}"
if [[ -z "$TARGET" || -z "$BUNDLE_PLATFORM" ]]; then
    echo "usage: $0 <bundle-root> <platform>" >&2
    exit 1
fi

case "$BUNDLE_PLATFORM" in
    windows-x64|linux-x64|darwin-x64|darwin-arm64) ;;
    *) echo "copy_yt_dlp_to_bundle: unknown platform '$BUNDLE_PLATFORM'" >&2; exit 1 ;;
esac

if [[ "$BUNDLE_PLATFORM" == darwin-* ]]; then
    PLATFORM_KEY="darwin-universal"
else
    PLATFORM_KEY="$BUNDLE_PLATFORM"
fi

asset="$(awk -v section="$PLATFORM_KEY" '
    $0 == "  " section ":" { in_section=1; next }
    in_section && $0 ~ /^  [a-z0-9-]+:/ { exit }
    in_section && $1 == "asset:" { gsub(/"/, "", $2); print $2; exit }
' "$MANIFEST")"
resource_dir="$(awk -v section="$PLATFORM_KEY" '
    $0 == "  " section ":" { in_section=1; next }
    in_section && $0 ~ /^  [a-z0-9-]+:/ { exit }
    in_section && $1 == "resource_dir:" { gsub(/"/, "", $2); print $2; exit }
' "$MANIFEST")"

[[ -n "$asset" && -n "$resource_dir" ]] || { echo "copy_yt_dlp_to_bundle: manifest entry missing" >&2; exit 1; }
source="$RESOURCES_SRC/downloads/$asset"
[[ -s "$source" ]] || { echo "copy_yt_dlp_to_bundle: missing fetched asset $source; run make yt-dlp-fetch" >&2; exit 1; }

checksum="$(shasum -a 256 "$source" | awk '{print $1}')"
grep -q "$checksum" "$MANIFEST" || { echo "copy_yt_dlp_to_bundle: checksum is not recorded in manifest" >&2; exit 1; }

case "$(uname -s)" in
    Darwin) DEST="$TARGET/Resources/yt-dlp" ;;
    *) DEST="$TARGET/resources/yt-dlp" ;;
esac

rm -rf "$DEST"
mkdir -p "$DEST/tools/$resource_dir" "$DEST/licenses"
cp "$source" "$DEST/tools/$resource_dir/$asset"
cp "$MANIFEST" "$DEST/"
cp "$RESOURCES_SRC/CREDITS.md" "$DEST/"
cp "$RESOURCES_SRC/THIRD-PARTY-NOTICES.txt" "$DEST/"
cp -R "$RESOURCES_SRC/licenses/." "$DEST/licenses/"

if [[ "$BUNDLE_PLATFORM" != windows-x64 ]]; then
    chmod 755 "$DEST/tools/$resource_dir/$asset"
fi

echo "copy_yt_dlp_to_bundle: installed $BUNDLE_PLATFORM resources into $DEST"
find "$DEST" -type f | sort
