#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESOURCES_DIR="$PROJECT_ROOT/build/resources/ffmpeg"
TOOLS_DIR="$RESOURCES_DIR/tools"
MANIFEST="$RESOURCES_DIR/manifest.yaml"
LICENSES_DIR="$RESOURCES_DIR/licenses"

log() { echo "[verify_ffmpeg] $*"; }
fail() { echo "[verify_ffmpeg] FAIL: $*" >&2; exit 1; }
warn() { echo "[verify_ffmpeg] WARN: $*" >&2; }

PASS=0
FAIL=0

record_pass() { PASS=$((PASS + 1)); echo "  PASS: $*"; }
record_fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $*"; }

[[ -f "$MANIFEST" ]] || fail "manifest.yaml not found"
[[ -f "$RESOURCES_DIR/CREDITS.md" ]] || warn "CREDITS.md not found"

log "Checking license files"
for f in licenses/FFmpeg-COPYING.LGPLv2.1 licenses/THIRD-PARTY-NOTICES.md; do
    [[ -f "$RESOURCES_DIR/$f" ]] && record_pass "license present: $f" || record_fail "missing license: $f"
done

require_cmd() { command -v "$1" >/dev/null 2>&1; }

if require_cmd file; then
    HAVE_FILE=1
else
    HAVE_FILE=0
    warn "'file' not found, skipping architecture checks"
fi

if require_cmd uname; then
    HOST_OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    HOST_ARCH_RAW="$(uname -m)"
    case "$HOST_ARCH_RAW" in
        x86_64|amd64) HOST_ARCH="amd64" ;;
        arm64|aarch64) HOST_ARCH="arm64" ;;
        *) HOST_ARCH="$HOST_ARCH_RAW" ;;
    esac
    log "Host: $HOST_OS/$HOST_ARCH"
else
    HOST_OS="unknown"
    HOST_ARCH="unknown"
fi

PLATFORMS="windows-x64 linux-x64 darwin-x64 darwin-arm64"

log "Checking installed tools"

for platform in $PLATFORMS; do
    dir="$TOOLS_DIR/$platform"
    echo ""
    log "--- $platform ---"

    if [[ ! -d "$dir" ]]; then
        record_fail "missing directory $dir (run make ffmpeg-fetch first)"
        continue
    fi

    bin_suffix=""
    [[ "$platform" == windows-* ]] && bin_suffix=".exe"

    ff="$dir/ffmpeg${bin_suffix}"
    fp="$dir/ffprobe${bin_suffix}"

    for tool in ffmpeg ffprobe; do
        case "$tool" in
            ffmpeg) path="$ff" ;;
            ffprobe) path="$fp" ;;
        esac

        if [[ ! -f "$path" ]]; then
            record_fail "$tool missing at $path"
            continue
        fi

        size="$(wc -c < "$path")"
        record_pass "$tool present ($size bytes)"

        if [[ "$platform" != windows-* ]]; then
            perms="$(stat -f '%Lp' "$path" 2>/dev/null || stat -c '%a' "$path" 2>/dev/null || echo "???")"
            if [[ "$perms" == "???" ]]; then
                warn "cannot read permissions for $path"
            else
                readable_perms="$((perms & 0111))"
                if [[ $readable_perms -ne 0 ]]; then
                    record_pass "$tool executable (mode $perms)"
                else
                    record_fail "$tool NOT executable (mode $perms)"
                fi
            fi
        fi

        if [[ $HAVE_FILE -eq 1 ]]; then
            file_info="$(file -b "$path")"
            echo "  file: $file_info"
            if [[ "$platform" == "$HOST_OS-$HOST_ARCH" ]]; then
                case "$HOST_ARCH" in
                    amd64)
                        if echo "$file_info" | grep -qiE 'x86_64|amd64'; then
                            record_pass "$tool architecture matches host (amd64)"
                        else
                            record_fail "$tool architecture does not match host: $file_info"
                        fi
                        ;;
                    arm64)
                        if echo "$file_info" | grep -qiE 'arm64|aarch64'; then
                            record_pass "$tool architecture matches host (arm64)"
                        else
                            record_fail "$tool architecture does not match host: $file_info"
                        fi
                        ;;
                    *)
                        warn "unknown host arch $HOST_ARCH"
                        ;;
                esac
            fi
        fi
    done

    if [[ "$platform" == "$HOST_OS-$HOST_ARCH" ]]; then
        echo ""
        log "Running ffmpeg -version for host platform"
        if "$ff" -version >/tmp/ffmpeg_ver.txt 2>&1; then
            ver_line="$(head -n1 /tmp/ffmpeg_ver.txt)"
            echo "  $ver_line"
            if echo "$ver_line" | grep -qE 'ffmpeg version n?8\.0\.'; then
                record_pass "ffmpeg version 8.0.x detected"
            else
                record_fail "unexpected ffmpeg version output: $ver_line"
            fi
        else
            record_fail "ffmpeg -version failed"
        fi
        if "$fp" -version >/tmp/ffprobe_ver.txt 2>&1; then
            ver_line="$(head -n1 /tmp/ffprobe_ver.txt)"
            echo "  $ver_line"
            if echo "$ver_line" | grep -qE 'ffprobe version n?8\.0\.'; then
                record_pass "ffprobe version 8.0.x detected"
            else
                record_fail "unexpected ffprobe version output: $ver_line"
            fi
        else
            record_fail "ffprobe -version failed"
        fi
    fi
done

echo ""
echo "============================================"
echo "Results: $PASS passed, $FAIL failed"
echo "============================================"

[[ $FAIL -eq 0 ]]
