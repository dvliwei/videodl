#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MANIFEST="$PROJECT_ROOT/build/resources/ffmpeg/manifest.yaml"

python3 - "$MANIFEST" <<'PY'
import re
import sys
from pathlib import Path

manifest = Path(sys.argv[1]).read_text()

def require(pattern, message):
    if not re.search(pattern, manifest, re.MULTILINE):
        raise SystemExit(f"manifest check failed: {message}")

require(r'^  version: "8\.0\.3"$', "FFmpeg version must be 8.0.3")
require(r'^  type: "LGPL-2\.1-or-later"$', "license must be LGPL-2.1-or-later")
require(r'^    - "--disable-gpl"$', "GPL must be disabled")
require(r'^    - "--disable-nonfree"$', "nonfree components must be disabled")
require(r'^    - "--disable-version3"$', "version 3 components must be disabled")

for platform, digest in {
    "windows-x64": "fd3473736674343cd1948eb45f28b46f377616432a5cc15eb896bdaabea506f2",
    "linux-x64": "2612ed26322c864a7411c3578b74fe7f229127e6f9f906d3aa994937a4c9620e",
    "darwin-x64": "b84b4515aaf2fe443a76445f02d05544a384dcd46cc6059fb8a1a4f9aaa3035d",
    "darwin-arm64": "26f6269b117a51c5fdfd3a870c4e0a62b99f0575740a3933fc3d852f0a3f80b1",
}.items():
    block = re.search(rf'^  {platform}:\n((?:    .*\n)+)', manifest, re.MULTILINE)
    if not block:
        raise SystemExit(f"manifest check failed: missing platform {platform}")
    body = block.group(1)
    if f'archive_sha256: "{digest}"' not in body:
        raise SystemExit(f"manifest check failed: wrong archive hash for {platform}")
    if not re.search(r'^    configure_report_url: "https://', body, re.MULTILINE):
        raise SystemExit(f"manifest check failed: missing configure report for {platform}")

for forbidden in ("--enable-libx264", "--enable-libx265", "--enable-libfdk-aac", "--enable-gpl", "--enable-nonfree"):
    if forbidden in manifest:
        raise SystemExit(f"manifest check failed: forbidden build feature {forbidden}")

print("FFmpeg manifest checks passed")
PY
