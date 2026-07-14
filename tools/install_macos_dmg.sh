#!/bin/bash
# Install aulycmail from a packaged DMG into /Applications.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DMG_PATH="$ROOT/dist/aulycmail.dmg"
APP_NAME="aulycmail.app"
DEST_DIR="/Applications"
LAUNCH=1
ALLOW_ADHOC=0
MOUNT_POINT=""

usage() {
  cat <<'EOF'
Usage: tools/install_macos_dmg.sh [options]

Options:
  --dmg PATH       DMG to install from (default: dist/aulycmail.dmg)
  --app-name NAME  App bundle name inside the DMG (default: aulycmail.app)
  --allow-adhoc    Allow an explicitly ad-hoc signed internal test release
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

MANIFEST_PATH="${DMG_PATH%.dmg}.manifest.json"
if [[ ! -f "$MANIFEST_PATH" ]]; then
  echo "Missing release manifest: $MANIFEST_PATH" >&2
  exit 1
fi

EXPECTED_VERSION="$(/usr/bin/plutil -extract version raw -o - "$MANIFEST_PATH")"
EXPECTED_BUILD="$(/usr/bin/plutil -extract buildNumber raw -o - "$MANIFEST_PATH")"
EXPECTED_COMMIT="$(/usr/bin/plutil -extract commit raw -o - "$MANIFEST_PATH")"
SIGNATURE_TYPE="$(/usr/bin/plutil -extract signatureType raw -o - "$MANIFEST_PATH")"
EXPECTED_SHA256="$(/usr/bin/plutil -extract sha256 raw -o - "$MANIFEST_PATH")"
EXPECTED_ARTIFACT="$(/usr/bin/plutil -extract artifact raw -o - "$MANIFEST_PATH")"
ACTUAL_SHA256="$(shasum -a 256 "$DMG_PATH" | awk '{print $1}')"
if [[ "$EXPECTED_ARTIFACT" != "$(basename "$DMG_PATH")" ]]; then
  echo "Manifest artifact name does not match the DMG." >&2
  exit 1
fi
if [[ "$EXPECTED_SHA256" != "$ACTUAL_SHA256" ]]; then
  echo "DMG checksum does not match the release manifest." >&2
  exit 1
fi
if [[ "$SIGNATURE_TYPE" != "developer-id" && "$SIGNATURE_TYPE" != "adhoc" ]]; then
  echo "Unsupported release signature type: $SIGNATURE_TYPE" >&2
  exit 1
fi
if [[ "$SIGNATURE_TYPE" == "adhoc" && "$ALLOW_ADHOC" != "1" ]]; then
  echo "Refusing ad-hoc test release without explicit --allow-adhoc." >&2
  exit 1
fi

verify_app_metadata() {
  local app_path="$1"
  local info_plist="$app_path/Contents/Info.plist"
  local executable_name
  local actual_version
  local actual_short
  local actual_build
  local actual_commit
  local runtime_version
  local expected_short="${EXPECTED_VERSION%%[-+]*}"
  local expected_runtime="$EXPECTED_VERSION"

  executable_name="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$info_plist")"
  actual_version="$(/usr/bin/plutil -extract AULYCSemanticVersion raw -o - "$info_plist")"
  actual_short="$(/usr/bin/plutil -extract CFBundleShortVersionString raw -o - "$info_plist")"
  actual_build="$(/usr/bin/plutil -extract CFBundleVersion raw -o - "$info_plist")"
  actual_commit="$(/usr/bin/plutil -extract AULYCCommitSHA raw -o - "$info_plist")"
  runtime_version="$("$app_path/Contents/MacOS/$executable_name" --version)"
  if [[ "$EXPECTED_BUILD" != "0" ]]; then
    expected_runtime="$EXPECTED_VERSION (build $EXPECTED_BUILD)"
  fi

  if [[ "$actual_version" != "$EXPECTED_VERSION" || "$actual_short" != "$expected_short" || \
        "$actual_build" != "$EXPECTED_BUILD" || "$actual_commit" != "$EXPECTED_COMMIT" || \
        "$runtime_version" != "$expected_runtime" ]]; then
    echo "Installed app metadata does not match the release manifest." >&2
    echo "Expected: version=$EXPECTED_VERSION build=$EXPECTED_BUILD commit=$EXPECTED_COMMIT" >&2
    echo "Actual:   version=$actual_version build=$actual_build commit=$actual_commit runtime=$runtime_version" >&2
    exit 1
  fi
}

cleanup() {
  if [[ -n "$MOUNT_POINT" && -d "$MOUNT_POINT" ]]; then
    hdiutil detach "$MOUNT_POINT" -quiet 2>/dev/null || hdiutil detach "$MOUNT_POINT" -force -quiet 2>/dev/null || true
  fi
}
trap cleanup EXIT

if [[ "$SIGNATURE_TYPE" == "developer-id" ]]; then
  echo "Verifying DMG Gatekeeper status..."
  spctl -a -vvv -t open --context context:primary-signature "$DMG_PATH"
else
  echo "Verifying ad-hoc DMG signature for test release..."
  codesign --verify --verbose=2 "$DMG_PATH"
fi

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
verify_app_metadata "$SOURCE_APP"

echo "Installing $APP_NAME to $DEST_DIR..."
if [[ -d "$DEST_APP" ]]; then
  rm -rf "$DEST_APP"
fi
ditto "$SOURCE_APP" "$DEST_APP"

echo "Verifying installed app signature..."
codesign --verify --deep --strict --verbose=2 "$DEST_APP"
if [[ "$SIGNATURE_TYPE" == "developer-id" ]]; then
  spctl -a -vvv -t exec "$DEST_APP"
else
  echo "Skipping Gatekeeper assessment for explicitly ad-hoc signed test release."
fi
verify_app_metadata "$DEST_APP"
echo "Verified installed version $EXPECTED_VERSION (build $EXPECTED_BUILD, commit $EXPECTED_COMMIT)."

if [[ "$LAUNCH" -eq 1 ]]; then
  echo "Launching installed app..."
  open "$DEST_APP"
fi

echo "Installed $DEST_APP"
