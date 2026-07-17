#!/bin/bash
# Package an existing aulycMail.app and derive release identity from Git and the real app.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SOURCE_ROOT="$ROOT"
APP="$ROOT/.cache/build/aulycMail.app"
DMG_PATH="$ROOT/dist/aulycMail.dmg"
VOLUME_NAME="aulycMail Installer"
RELEASE_CHANNEL="local"
TAG=""
SIGN_IDENTITY="${SIGN_IDENTITY:-}"
NOTARY_PROFILE="${NOTARY_PROFILE:-}"
ADHOC_SIGN=0

usage() {
  cat <<'EOF'
Usage: tools/package_macos_dmg.sh [options]

Options:
  --source-root PATH     Git source used to build the app
  --app PATH             App bundle to package (default: .cache/build/aulycMail.app)
  --output PATH          DMG output path (default: dist/aulycMail.dmg)
  --volume-name NAME     Mounted DMG volume name
  --release-channel NAME local, test, or formal
  --tag TAG              Exact annotated tag for test/formal releases
  --adhoc-sign           Ad-hoc sign the app and DMG for a test release
  --sign IDENTITY        Developer ID Application identity
  --notary-profile NAME  notarytool Keychain profile
  -h, --help             Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source-root)
      SOURCE_ROOT="$2"
      shift 2
      ;;
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
    --release-channel)
      RELEASE_CHANNEL="$2"
      shift 2
      ;;
    --tag)
      TAG="$2"
      shift 2
      ;;
    --adhoc-sign)
      ADHOC_SIGN=1
      shift
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
if [[ ! -d "$SOURCE_ROOT" ]]; then
  echo "Missing source root: $SOURCE_ROOT" >&2
  exit 1
fi
SOURCE_ROOT="$(cd "$SOURCE_ROOT" && pwd)"
if [[ "$RELEASE_CHANNEL" != "local" && "$RELEASE_CHANNEL" != "test" && "$RELEASE_CHANNEL" != "formal" ]]; then
  echo "--release-channel must be local, test, or formal." >&2
  exit 1
fi
if [[ "$RELEASE_CHANNEL" == "test" ]]; then
  if [[ -z "$TAG" || "$ADHOC_SIGN" != "1" || -n "$SIGN_IDENTITY" || -n "$NOTARY_PROFILE" ]]; then
    echo "Test releases require an exact tag and --adhoc-sign, without Developer ID or notarization." >&2
    exit 1
  fi
elif [[ "$RELEASE_CHANNEL" == "formal" ]]; then
  if [[ -z "$TAG" || -z "$SIGN_IDENTITY" || -z "$NOTARY_PROFILE" || "$ADHOC_SIGN" == "1" ]]; then
    echo "Formal releases require an exact tag, --sign, and --notary-profile." >&2
    exit 1
  fi
elif [[ -n "$TAG" || "$ADHOC_SIGN" == "1" || -n "$SIGN_IDENTITY" || -n "$NOTARY_PROFILE" ]]; then
  echo "Local DMGs must not claim a release tag, release signature, or notarization." >&2
  exit 1
fi

if [[ "$RELEASE_CHANNEL" != "local" ]]; then
  node "$SOURCE_ROOT/tools/release-identity.mjs" verify-source --root "$SOURCE_ROOT" --tag "$TAG"
fi

APP_INFO="$APP/Contents/Info.plist"
APP_NAME="$(basename "$APP")"
APP_EXECUTABLE_NAME="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$APP_INFO")"
APP_EXECUTABLE="$APP/Contents/MacOS/$APP_EXECUTABLE_NAME"
APP_BUNDLE_NAME="$(/usr/bin/plutil -extract CFBundleName raw -o - "$APP_INFO")"
APP_DISPLAY_NAME="$(/usr/bin/plutil -extract CFBundleDisplayName raw -o - "$APP_INFO")"
SEMANTIC_VERSION="$(/usr/bin/plutil -extract AULYCSemanticVersion raw -o - "$APP_INFO")"
BUILD_NUMBER="$(/usr/bin/plutil -extract CFBundleVersion raw -o - "$APP_INFO")"
COMMIT_SHA="$(/usr/bin/plutil -extract AULYCCommitSHA raw -o - "$APP_INFO")"
BUNDLE_IDENTIFIER="$(/usr/bin/plutil -extract CFBundleIdentifier raw -o - "$APP_INFO")"
MINIMUM_SYSTEM_VERSION="$(/usr/bin/plutil -extract LSMinimumSystemVersion raw -o - "$APP_INFO")"
ARCHITECTURE="$(lipo -archs "$APP_EXECUTABLE")"
DIRTY=false
if [[ -n "$(git -C "$SOURCE_ROOT" status --porcelain --untracked-files=all)" ]]; then
  DIRTY=true
