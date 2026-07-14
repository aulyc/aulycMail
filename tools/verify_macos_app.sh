#!/bin/bash
# Cross-check an app bundle against a validated release manifest.
set -euo pipefail

APP=""
MANIFEST=""
CHANNEL=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --app)
      APP="$2"
      shift 2
      ;;
    --manifest)
      MANIFEST="$2"
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

if [[ ! -d "$APP" || ! -f "$MANIFEST" ]]; then
  echo "--app and --manifest must identify existing paths." >&2
  exit 1
fi
if [[ "$CHANNEL" != "test" && "$CHANNEL" != "formal" ]]; then
  echo "--channel must be test or formal." >&2
  exit 1
fi

EXPECTED_VERSION="$(/usr/bin/plutil -extract version raw -o - "$MANIFEST")"
EXPECTED_BUILD="$(/usr/bin/plutil -extract buildNumber raw -o - "$MANIFEST")"
EXPECTED_CHANNEL="$(/usr/bin/plutil -extract releaseChannel raw -o - "$MANIFEST")"
EXPECTED_COMMIT="$(/usr/bin/plutil -extract commit raw -o - "$MANIFEST")"
EXPECTED_DIRTY="$(/usr/bin/plutil -extract dirty raw -o - "$MANIFEST")"
EXPECTED_ARCH="$(/usr/bin/plutil -extract architecture raw -o - "$MANIFEST")"
EXPECTED_BUNDLE_ID="$(/usr/bin/plutil -extract bundleIdentifier raw -o - "$MANIFEST")"
EXPECTED_MIN_SYSTEM="$(/usr/bin/plutil -extract minimumSystemVersion raw -o - "$MANIFEST")"
EXPECTED_SIGNATURE="$(/usr/bin/plutil -extract signatureType raw -o - "$MANIFEST")"
EXPECTED_HARDENED="$(/usr/bin/plutil -extract hardenedRuntime raw -o - "$MANIFEST")"
EXPECTED_TEAM="$(/usr/bin/plutil -extract teamIdentifier raw -o - "$MANIFEST" 2>/dev/null || true)"

if [[ "$EXPECTED_CHANNEL" != "$CHANNEL" || "$EXPECTED_DIRTY" != "false" ]]; then
  echo "Manifest channel or dirty state is not valid for this release verification." >&2
  exit 1
fi

INFO="$APP/Contents/Info.plist"
EXECUTABLE_NAME="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$INFO")"
EXECUTABLE="$APP/Contents/MacOS/$EXECUTABLE_NAME"
ACTUAL_VERSION="$(/usr/bin/plutil -extract AULYCSemanticVersion raw -o - "$INFO")"
ACTUAL_SHORT="$(/usr/bin/plutil -extract CFBundleShortVersionString raw -o - "$INFO")"
ACTUAL_BUILD="$(/usr/bin/plutil -extract CFBundleVersion raw -o - "$INFO")"
ACTUAL_COMMIT="$(/usr/bin/plutil -extract AULYCCommitSHA raw -o - "$INFO")"
ACTUAL_BUNDLE_ID="$(/usr/bin/plutil -extract CFBundleIdentifier raw -o - "$INFO")"
ACTUAL_MIN_SYSTEM="$(/usr/bin/plutil -extract LSMinimumSystemVersion raw -o - "$INFO")"
ACTUAL_ARCH="$(lipo -archs "$EXECUTABLE")"
FILE_DESCRIPTION="$(file "$EXECUTABLE")"
RUNTIME_VERSION="$($EXECUTABLE --version)"
EXPECTED_SHORT="${EXPECTED_VERSION%%[-+]*}"

if [[ "$ACTUAL_VERSION" != "$EXPECTED_VERSION" || "$ACTUAL_SHORT" != "$EXPECTED_SHORT" || \
      "$ACTUAL_BUILD" != "$EXPECTED_BUILD" || "$ACTUAL_COMMIT" != "$EXPECTED_COMMIT" || \
      "$ACTUAL_BUNDLE_ID" != "$EXPECTED_BUNDLE_ID" || "$ACTUAL_MIN_SYSTEM" != "$EXPECTED_MIN_SYSTEM" ]]; then
  echo "App Info.plist does not match the release manifest." >&2
  exit 1
fi
if [[ "$RUNTIME_VERSION" != "$EXPECTED_VERSION (build $EXPECTED_BUILD)" ]]; then
  echo "App --version output does not match the release manifest." >&2
  exit 1
fi
if [[ "$ACTUAL_ARCH" != "$EXPECTED_ARCH" || "$ACTUAL_ARCH" != "arm64" || "$FILE_DESCRIPTION" != *"arm64"* ]]; then
  echo "App architecture does not match the arm64 release manifest." >&2
  exit 1
fi

codesign --verify --deep --strict --verbose=2 "$APP"
CODESIGN_INFO="$(codesign -dv --verbose=4 "$APP" 2>&1)"
ACTUAL_TEAM="$(printf '%s\n' "$CODESIGN_INFO" | sed -n 's/^TeamIdentifier=//p' | head -n 1)"
[[ "$ACTUAL_TEAM" == "not set" ]] && ACTUAL_TEAM=""
ACTUAL_SIGNATURE="developer-id"
if printf '%s\n' "$CODESIGN_INFO" | grep -q '^Signature=adhoc$'; then
  ACTUAL_SIGNATURE="adhoc"
elif ! printf '%s\n' "$CODESIGN_INFO" | grep -q '^Authority=Developer ID Application:'; then
  ACTUAL_SIGNATURE="unknown"
fi
ACTUAL_HARDENED="false"
if printf '%s\n' "$CODESIGN_INFO" | grep -q '^CodeDirectory .*flags=.*runtime'; then
  ACTUAL_HARDENED="true"
fi

if [[ "$ACTUAL_SIGNATURE" != "$EXPECTED_SIGNATURE" || "$ACTUAL_TEAM" != "$EXPECTED_TEAM" || \
      "$ACTUAL_HARDENED" != "$EXPECTED_HARDENED" ]]; then
  echo "App signing identity, Team ID, or Hardened Runtime state does not match the manifest." >&2
  exit 1
fi

if [[ "$CHANNEL" == "test" ]]; then
  if [[ "$ACTUAL_SIGNATURE" != "adhoc" || -n "$ACTUAL_TEAM" || "$ACTUAL_HARDENED" != "false" ]]; then
    echo "Test release app is not a plain ad-hoc identity." >&2
    exit 1
  fi
else
  if [[ "$ACTUAL_SIGNATURE" != "developer-id" || -z "$ACTUAL_TEAM" || "$ACTUAL_HARDENED" != "true" ]]; then
    echo "Formal release app is missing Developer ID, Team ID, or Hardened Runtime." >&2
    exit 1
  fi
  spctl -a -vvv -t exec "$APP"
fi

echo "Verified app identity $EXPECTED_VERSION (build $EXPECTED_BUILD, $EXPECTED_SIGNATURE, arm64)."
