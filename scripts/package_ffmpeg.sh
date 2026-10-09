#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESOURCES_DIR="$PROJECT_ROOT/build/resources/ffmpeg"

BUNDLE_FFMPEG_DIR="${1:-}"
if [[ -z "$BUNDLE_FFMPEG_DIR" ]]; then
    echo "usage: $0 <bundle-ffmpeg-dir>" >&2
    echo "  macOS: $0 build/bin/videodl.app/Contents/Resources/ffmpeg" >&2
    echo "  Win/Linux: $0 build/bin/resources/ffmpeg" >&2
    exit 1
fi

if [[ ! -d "$BUNDLE_FFMPEG_DIR" ]]; then
    echo "package_ffmpeg: bundle ffmpeg dir not found at $BUNDLE_FFMPEG_DIR" >&2
    exit 1
fi

checksum_file="$BUNDLE_FFMPEG_DIR/SHA256SUMS.txt"
: > "$checksum_file"

while IFS= read -r -d '' f; do
    sha="$(shasum -a 256 "$f" | awk '{print $1}')"
    rel="${f#$BUNDLE_FFMPEG_DIR/}"
    echo "$sha  $rel" >> "$checksum_file"
done < <(find "$BUNDLE_FFMPEG_DIR" -type f -not -name 'SHA256SUMS.txt' -print0 | sort -z)

echo "package_ffmpeg: checksum file written to $checksum_file"
cat "$checksum_file"
