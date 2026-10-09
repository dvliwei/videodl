#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

APP_PATH="${1:-}"
if [[ -z "$APP_PATH" ]]; then
    echo "usage: $0 <path-to.app>" >&2
    exit 1
fi

if [[ ! -d "$APP_PATH" ]]; then
    echo "codesign_bundle: $APP_PATH not found or not a directory" >&2
    exit 1
fi

if ! command -v codesign >/dev/null 2>&1; then
    echo "codesign_bundle: codesign not available (not macOS?)" >&2
    exit 1
fi

IDENTITY="${CODESIGN_IDENTITY:--}"

echo "codesign_bundle: force re-signing $APP_PATH with identity '$IDENTITY'"

codesign --force --deep --sign "$IDENTITY" "$APP_PATH"

echo "codesign_bundle: verifying signature"
codesign --verify --deep --strict --verbose=2 "$APP_PATH"

echo "codesign_bundle: OK"