fi

if [[ ! "$BUILD_NUMBER" =~ ^[0-9]+$ || ! "$COMMIT_SHA" =~ ^[0-9a-f]{7,64}$ ]]; then
  echo "App bundle contains an invalid build number or commit." >&2
  exit 1
fi
if [[ "$ARCHITECTURE" != "arm64" || "$BUNDLE_IDENTIFIER" != "com.aulyc.aulycmail" ]]; then
  echo "Only the arm64 com.aulyc.aulycmail bundle can be packaged." >&2
  exit 1
fi
if [[ "$APP_NAME" != "aulycMail.app" || "$APP_BUNDLE_NAME" != "aulycMail" || \
      "$APP_DISPLAY_NAME" != "aulycMail" || "$APP_EXECUTABLE_NAME" != "aulycMail" ]]; then
  echo "Only the aulycMail.app product identity can be packaged." >&2
  exit 1
fi

if [[ "$RELEASE_CHANNEL" != "local" ]]; then
  SOURCE_VERSION="$(node "$SOURCE_ROOT/tools/version-bump.mjs" get version)"
  SOURCE_BUILD="$(node "$SOURCE_ROOT/tools/version-bump.mjs" get build)"
  SOURCE_COMMIT="$(git -C "$SOURCE_ROOT" rev-parse HEAD)"
  if [[ "$DIRTY" != "false" || "$SEMANTIC_VERSION" != "$SOURCE_VERSION" || \
        "$BUILD_NUMBER" != "$SOURCE_BUILD" || "$COMMIT_SHA" != "$SOURCE_COMMIT" || "$TAG" != "$SOURCE_VERSION" ]]; then
    echo "App identity does not match the clean tagged release source." >&2
    exit 1
  fi
  # The requested tag has already been verified as annotated and exact. Record
  # the tag derived from the isolated source identity, not the caller's input.
  TAG="$SOURCE_VERSION"
fi

mkdir -p "$(dirname "$DMG_PATH")"
DMG_PATH="$(cd "$(dirname "$DMG_PATH")" && pwd)/$(basename "$DMG_PATH")"
MANIFEST_PATH="${DMG_PATH%.dmg}.manifest.json"
if [[ -e "$DMG_PATH" || -e "$MANIFEST_PATH" ]]; then
  echo "Refusing to overwrite existing artifact or manifest: $DMG_PATH" >&2
  exit 1
fi

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/aulycMail-dmg.XXXXXX")"
STAGING_DIR="$WORK_DIR/staging"
RW_DMG="$WORK_DIR/$VOLUME_NAME-rw.dmg"
MOUNT_POINT=""
OUTPUT_CREATED=0
NOTARY_SUBMISSION_ID=""
NOTARIZED=false

cleanup() {
  local status=$?
  set +e
  if [[ -n "$MOUNT_POINT" && -d "$MOUNT_POINT" ]]; then
    hdiutil detach "$MOUNT_POINT" -quiet 2>/dev/null || hdiutil detach "$MOUNT_POINT" -force -quiet 2>/dev/null || true
  fi
  rm -rf "$WORK_DIR"
  if [[ "$status" -ne 0 && "$OUTPUT_CREATED" == "1" ]]; then
    rm -f "$DMG_PATH" "$MANIFEST_PATH"
  fi
  return "$status"
}
trap cleanup EXIT

mkdir -p "$STAGING_DIR"
echo "Staging $APP_NAME..."
ditto "$APP" "$STAGING_DIR/$APP_NAME"
ln -s /Applications "$STAGING_DIR/Applications"
STAGED_APP="$STAGING_DIR/$APP_NAME"

