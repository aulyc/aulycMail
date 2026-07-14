#!/bin/bash
# Verify a release DMG and its contained app without installing it.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DMG=""
REPO="$ROOT"
CHANNEL=""
MOUNT_POINT=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dmg)
      DMG="$2"
      shift 2
      ;;
    --repo)
      REPO="$2"
      shift 2
      ;;
    --channel)
      CHANNEL="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 2
      ;;
  esac
done

if [[ ! -f "$DMG" ]]; then
  echo "Missing release DMG: $DMG" >&2
  exit 1
fi
if [[ "$CHANNEL" != "test" && "$CHANNEL" != "formal" ]]; then
  echo "--channel must be test or formal." >&2
  exit 1
fi
REPO="$(cd "$REPO" && pwd)"
DMG="$(cd "$(dirname "$DMG")" && pwd)/$(basename "$DMG")"
MANIFEST="${DMG%.dmg}.manifest.json"

node "$REPO/tools/release-identity.mjs" verify-manifest \
  --manifest "$MANIFEST" --repo "$REPO" --dmg "$DMG"

EXPECTED_CHANNEL="$(/usr/bin/plutil -extract releaseChannel raw -o - "$MANIFEST")"
EXPECTED_SIGNATURE="$(/usr/bin/plutil -extract signatureType raw -o - "$MANIFEST")"
EXPECTED_TEAM="$(/usr/bin/plutil -extract teamIdentifier raw -o - "$MANIFEST" 2>/dev/null || true)"
if [[ "$EXPECTED_CHANNEL" != "$CHANNEL" ]]; then
  echo "Requested verification channel does not match the manifest." >&2
  exit 1
fi

codesign --verify --verbose=2 "$DMG"
DMG_CODESIGN_INFO="$(codesign -dv --verbose=4 "$DMG" 2>&1)"
DMG_TEAM="$(printf '%s\n' "$DMG_CODESIGN_INFO" | sed -n 's/^TeamIdentifier=//p' | head -n 1)"
[[ "$DMG_TEAM" == "not set" ]] && DMG_TEAM=""
if [[ "$CHANNEL" == "test" ]]; then
  if [[ "$EXPECTED_SIGNATURE" != "adhoc" ]] || ! printf '%s\n' "$DMG_CODESIGN_INFO" | grep -q '^Signature=adhoc$'; then
    echo "Test release DMG is not ad-hoc signed." >&2
    exit 1
  fi
  if [[ -n "$DMG_TEAM" ]]; then
    echo "Test release DMG must not claim a Team ID." >&2
    exit 1
  fi
else
  if [[ "$EXPECTED_SIGNATURE" != "developer-id" || -z "$EXPECTED_TEAM" || "$DMG_TEAM" != "$EXPECTED_TEAM" ]]; then
    echo "Formal release DMG Developer ID or Team ID does not match the manifest." >&2
    exit 1
  fi
  if ! printf '%s\n' "$DMG_CODESIGN_INFO" | grep -q '^Authority=Developer ID Application:'; then
    echo "Formal release DMG is not signed by a Developer ID Application identity." >&2
    exit 1
  fi
  xcrun stapler validate "$DMG"
  spctl -a -vvv -t open --context context:primary-signature "$DMG"
fi

cleanup() {
  if [[ -n "$MOUNT_POINT" && -d "$MOUNT_POINT" ]]; then
    hdiutil detach "$MOUNT_POINT" -quiet 2>/dev/null || hdiutil detach "$MOUNT_POINT" -force -quiet 2>/dev/null || true
  fi
}
trap cleanup EXIT

ATTACH_OUTPUT="$(hdiutil attach "$DMG" -readonly -noautoopen -noverify)"
MOUNT_POINT="$(printf '%s\n' "$ATTACH_OUTPUT" | sed -n 's|^.*\(/Volumes/.*\)$|\1|p' | head -n 1)"
if [[ -z "$MOUNT_POINT" || ! -d "$MOUNT_POINT" ]]; then
  echo "Failed to locate mounted DMG volume." >&2
  exit 1
fi
SOURCE_APP="$MOUNT_POINT/aulycmail.app"
if [[ ! -d "$SOURCE_APP" ]]; then
  echo "Release DMG does not contain aulycmail.app." >&2
  exit 1
fi

bash "$REPO/tools/verify_macos_app.sh" \
  --app "$SOURCE_APP" --manifest "$MANIFEST" --channel "$CHANNEL"

echo "Verified release artifact $(basename "$DMG") and contained app."
