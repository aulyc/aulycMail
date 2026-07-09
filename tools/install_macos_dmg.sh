#!/bin/bash
# Install aulycmail from a packaged DMG into /Applications.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DMG_PATH="$ROOT/dist/aulycmail.dmg"
APP_NAME="aulycmail.app"
DEST_DIR="/Applications"
LAUNCH=1
MOUNT_POINT=""

usage() {
  cat <<'EOF'
Usage: tools/install_macos_dmg.sh [options]

Options:
  --dmg PATH       DMG to install from (default: dist/aulycmail.dmg)
  --app-name NAME  App bundle name inside the DMG (default: aulycmail.app)
  --no-launch      Do not launch the installed app after copying
  -h, --help       Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dmg)
      DMG_PATH="$2"
      shift 2
      ;;
    --app-name)
      APP_NAME="$2"
      shift 2
      ;;
    --no-launch)
      LAUNCH=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ ! -f "$DMG_PATH" ]]; then
  echo "Missing DMG: $DMG_PATH" >&2
  exit 1
fi

cleanup() {
  if [[ -n "$MOUNT_POINT" && -d "$MOUNT_POINT" ]]; then
    hdiutil detach "$MOUNT_POINT" -quiet 2>/dev/null || hdiutil detach "$MOUNT_POINT" -force -quiet 2>/dev/null || true
  fi
}
trap cleanup EXIT

echo "Verifying DMG Gatekeeper status..."
spctl -a -vvv -t open --context context:primary-signature "$DMG_PATH"

echo "Mounting $DMG_PATH..."
ATTACH_OUTPUT="$(hdiutil attach "$DMG_PATH" -readonly -noautoopen -noverify)"
MOUNT_POINT="$(printf '%s\n' "$ATTACH_OUTPUT" | sed -n 's|^.*\(/Volumes/.*\)$|\1|p' | head -n 1)"
if [[ -z "$MOUNT_POINT" || ! -d "$MOUNT_POINT" ]]; then
  echo "Failed to locate mounted DMG volume." >&2
  printf '%s\n' "$ATTACH_OUTPUT" >&2
  exit 1
fi

SOURCE_APP="$MOUNT_POINT/$APP_NAME"
DEST_APP="$DEST_DIR/$APP_NAME"
if [[ ! -d "$SOURCE_APP" ]]; then
  echo "Missing app bundle in DMG: $SOURCE_APP" >&2
  exit 1
fi

echo "Verifying app signature inside DMG..."
codesign --verify --deep --strict --verbose=2 "$SOURCE_APP"

echo "Installing $APP_NAME to $DEST_DIR..."
if [[ -d "$DEST_APP" ]]; then
  rm -rf "$DEST_APP"
fi
ditto "$SOURCE_APP" "$DEST_APP"

echo "Verifying installed app signature..."
codesign --verify --deep --strict --verbose=2 "$DEST_APP"
spctl -a -vvv -t exec "$DEST_APP"

if [[ "$LAUNCH" -eq 1 ]]; then
  echo "Launching installed app..."
  open "$DEST_APP"
fi

echo "Installed $DEST_APP"