if [[ "$RELEASE_CHANNEL" == "formal" ]]; then
  echo "Signing app with $SIGN_IDENTITY..."
  codesign --force --deep --options runtime --timestamp --sign "$SIGN_IDENTITY" "$STAGED_APP"
elif [[ "$RELEASE_CHANNEL" == "test" ]]; then
  echo "Ad-hoc signing app for test release..."
  codesign --force --deep --sign - "$STAGED_APP"
fi
codesign --verify --deep --strict --verbose=2 "$STAGED_APP"

CODESIGN_INFO="$(codesign -dv --verbose=4 "$STAGED_APP" 2>&1)"
TEAM_IDENTIFIER="$(printf '%s\n' "$CODESIGN_INFO" | sed -n 's/^TeamIdentifier=//p' | head -n 1)"
[[ "$TEAM_IDENTIFIER" == "not set" ]] && TEAM_IDENTIFIER=""
SIGNATURE_TYPE="unknown"
if printf '%s\n' "$CODESIGN_INFO" | grep -q '^Signature=adhoc$'; then
  SIGNATURE_TYPE="adhoc"
elif printf '%s\n' "$CODESIGN_INFO" | grep -q '^Authority=Developer ID Application:'; then
  SIGNATURE_TYPE="developer-id"
fi
HARDENED_RUNTIME=false
if printf '%s\n' "$CODESIGN_INFO" | grep -q '^CodeDirectory .*flags=.*runtime'; then
  HARDENED_RUNTIME=true
fi

if [[ "$RELEASE_CHANNEL" == "test" && ( "$SIGNATURE_TYPE" != "adhoc" || -n "$TEAM_IDENTIFIER" || "$HARDENED_RUNTIME" != "false" ) ]]; then
  echo "Test app signing identity is not plain ad-hoc." >&2
  exit 1
fi
if [[ "$RELEASE_CHANNEL" == "formal" && ( "$SIGNATURE_TYPE" != "developer-id" || -z "$TEAM_IDENTIFIER" || "$HARDENED_RUNTIME" != "true" ) ]]; then
  echo "Formal app is missing Developer ID, Team ID, or Hardened Runtime." >&2
  exit 1
fi
if [[ "$RELEASE_CHANNEL" == "local" ]]; then
  SIGNATURE_TYPE="unsigned"
  TEAM_IDENTIFIER=""
  HARDENED_RUNTIME=false
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
echo "Removing transient macOS filesystem metadata..."
rm -rf -- "$MOUNT_POINT/.fseventsd"
if [[ -e "$MOUNT_POINT/.fseventsd" ]]; then
  echo "Failed to remove .fseventsd from the writable DMG." >&2
  exit 1
fi
hdiutil detach "$MOUNT_POINT" -quiet
MOUNT_POINT=""

echo "Compressing DMG..."
hdiutil convert "$RW_DMG" -format UDZO -imagekey zlib-level=9 -o "$DMG_PATH" >/dev/null
OUTPUT_CREATED=1

echo "Verifying final DMG filesystem contents..."
ATTACH_OUTPUT="$(hdiutil attach "$DMG_PATH" -readonly -noverify -noautoopen)"
MOUNT_POINT="$(printf '%s\n' "$ATTACH_OUTPUT" | sed -n 's|^.*\(/Volumes/.*\)$|\1|p' | head -n 1)"
if [[ -z "$MOUNT_POINT" || ! -d "$MOUNT_POINT" ]]; then
  echo "Failed to locate the final mounted DMG volume." >&2
  exit 1
fi
if [[ -e "$MOUNT_POINT/.fseventsd" ]]; then
  echo "Final DMG unexpectedly contains .fseventsd." >&2
  exit 1
fi
hdiutil detach "$MOUNT_POINT" -quiet
MOUNT_POINT=""

if [[ "$RELEASE_CHANNEL" == "formal" ]]; then
  echo "Signing DMG with $SIGN_IDENTITY..."
  codesign --force --timestamp --sign "$SIGN_IDENTITY" "$DMG_PATH"
