#!/bin/bash
# Verify the pre-tag production candidate built with release identity/configuration.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SOURCE_ROOT="$ROOT"
APP="$ROOT/.cache/build/aulycMail.app"

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
    *)
      echo "Unknown option: $1" >&2
      exit 2
      ;;
  esac
done

SOURCE_ROOT="$(cd "$SOURCE_ROOT" && pwd)"
if [[ ! -d "$APP" ]]; then
  echo "Missing release candidate app: $APP" >&2
  exit 1
fi

node "$SOURCE_ROOT/tools/release-identity.mjs" verify-preflight --root "$SOURCE_ROOT"

EXPECTED_VERSION="$(node "$SOURCE_ROOT/tools/version-bump.mjs" get version)"
EXPECTED_BUILD="$(node "$SOURCE_ROOT/tools/version-bump.mjs" get build)"
EXPECTED_COMMIT="$(git -C "$SOURCE_ROOT" rev-parse HEAD)"
INFO="$APP/Contents/Info.plist"
EXECUTABLE_NAME="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$INFO")"
EXECUTABLE="$APP/Contents/MacOS/$EXECUTABLE_NAME"
ACTUAL_NAME="$(/usr/bin/plutil -extract CFBundleName raw -o - "$INFO")"
ACTUAL_DISPLAY_NAME="$(/usr/bin/plutil -extract CFBundleDisplayName raw -o - "$INFO")"
ACTUAL_VERSION="$(/usr/bin/plutil -extract AULYCSemanticVersion raw -o - "$INFO")"
ACTUAL_BUILD="$(/usr/bin/plutil -extract CFBundleVersion raw -o - "$INFO")"
ACTUAL_COMMIT="$(/usr/bin/plutil -extract AULYCCommitSHA raw -o - "$INFO")"
ACTUAL_BUNDLE_ID="$(/usr/bin/plutil -extract CFBundleIdentifier raw -o - "$INFO")"
ACTUAL_MIN_SYSTEM="$(/usr/bin/plutil -extract LSMinimumSystemVersion raw -o - "$INFO")"
ACTUAL_ARCH="$(lipo -archs "$EXECUTABLE")"
RUNTIME_VERSION="$($EXECUTABLE --version)"
FILE_DESCRIPTION="$(file "$EXECUTABLE")"

if [[ "$ACTUAL_VERSION" != "$EXPECTED_VERSION" || "$ACTUAL_BUILD" != "$EXPECTED_BUILD" || \
      "$ACTUAL_COMMIT" != "$EXPECTED_COMMIT" ]]; then
  echo "Release candidate version/build/commit does not match the release source." >&2
  exit 1
fi
if [[ "$(basename "$APP")" != "aulycMail.app" || "$ACTUAL_NAME" != "aulycMail" || \
      "$ACTUAL_DISPLAY_NAME" != "aulycMail" || "$EXECUTABLE_NAME" != "aulycMail" ]]; then
  echo "Release candidate product name, bundle name, or executable name is incorrect." >&2
  exit 1
fi
if [[ "$ACTUAL_BUNDLE_ID" != "com.aulyc.aulycmail" || "$ACTUAL_MIN_SYSTEM" != "11.0" ]]; then
  echo "Release candidate Bundle ID or minimum system version is incorrect." >&2
  exit 1
fi
if [[ "$ACTUAL_ARCH" != "arm64" || "$FILE_DESCRIPTION" != *"arm64"* ]]; then
  echo "Release candidate is not an arm64 Apple Silicon executable." >&2
  exit 1
fi
if [[ "$RUNTIME_VERSION" != "$EXPECTED_VERSION (build $EXPECTED_BUILD)" ]]; then
  echo "Release candidate --version output does not match release metadata." >&2
  exit 1
fi
codesign --verify --deep --strict --verbose=2 "$APP"

echo "Verified aulycMail release candidate $EXPECTED_VERSION (build $EXPECTED_BUILD, arm64, com.aulyc.aulycmail)."
