#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MANIFEST="${FFMPEG_MANIFEST:-$PROJECT_ROOT/build/resources/ffmpeg/manifest.yaml}"
TOOLS_ROOT="${FFMPEG_TOOLS_ROOT:-$PROJECT_ROOT/build/resources/ffmpeg/tools}"
LICENSES_ROOT="${FFMPEG_LICENSES_ROOT:-$PROJECT_ROOT/build/resources/ffmpeg/licenses}"
WORK_DIR="$(mktemp -d)"

cleanup() { rm -rf "$WORK_DIR"; }
trap cleanup EXIT

log() { echo "[fetch_ffmpeg] $*"; }

require_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "error: required command '$1' not found" >&2
        exit 1
    fi
}

sha256_file() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$1" | awk '{print $1}'
    else
        echo "error: neither sha256sum nor shasum is available" >&2
        exit 1
    fi
}

for cmd in curl python3 tar unzip awk; do
    require_command "$cmd"
done

validate_manifest() {
    python3 - "$MANIFEST" <<'PY'
import re
import sys
from pathlib import Path

text = Path(sys.argv[1]).read_text()
required = {
    "version": r'^  version: "8\.0\.3"$',
    "license": r'^  type: "LGPL-2\.1-or-later"$',
    "disable GPL": r'^    - "--disable-gpl"$',
    "disable nonfree": r'^    - "--disable-nonfree"$',
    "disable version3": r'^    - "--disable-version3"$',
    "disable autodetect": r'^    - "--disable-autodetect"$',
}
for name, pattern in required.items():
    if not re.search(pattern, text, re.MULTILINE):
        raise SystemExit(f"manifest: missing {name} policy")

for platform in ("windows-x64", "linux-x64", "darwin-x64", "darwin-arm64"):
    match = re.search(rf'^  {platform}:\n((?:    .*\n)+)', text, re.MULTILINE)
    if not match:
        raise SystemExit(f"manifest: missing target {platform}")
    block = match.group(1)
    digest = re.search(r'^    archive_sha256: "([0-9a-f]{64})"$', block, re.MULTILINE)
    if not digest:
        raise SystemExit(f"manifest: {platform} must have a 64-character archive SHA-256")
    if not re.search(r'^    archive_url: "https://', block, re.MULTILINE):
        raise SystemExit(f"manifest: {platform} archive URL must be HTTPS")
    if not re.search(r'^    configure_report_url: "https://', block, re.MULTILINE):
        raise SystemExit(f"manifest: {platform} is missing its configure report URL")

for forbidden in ("--enable-gpl", "--enable-nonfree", "--enable-version3"):
    if forbidden in text:
        raise SystemExit(f"manifest: forbidden build flag {forbidden}")
PY
}

parse_manifest() {
    python3 - "$MANIFEST" <<'PY'
import re
import sys
from pathlib import Path

text = Path(sys.argv[1]).read_text()
for match in re.finditer(r'^  ([a-z0-9-]+):\n((?:    .*\n)*)', text, re.MULTILINE):
    name, body = match.groups()
    if not re.search(r'^    goos:', body, re.MULTILINE):
        continue
    def get(key):
        found = re.search(rf'^    {key}:\s*"([^"]*)"$', body, re.MULTILINE)
        return found.group(1) if found else ""
    def get_tool(key):
        found = re.search(rf'^      {key}:\s*"([^"]*)"$', body, re.MULTILINE)
        return found.group(1) if found else ""
    print("|".join((name, get("archive_url"), get("archive_sha256"), get("archive_format"), get("extract_root"), get_tool("ffmpeg"), get_tool("ffprobe"))))
PY
}

download_and_extract() {
    local platform="$1" url="$2" expected_sha="$3" fmt="$4" extract_root="$5" ff_rel="$6" fp_rel="$7"
    local target_dir="$TOOLS_ROOT/$platform"
    local archive="$WORK_DIR/$platform.$fmt"

    log "Downloading $platform from pinned release asset"
    curl -fsSL --retry 3 -o "$archive" "$url"

    local actual_sha
    actual_sha="$(sha256_file "$archive")"
    if [ "$actual_sha" != "$expected_sha" ]; then
        echo "error: archive SHA-256 mismatch for $platform" >&2
        echo "  expected: $expected_sha" >&2
        echo "  actual:   $actual_sha" >&2
        exit 1
    fi
    log "$platform archive SHA-256 verified: $actual_sha"

    local extract_dir="$WORK_DIR/$platform-extract"
    mkdir -p "$extract_dir"
    case "$fmt" in
        zip) unzip -q "$archive" -d "$extract_dir" ;;
        tar.gz) tar -xzf "$archive" -C "$extract_dir" ;;
        *) echo "error: unsupported archive format $fmt" >&2; exit 1 ;;
    esac

    local src_bin_dir="$extract_dir"
    if [ -n "$extract_root" ] && [ -d "$extract_dir/$extract_root" ]; then
        src_bin_dir="$extract_dir/$extract_root"
    fi
    [ -f "$src_bin_dir/$ff_rel" ] || { echo "error: $platform missing $ff_rel after extraction" >&2; exit 1; }
    [ -f "$src_bin_dir/$fp_rel" ] || { echo "error: $platform missing $fp_rel after extraction" >&2; exit 1; }

    rm -rf "$target_dir"
    mkdir -p "$target_dir"
    cp "$src_bin_dir/$ff_rel" "$target_dir/"
    cp "$src_bin_dir/$fp_rel" "$target_dir/"
    chmod +x "$target_dir/$ff_rel" "$target_dir/$fp_rel" 2>/dev/null || true

    {
        printf '%s  %s\n' "$(sha256_file "$target_dir/$ff_rel")" "$ff_rel"
        printf '%s  %s\n' "$(sha256_file "$target_dir/$fp_rel")" "$fp_rel"
    } > "$target_dir/SHA256SUMS"
    log "$platform binaries extracted and SHA256SUMS written"
}

collect_licenses() {
    mkdir -p "$LICENSES_ROOT"
    curl -fsSL --retry 3 -o "$LICENSES_ROOT/FFmpeg-COPYING.LGPLv2.1" \
        "https://raw.githubusercontent.com/FFmpeg/FFmpeg/n8.0.3/COPYING.LGPLv2.1"
    [ -f "$LICENSES_ROOT/THIRD-PARTY-NOTICES.md" ] || {
        echo "error: missing $LICENSES_ROOT/THIRD-PARTY-NOTICES.md" >&2
        exit 1
    }
}

log "=== VideoDL FFmpeg Bootstrap ==="
log "Manifest: $MANIFEST"
validate_manifest
mkdir -p "$TOOLS_ROOT"

while IFS='|' read -r platform url sha fmt extract_root ff_rel fp_rel; do
    [ -z "$platform" ] && continue
    download_and_extract "$platform" "$url" "$sha" "$fmt" "$extract_root" "$ff_rel" "$fp_rel"
done < <(parse_manifest)

collect_licenses
log "All pinned FFmpeg assets and license materials fetched. Run scripts/verify_ffmpeg.sh."
