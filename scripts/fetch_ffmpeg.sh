#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESOURCES_DIR="$PROJECT_ROOT/build/resources/ffmpeg"
TOOLS_DIR="$RESOURCES_DIR/tools"
TMP_DIR="$(mktemp -d)"

cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

log() { echo "[fetch_ffmpeg] $*"; }
fail() { echo "[fetch_ffmpeg] ERROR: $*" >&2; exit 1; }

require_cmd() { command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"; }

require_cmd curl
require_cmd sha256sum
require_cmd python3

MANIFEST="$RESOURCES_DIR/manifest.yaml"
[[ -f "$MANIFEST" ]] || fail "manifest.yaml not found at $MANIFEST"

log "Parsing manifest.yaml"

parse_manifest_field() {
    local platform="$1" field="$2"
    python3 - "$MANIFEST" "$platform" "$field" <<'PY'
import sys, re, pathlib
text = pathlib.Path(sys.argv[1]).read_text()
platform = sys.argv[2]
field = sys.argv[3]
in_block = False
indent_platform = -1
for line in text.splitlines():
    stripped = line.rstrip()
    if not stripped or stripped.startswith('#'):
        continue
    m = re.match(r'^(\s*)([a-z][-_a-z0-9]*):\s*(.*)$', stripped)
    if not m:
        continue
    indent = len(m.group(1))
    key = m.group(2)
    value = m.group(3).strip()
    if not in_block:
        if key == platform and value == '':
            in_block = True
            indent_platform = indent + 2
        continue
    if indent < indent_platform:
        break
    if indent == indent_platform and key == field:
        v = value.strip('"').strip("'")
        print(v)
        sys.exit(0)
print('')
PY
}

PLATFORMS="windows-x64 linux-x64 darwin-x64 darwin-arm64"

for platform in $PLATFORMS; do
    log "--- $platform ---"

    archive_url="$(parse_manifest_field "$platform" archive_url)"
    archive_sha="$(parse_manifest_field "$platform" archive_sha256)"
    archive_format="$(parse_manifest_field "$platform" archive_format)"
    extract_root="$(parse_manifest_field "$platform" extract_root)"

    [[ -n "$archive_url" ]] || fail "no archive_url for $platform"
    [[ -n "$archive_sha" ]] || fail "no archive_sha256 for $platform"

    target_dir="$TOOLS_DIR/$platform"
    mkdir -p "$target_dir"

    archive_name="$(basename "$archive_url")"
    archive_path="$TMP_DIR/$archive_name"

    if [[ -f "$archive_path" ]]; then
        log "reusing cached $archive_name"
    else
        log "downloading $archive_url"
        # GitHub's release asset CDN can intermittently fail during HTTP/2 framing
        # or stall on macOS/proxy combinations. Use HTTP/1.1, resume partial files,
        # and retry transport failures, including curl error 16.
        curl --http1.1 -fL --retry 5 --retry-all-errors --retry-delay 2 \
            --connect-timeout 30 --speed-limit 1024 --speed-time 30 \
            --continue-at - -o "$archive_path" "$archive_url"
    fi

    log "verifying sha256"
    got_sha="$(sha256sum "$archive_path" | awk '{print $1}')"
    if [[ "$got_sha" != "$archive_sha" ]]; then
        fail "sha256 mismatch for $platform: got $got_sha, want $archive_sha"
    fi
    log "sha256 ok"

    extract_dir="$TMP_DIR/extract_$platform"
    mkdir -p "$extract_dir"

    case "$archive_format" in
        zip)
            require_cmd unzip
            log "extracting zip"
            unzip -q -o "$archive_path" -d "$extract_dir"
            ;;
        tar.gz)
            log "extracting tar.gz"
            tar xzf "$archive_path" -C "$extract_dir"
            ;;
        *)
            fail "unknown archive format: $archive_format"
            ;;
    esac

    source_root="$extract_dir"
    if [[ -n "$extract_root" ]]; then
        source_root="$extract_dir/$extract_root"
    fi

    find "$target_dir" -maxdepth 1 -type f -delete

    if [[ "$platform" == windows-* ]]; then
        cp "$source_root/ffmpeg.exe" "$target_dir/ffmpeg.exe"
        cp "$source_root/ffprobe.exe" "$target_dir/ffprobe.exe"
    else
        cp "$source_root/ffmpeg" "$target_dir/ffmpeg"
        cp "$source_root/ffprobe" "$target_dir/ffprobe"
        chmod +x "$target_dir/ffmpeg" "$target_dir/ffprobe"
    fi

    log "installed $platform -> $target_dir"
done

log "All platforms fetched and verified."
log "Resources tree:"
find "$RESOURCES_DIR" -type f -o -type d | sort