elif [[ "$RELEASE_CHANNEL" == "test" ]]; then
  echo "Ad-hoc signing DMG for test release..."
  codesign --force --sign - "$DMG_PATH"
fi
if [[ "$RELEASE_CHANNEL" != "local" ]]; then
  codesign --verify --verbose=2 "$DMG_PATH"
fi

if [[ "$RELEASE_CHANNEL" == "formal" ]]; then
  echo "Submitting DMG for notarization using Keychain profile $NOTARY_PROFILE..."
  NOTARY_RESULT="$WORK_DIR/notary-result.json"
  xcrun notarytool submit "$DMG_PATH" \
    --keychain-profile "$NOTARY_PROFILE" \
    --wait \
    --output-format json >"$NOTARY_RESULT"
  NOTARY_STATUS="$(/usr/bin/plutil -extract status raw -o - "$NOTARY_RESULT")"
  NOTARY_SUBMISSION_ID="$(/usr/bin/plutil -extract id raw -o - "$NOTARY_RESULT")"
  if [[ "$NOTARY_STATUS" != "Accepted" ]]; then
    echo "Notarization failed with status: $NOTARY_STATUS" >&2
    exit 1
  fi
  NOTARIZED=true
  echo "Notarization accepted: $NOTARY_SUBMISSION_ID"
  xcrun stapler staple "$DMG_PATH"
  xcrun stapler validate "$DMG_PATH"
  spctl -a -vvv -t open --context context:primary-signature "$DMG_PATH"
fi

if [[ "$RELEASE_CHANNEL" != "local" ]]; then
  node "$SOURCE_ROOT/tools/release-identity.mjs" verify-source --root "$SOURCE_ROOT" --tag "$TAG"
fi

DMG_SHA256="$(shasum -a 256 "$DMG_PATH" | awk '{print $1}')"
BUILT_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
node --input-type=module - "$MANIFEST_PATH" "$SEMANTIC_VERSION" "$BUILD_NUMBER" "$RELEASE_CHANNEL" \
  "$TAG" "$COMMIT_SHA" "$DIRTY" "$(basename "$DMG_PATH")" "$DMG_SHA256" "$ARCHITECTURE" \
  "$BUNDLE_IDENTIFIER" "$TEAM_IDENTIFIER" "$MINIMUM_SYSTEM_VERSION" "$SIGNATURE_TYPE" \
  "$HARDENED_RUNTIME" "$NOTARIZED" "$NOTARY_SUBMISSION_ID" "$BUILT_AT" <<'NODE'
import fs from 'node:fs'

const [
  manifestPath, version, buildNumber, releaseChannel, tag, commit, dirty,
  artifact, sha256, architecture, bundleIdentifier, teamIdentifier,
  minimumSystemVersion, signatureType, hardenedRuntime, notarized,
  notarizationSubmissionId, builtAt,
] = process.argv.slice(2)

const manifest = {
  application: 'aulycMail',
  releaseProfile: 'macos-arm64-app',
  version,
  buildNumber: Number(buildNumber),
  releaseChannel,
  tag: tag || null,
  commit,
  dirty: dirty === 'true',
  artifact,
  sha256,
  architecture,
  bundleIdentifier,
  teamIdentifier: teamIdentifier || null,
  minimumSystemVersion,
  signatureType,
  hardenedRuntime: hardenedRuntime === 'true',
  notarized: notarized === 'true',
  notarizationSubmissionId: notarizationSubmissionId || null,
  builtAt,
}
fs.writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`)
NODE

node "$SOURCE_ROOT/tools/release-identity.mjs" verify-manifest \
  --manifest "$MANIFEST_PATH" --repo "$SOURCE_ROOT" --dmg "$DMG_PATH"
if [[ "$RELEASE_CHANNEL" != "local" ]]; then
  bash "$SOURCE_ROOT/tools/verify_release_artifact.sh" \
    --dmg "$DMG_PATH" --repo "$SOURCE_ROOT" --channel "$RELEASE_CHANNEL"
  node "$SOURCE_ROOT/tools/release-identity.mjs" verify-source --root "$SOURCE_ROOT" --tag "$TAG"
fi

echo "Created $DMG_PATH"
echo "Created $MANIFEST_PATH"
