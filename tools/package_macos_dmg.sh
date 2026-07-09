#!/bin/bash
# Build a drag-to-Applications macOS DMG from an existing aulycmail.app bundle.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP="$ROOT/build/bin/aulycmail.app"
DMG_PATH="$ROOT/dist/aulycmail.dmg"
VOLUME_NAME="aulycmail Installer"
SIGN_IDENTITY="${SIGN_IDENTITY:-}"
NOTARY_PROFILE="${NOTARY_PROFILE:-}"

usage() {
  cat <<'EOF'
Usage: tools/package_macos_dmg.sh [options]

Options:
  --app PATH             App bundle to package (default: build/bin/aulycmail.app)
  --output PATH          DMG output path (default: dist/aulycmail.dmg)
  --volume-name NAME     Mounted DMG volume name (default: aulycmail)
  --sign IDENTITY        Developer ID Application identity for app and DMG signing
  --notary-profile NAME  notarytool keychain profile; notarizes and staples the DMG
  -h, --help             Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --app)
      APP="$2"
      shift 2
      ;;
    --output)
      DMG_PATH="$2"
      shift 2
      ;;
    --volume-name)
      VOLUME_NAME="$2"
      shift 2
      ;;
    --sign)
      SIGN_IDENTITY="$2"
      shift 2
      ;;
    --notary-profile)
      NOTARY_PROFILE="$2"
      shift 2
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

if [[ ! -d "$APP" ]]; then
  echo "Missing app bundle: $APP" >&2
  exit 1
fi

APP_NAME="$(basename "$APP")"
DMG_PATH="$(cd "$(dirname "$DMG_PATH")" && pwd)/$(basename "$DMG_PATH")"
DIST_DIR="$(dirname "$DMG_PATH")"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/aulycmail-dmg.XXXXXX")"
STAGING_DIR="$WORK_DIR/staging"
RW_DMG="$WORK_DIR/$VOLUME_NAME-rw.dmg"
MOUNT_POINT=""

cleanup() {
  if [[ -n "$MOUNT_POINT" && -d "$MOUNT_POINT" ]]; then
    for _ in 1 2 3 4 5; do
      if hdiutil detach "$MOUNT_POINT" -quiet 2>/dev/null; then
        break
      fi
      sleep 1
    done
    if [[ -d "$MOUNT_POINT" ]]; then
      hdiutil detach "$MOUNT_POINT" -force -quiet 2>/dev/null || true
    fi
  fi
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

mkdir -p "$DIST_DIR" "$STAGING_DIR"

echo "Staging $APP_NAME..."
ditto "$APP" "$STAGING_DIR/$APP_NAME"
ln -s /Applications "$STAGING_DIR/Applications"

if [[ -n "$SIGN_IDENTITY" ]]; then
  echo "Signing app with $SIGN_IDENTITY..."
  codesign --force --deep --options runtime --timestamp --sign "$SIGN_IDENTITY" "$STAGING_DIR/$APP_NAME"
  codesign --verify --deep --strict --verbose=2 "$STAGING_DIR/$APP_NAME"
fi

echo "Creating read/write DMG..."
hdiutil create \
  -volname "$VOLUME_NAME" \
  -srcfolder "$STAGING_DIR" \
  -fs HFS+ \
  -format UDRW \
  -ov \
  "$RW_DMG" >/dev/null

echo "Mounting DMG to write Finder layout..."
ATTACH_OUTPUT="$(hdiutil attach "$RW_DMG" -readwrite -noverify -noautoopen)"
MOUNT_POINT="$(printf '%s\n' "$ATTACH_OUTPUT" | sed -n 's|^.*\(/Volumes/.*\)$|\1|p' | head -n 1)"
if [[ -z "$MOUNT_POINT" || ! -d "$MOUNT_POINT" ]]; then
  echo "Failed to locate mounted DMG volume." >&2
  printf '%s\n' "$ATTACH_OUTPUT" >&2
  exit 1
fi

osascript - "$VOLUME_NAME" "$APP_NAME" <<'OSA'
on run argv
  set diskName to item 1 of argv
  set appName to item 2 of argv
  tell application "Finder"
    tell disk diskName
      open
      delay 1
      set current view of container window to icon view
      set toolbar visible of container window to false
      set statusbar visible of container window to false
      try
        set pathbar visible of container window to false
      end try
      set bounds of container window to {100, 100, 620, 420}
      set viewOptions to the icon view options of container window
      set arrangement of viewOptions to not arranged
      set icon size of viewOptions to 96
      set position of item appName of container window to {157, 125}
      set position of item "Applications" of container window to {363, 125}
      update without registering applications
      delay 2
      close container window
    end tell
  end tell
end run
OSA

sync
hdiutil detach "$MOUNT_POINT" -quiet
MOUNT_POINT=""

echo "Compressing DMG..."
hdiutil convert "$RW_DMG" \
  -format UDZO \
  -imagekey zlib-level=9 \
  -ov \
  -o "$DMG_PATH" >/dev/null

if [[ -n "$SIGN_IDENTITY" ]]; then
  echo "Signing DMG with $SIGN_IDENTITY..."
  codesign --force --timestamp --sign "$SIGN_IDENTITY" "$DMG_PATH"
  codesign --verify --verbose=2 "$DMG_PATH"
fi

if [[ -n "$NOTARY_PROFILE" ]]; then
  echo "Submitting DMG for notarization using keychain profile $NOTARY_PROFILE..."
  xcrun notarytool submit "$DMG_PATH" --keychain-profile "$NOTARY_PROFILE" --wait
  xcrun stapler staple "$DMG_PATH"
  xcrun stapler validate "$DMG_PATH"
  spctl -a -vvv -t open --context context:primary-signature "$DMG_PATH"
fi

echo "Created $DMG_PATH"
