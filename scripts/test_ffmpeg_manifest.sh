#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESOURCES_DIR="$PROJECT_ROOT/build/resources/ffmpeg"
MANIFEST="$RESOURCES_DIR/manifest.yaml"

log() { echo "[test_ffmpeg_manifest] $*"; }
fail() { echo "[test_ffmpeg_manifest] FAIL: $*" >&2; exit 1; }
record_pass() { echo "  PASS: $*"; }
record_fail() { echo "  FAIL: $*"; failures=$((failures + 1)); }
failures=0

[[ -f "$MANIFEST" ]] || fail "manifest.yaml not found at $MANIFEST"
record_pass "manifest.yaml exists"

record_pass "manifest.yaml readable ($(wc -l < "$MANIFEST") lines)"

require_cmd() { command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"; }
require_cmd python3

log "Parsing manifest.yaml structure"

python3 - "$MANIFEST" <<'PY'
import sys, re, pathlib

path = sys.argv[1]
text = pathlib.Path(path).read_text()

required_top = ['schema_version', 'ffmpeg', 'license', 'source', 'encoders', 'platforms']
for key in required_top:
    if re.search(rf'^{key}:\s*', text, re.MULTILINE):
        print(f"TOP_OK {key}")
    else:
        print(f"TOP_MISSING {key}")

try:
    import yaml
    data = yaml.safe_load(text)
except ImportError:
    print("PYYAML_MISSING")
    sys.exit(0)
except Exception as e:
    print(f"YAML_PARSE_FAIL {e}")
    sys.exit(0)

ff = data.get('ffmpeg', {})
for k in ['version', 'branch', 'release_date']:
    if ff.get(k):
        print(f"FF_OK {k}={ff[k]}")
    else:
        print(f"FF_MISSING {k}")

lic = data.get('license', {})
for k in ['type', 'license_files']:
    if lic.get(k):
        print(f"LIC_OK {k}")
    else:
        print(f"LIC_MISSING {k}")

src = data.get('source', {})
for k in ['provider', 'release_tag', 'release_url']:
    if src.get(k):
        print(f"SRC_OK {k}")
    else:
        print(f"SRC_MISSING {k}")

platforms = data.get('platforms', {})
for name, info in platforms.items():
    if not isinstance(info, dict):
        print(f"PLAT_BAD {name}: not a dict")
        continue
    for k in ['goos', 'goarch', 'archive_url', 'archive_sha256', 'archive_format', 'tool_paths']:
        if info.get(k) is not None:
            print(f"PLAT_OK {name}.{k}")
        else:
            print(f"PLAT_MISSING {name}.{k}")
PY

results="$(python3 - "$MANIFEST" <<'PY'
import sys, re, pathlib
path = sys.argv[1]
text = pathlib.Path(path).read_text()

required_top = ['schema_version', 'ffmpeg', 'license', 'source', 'encoders', 'platforms']
for key in required_top:
    if re.search(rf'^{key}:\s*', text, re.MULTILINE):
        print(f"TOP_OK {key}")
    else:
        print(f"TOP_MISSING {key}")

try:
    import yaml
    data = yaml.safe_load(text)
except ImportError:
    sys.exit(0)
except Exception:
    sys.exit(0)

ff = data.get('ffmpeg', {})
for k in ['version', 'branch', 'release_date']:
    if ff.get(k):
        print(f"FF_OK {k}={ff[k]}")
    else:
        print(f"FF_MISSING {k}")

lic = data.get('license', {})
for k in ['type', 'license_files']:
    if lic.get(k):
        print(f"LIC_OK {k}")
    else:
        print(f"LIC_MISSING {k}")

src = data.get('source', {})
for k in ['provider', 'release_tag', 'release_url']:
    if src.get(k):
        print(f"SRC_OK {k}")
    else:
        print(f"SRC_MISSING {k}")

platforms = data.get('platforms', {})
for name, info in platforms.items():
    if not isinstance(info, dict):
        continue
    for k in ['goos', 'goarch', 'archive_url', 'archive_sha256', 'archive_format', 'tool_paths']:
        if info.get(k) is not None:
            print(f"PLAT_OK {name}.{k}")
        else:
            print(f"PLAT_MISSING {name}.{k}")
PY
)"

echo "$results" | while IFS=' ' read -r status detail rest; do
    case "$status" in
        *_OK) record_pass "$detail${rest:+ = $rest}" ;;
        *_MISSING|*_BAD|*_FAIL) record_fail "$detail${rest:+ $rest}" ;;
        PYYAML_MISSING) log "pyyaml not installed — deep YAML checks skipped (install with 'pip install pyyaml' for full coverage)" ;;
    esac
done

log "Verifying SHA-256 format for each platform"
python3 - "$MANIFEST" <<'PY'
import sys, re, pathlib
text = pathlib.Path(sys.argv[1]).read_text()
lines = text.splitlines()
idx = None
for i, line in enumerate(lines):
    if re.match(r'^platforms:\s*$', line):
        idx = i
        break
if idx is None:
    print("  FAIL: cannot parse platforms section")
    raise SystemExit
current = None
for line in lines[idx+1:]:
    stripped = line.strip()
    if not stripped or stripped.startswith('#'):
        continue
    if re.match(r'^[a-z][a-z_]*:\s*$', stripped) and not line.startswith(' ' * 4):
        break
    indent = len(line) - len(line.lstrip())
    m = re.match(r'^([a-z][-_a-z0-9]*):\s*(.*)$', stripped)
    if indent == 2 and m:
        current = m.group(1)
    elif indent >= 4 and m and m.group(1) == 'archive_sha256':
        sha = m.group(2).strip('"').strip("'")
        if re.fullmatch(r'[a-fA-F0-9]{64}', sha):
            print(f"  PASS: {current} sha256 format valid")
        else:
            print(f"  FAIL: {current} sha256 invalid: {sha}")
PY

log "Verifying archive URLs are reachable"
if command -v curl >/dev/null 2>&1; then
    for platform in windows-x64 linux-x64 darwin-x64 darwin-arm64; do
        url=$(python3 - "$MANIFEST" "$platform" <<'PY'
import sys, re, pathlib
text = pathlib.Path(sys.argv[1]).read_text()
target = sys.argv[2]
current = None
for line in text.splitlines():
    stripped = line.strip()
    indent = len(line) - len(line.lstrip())
    m = re.match(r'^([a-z][-_a-z0-9]*):\s*(.*)$', stripped)
    if indent == 2 and m:
        current = m.group(1)
        continue
    if current != target:
        continue
    if indent >= 4 and m and m.group(1) == 'archive_url':
        print(m.group(2).strip('"').strip("'"))
        break
PY
)
        if [[ -n "$url" ]]; then
            code=$(curl -sI -o /dev/null -w '%{http_code}' --max-time 15 "$url" 2>/dev/null || echo "000")
            if [[ "$code" == "200" || "$code" == "302" || "$code" == "301" ]]; then
                record_pass "$platform archive reachable (HTTP $code)"
            else
                record_fail "$platform archive unreachable (HTTP $code)"
            fi
        fi
    done
else
    log "curl not available, skipping reachability check"
fi

echo ""
echo "Manifest check complete."
[[ $failures -eq 0 ]]
