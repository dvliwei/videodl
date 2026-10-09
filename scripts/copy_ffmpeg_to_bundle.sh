#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESOURCES_SRC="$PROJECT_ROOT/build/resources/ffmpeg"

if [[ ! -d "$RESOURCES_SRC" ]]; then
    echo "copy_ffmpeg_to_bundle: no resources at $RESOURCES_SRC" >&2
    exit 1
fi

TARGET="${1:-}"
BUNDLE_PLATFORM="${2:-}"

if [[ -z "$TARGET" || -z "$BUNDLE_PLATFORM" ]]; then
    echo "usage: $0 <bundle-root> <platform>" >&2
    echo "  macOS ARM64:   $0 build/bin/videodl.app/Contents darwin-arm64" >&2
    echo "  macOS x86_64:  $0 build/bin/videodl.app/Contents darwin-x64" >&2
    echo "  Windows x86_64:$0 build/bin windows-x64" >&2
    echo "  Linux x86_64:  $0 build/bin linux-x64" >&2
    exit 1
fi

if [[ ! -d "$TARGET" ]]; then
    echo "copy_ffmpeg_to_bundle: target $TARGET does not exist" >&2
    exit 1
fi

case "$BUNDLE_PLATFORM" in
    darwin-arm64|darwin-x64|windows-x64|linux-x64) ;;
    *)
        echo "copy_ffmpeg_to_bundle: unknown platform '$BUNDLE_PLATFORM'" >&2
        exit 1
        ;;
esac

TOOLS_SRC="$RESOURCES_SRC/tools/$BUNDLE_PLATFORM"
if [[ ! -d "$TOOLS_SRC" ]]; then
    echo "copy_ffmpeg_to_bundle: no tools for $BUNDLE_PLATFORM at $TOOLS_SRC" >&2
    echo "run 'make ffmpeg-fetch' first" >&2
    exit 1
fi

case "$(uname -s)" in
    Darwin)
        DEST="$TARGET/Resources/ffmpeg"
        ;;
    *)
        DEST="$TARGET/resources/ffmpeg"
        ;;
esac

rm -rf "$DEST"
mkdir -p "$DEST/tools"

cp -R "$TOOLS_SRC" "$DEST/tools/"
cp "$RESOURCES_SRC/manifest.yaml" "$DEST/"
if [[ -d "$RESOURCES_SRC/licenses" ]]; then
    cp -R "$RESOURCES_SRC/licenses" "$DEST/"
fi
if [[ -f "$RESOURCES_SRC/CREDITS.md" ]]; then
    cp "$RESOURCES_SRC/CREDITS.md" "$DEST/"
fi

chmod +x "$DEST/tools/$BUNDLE_PLATFORM"/ffmpeg* 2>/dev/null || true

echo "copy_ffmpeg_to_bundle: installed $BUNDLE_PLATFORM resources into $DEST"
du -sh "$DEST"
find "$DEST" -type f | sort
