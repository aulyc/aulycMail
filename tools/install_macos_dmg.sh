#!/bin/bash
# Install a verified test or formal DMG into /Applications.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DMG_PATH="$ROOT/dist/aulycmail.dmg"
SOURCE_REPO="$ROOT"
DEST_APP="/Applications/aulycmail.app"
LAUNCH=1
ALLOW_ADHOC=0
MOUNT_POINT=""

usage() {
  cat <<'EOF'
Usage: tools/install_macos_dmg.sh [options]

Options:
  --dmg PATH          Release DMG to install
  --source-repo PATH  Repository containing the release tag
  --allow-adhoc       Allow an explicitly ad-hoc signed test release
  --no-launch         Do not launch after installation
  -h, --help          Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dmg)
      DMG_PATH="$2"
      shift 2
      ;;
    --source-repo)
      SOURCE_REPO="$2"
      shift 2
      ;;
    --allow-adhoc)
      ALLOW_ADHOC=1
      shift
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
SOURCE_REPO="$(cd "$SOURCE_REPO" && pwd)"
DMG_PATH="$(cd "$(dirname "$DMG_PATH")" && pwd)/$(basename "$DMG_PATH")"
MANIFEST_PATH="${DMG_PATH%.dmg}.manifest.json"
if [[ ! -f "$MANIFEST_PATH" ]]; then
  echo "Missing release manifest: $MANIFEST_PATH" >&2
  exit 1
fi

CHANNEL="$(/usr/bin/plutil -extract releaseChannel raw -o - "$MANIFEST_PATH")"
if [[ "$CHANNEL" == "test" ]]; then
  if [[ "$ALLOW_ADHOC" != "1" ]]; then
    echo "Refusing ad-hoc test release without explicit --allow-adhoc." >&2
    exit 1
  fi
elif [[ "$CHANNEL" != "formal" ]]; then
  echo "Only test or formal release artifacts can be installed by this script." >&2
  exit 1
fi

bash "$SOURCE_REPO/tools/verify_release_artifact.sh" \
  --dmg "$DMG_PATH" --repo "$SOURCE_REPO" --channel "$CHANNEL"

cleanup() {
  if [[ -n "$MOUNT_POINT" && -d "$MOUNT_POINT" ]]; then
    hdiutil detach "$MOUNT_POINT" -quiet 2>/dev/null || hdiutil detach "$MOUNT_POINT" -force -quiet 2>/dev/null || true
  fi
}
trap cleanup EXIT

echo "Mounting $DMG_PATH..."
ATTACH_OUTPUT="$(hdiutil attach "$DMG_PATH" -readonly -noautoopen -noverify)"
MOUNT_POINT="$(printf '%s\n' "$ATTACH_OUTPUT" | sed -n 's|^.*\(/Volumes/.*\)$|\1|p' | head -n 1)"
if [[ -z "$MOUNT_POINT" || ! -d "$MOUNT_POINT" ]]; then
  echo "Failed to locate mounted DMG volume." >&2
  exit 1
fi
SOURCE_APP="$MOUNT_POINT/aulycmail.app"
if [[ ! -d "$SOURCE_APP" ]]; then
  echo "Missing aulycmail.app in release DMG." >&2
  exit 1
fi

echo "Installing verified aulycmail.app to /Applications..."
if [[ -d "$DEST_APP" ]]; then
  rm -rf "$DEST_APP"
fi
ditto "$SOURCE_APP" "$DEST_APP"

if [[ "$DEST_APP" != "/Applications/aulycmail.app" || ! -d "$DEST_APP" ]]; then
  echo "Installed app is not at the required /Applications/aulycmail.app path." >&2
  exit 1
fi
bash "$SOURCE_REPO/tools/verify_macos_app.sh" \
  --app "$DEST_APP" --manifest "$MANIFEST_PATH" --channel "$CHANNEL"

EXPECTED_VERSION="$(/usr/bin/plutil -extract version raw -o - "$MANIFEST_PATH")"
EXPECTED_BUILD="$(/usr/bin/plutil -extract buildNumber raw -o - "$MANIFEST_PATH")"
EXPECTED_COMMIT="$(/usr/bin/plutil -extract commit raw -o - "$MANIFEST_PATH")"
echo "Verified installed version $EXPECTED_VERSION (build $EXPECTED_BUILD, commit $EXPECTED_COMMIT)."

if [[ "$LAUNCH" -eq 1 ]]; then
  echo "Launching installed app..."
  open "$DEST_APP"
fi

echo "Installed $DEST_APP"
