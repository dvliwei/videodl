#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MANIFEST="${FFMPEG_MANIFEST:-$PROJECT_ROOT/build/resources/ffmpeg/manifest.yaml}"
TOOLS_ROOT="${FFMPEG_TOOLS_ROOT:-$PROJECT_ROOT/build/resources/ffmpeg/tools}"
LICENSES_ROOT="${FFMPEG_LICENSES_ROOT:-$PROJECT_ROOT/build/resources/ffmpeg/licenses}"

log() { echo "[verify_ffmpeg] $*"; }
ok() { echo "  OK:   $*"; }
fail() { echo "  FAIL: $*" >&2; exit_code=1; }

exit_code=0

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

validate_manifest() {
    python3 - "$MANIFEST" <<'PY'
import re
import sys
from pathlib import Path

text = Path(sys.argv[1]).read_text()
required = (
    (r'^  version: "8\.0\.3"$', "FFmpeg version"),
    (r'^  type: "LGPL-2\.1-or-later"$', "license type"),
    (r'^    - "--disable-gpl"$', "--disable-gpl"),
    (r'^    - "--disable-nonfree"$', "--disable-nonfree"),
    (r'^    - "--disable-version3"$', "--disable-version3"),
    (r'^    - "--disable-autodetect"$', "--disable-autodetect"),
)
for pattern, name in required:
    if not re.search(pattern, text, re.MULTILINE):
        raise SystemExit(f"manifest missing {name}")

for platform in ("windows-x64", "linux-x64", "darwin-x64", "darwin-arm64"):
    match = re.search(rf'^  {platform}:\n((?:    .*\n)+)', text, re.MULTILINE)
    if not match:
        raise SystemExit(f"manifest missing {platform}")
    block = match.group(1)
    if not re.search(r'^    archive_sha256: "[0-9a-f]{64}"$', block, re.MULTILINE):
        raise SystemExit(f"manifest has no locked SHA-256 for {platform}")
    if not re.search(r'^    configure_report_url: "https://', block, re.MULTILINE):
        raise SystemExit(f"manifest has no configure report for {platform}")

for forbidden in ("--enable-gpl", "--enable-nonfree", "--enable-version3"):
    if forbidden in text:
        raise SystemExit(f"manifest contains forbidden flag {forbidden}")
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
    print("|".join((name, get("archive_sha256"), get("archive_format"), get_tool("ffmpeg"), get_tool("ffprobe"))))
PY
}

verify_binary_checksum() {
    local dir="$1" name="$2" checksum_file="$dir/SHA256SUMS"
    if [ ! -f "$checksum_file" ]; then
        fail "$dir: SHA256SUMS missing; extracted binaries are not locked"
        return
    fi
    local expected actual
    expected="$(awk -v name="$name" '$2 == name { print $1 }' "$checksum_file")"
    if ! [[ "$expected" =~ ^[0-9a-f]{64}$ ]]; then
        fail "$dir/$name: SHA256SUMS entry missing or malformed"
        return
    fi
    actual="$(sha256_file "$dir/$name")"
    if [ "$actual" != "$expected" ]; then
        fail "$dir/$name: SHA-256 mismatch (expected $expected, got $actual)"
    else
        ok "$dir/$name: SHA-256 verified ($actual)"
    fi
}

log "=== VideoDL FFmpeg Verification ==="
log "Manifest: $MANIFEST"
validate_manifest || {
    echo "manifest validation failed" >&2
    exit 1
}

for license_file in "$LICENSES_ROOT/FFmpeg-COPYING.LGPLv2.1" "$LICENSES_ROOT/THIRD-PARTY-NOTICES.md"; do
    if [ -f "$license_file" ]; then
        ok "license material present: $license_file"
    else
        fail "license material missing: $license_file"
    fi
done

while IFS='|' read -r platform expected_archive_sha fmt ff_rel fp_rel; do
    [ -z "$platform" ] && continue
    dir="$TOOLS_ROOT/$platform"
    if [ ! -d "$dir" ]; then
        fail "$platform: tools directory missing at $dir"
        continue
    fi
    ok "$platform: tools directory exists"

    for tool in "$ff_rel" "$fp_rel"; do
        bin="$dir/$tool"
        if [ ! -f "$bin" ]; then
            fail "$platform: $tool missing at $bin"
            continue
        fi
        ok "$platform: $tool present"
        if [ "$platform" != "windows-x64" ]; then
            if [ ! -x "$bin" ]; then
                fail "$platform: $tool is not executable"
            else
                ok "$platform: $tool executable"
            fi
        fi
        verify_binary_checksum "$dir" "$tool"
    done
done < <(parse_manifest)

if [ "$exit_code" -eq 0 ]; then
    log "All FFmpeg binaries, checksums, manifest fields and license materials verified."
else
    log "FFmpeg verification FAILED."
fi
exit "$exit_code"
