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
if [[ -z "$TARGET" ]]; then
    echo "usage: $0 <bundle-root>" >&2
    echo "  macOS: $0 build/bin/videodl.app/Contents" >&2
    echo "  Windows/Linux: $0 build/bin" >&2
    exit 1
fi

if [[ ! -d "$TARGET" ]]; then
    echo "copy_ffmpeg_to_bundle: target $TARGET does not exist" >&2
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

mkdir -p "$DEST"

cp -R "$RESOURCES_SRC/tools" "$DEST/"
cp "$RESOURCES_SRC/manifest.yaml" "$DEST/"
if [[ -d "$RESOURCES_SRC/licenses" ]]; then
    cp -R "$RESOURCES_SRC/licenses" "$DEST/"
fi
if [[ -f "$RESOURCES_SRC/SHA256SUMS.txt" ]]; then
    cp "$RESOURCES_SRC/SHA256SUMS.txt" "$DEST/"
fi
if [[ -f "$RESOURCES_SRC/CREDITS.md" ]]; then
    cp "$RESOURCES_SRC/CREDITS.md" "$DEST/"
fi

chmod +x "$DEST/tools"/*/ffmpeg* 2>/dev/null || true

echo "copy_ffmpeg_to_bundle: installed resources into $DEST"
du -sh "$DEST"
find "$DEST" -type f | sort
