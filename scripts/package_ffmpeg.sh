#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESOURCES_DIR="$PROJECT_ROOT/build/resources/ffmpeg"

TARGET_PLATFORM="${1:-}"
if [[ -z "$TARGET_PLATFORM" ]]; then
    echo "usage: $0 <platform-dir>  (e.g. darwin-arm64, windows-x64)" >&2
    exit 1
fi

TOOLS_SRC="$RESOURCES_DIR/tools/$TARGET_PLATFORM"
if [[ ! -d "$TOOLS_SRC" ]]; then
    echo "package_ffmpeg: no tools for $TARGET_PLATFORM at $TOOLS_SRC" >&2
    echo "run 'make ffmpeg-fetch' first" >&2
    exit 1
fi

checksum_file="$RESOURCES_DIR/SHA256SUMS.txt"
: > "$checksum_file"

for tooldir in "$TOOLS_SRC"; do
    find "$tooldir" -type f | sort | while read -r f; do
        sha="$(shasum -a 256 "$f" | awk '{print $1}')"
        rel="${f#$RESOURCES_DIR/}"
        echo "$sha  $rel" >> "$checksum_file"
    done
done

echo "package_ffmpeg: checksum file written to $checksum_file"
cat "$checksum_file"
