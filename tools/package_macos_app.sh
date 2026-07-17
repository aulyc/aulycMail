#!/bin/bash
# Package the already-built aulycMail binary into a macOS .app bundle.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SEMANTIC_VERSION="${SEMANTIC_VERSION:?SEMANTIC_VERSION is required}"
SHORT_VERSION="${SHORT_VERSION:?SHORT_VERSION is required}"
BUILD_NUMBER="${BUILD_NUMBER:-0}"
COMMIT_SHA="${COMMIT_SHA:-unknown}"
APP="${AULYCMAIL_APP_BUNDLE:-$ROOT/.cache/build/aulycMail.app}"
BIN="${AULYCMAIL_APP_BINARY:-$ROOT/.cache/build/aulycMail}"
CONTENTS="$APP/Contents"
MACOS="$CONTENTS/MacOS"
RESOURCES="$CONTENTS/Resources"
LEGAL="$RESOURCES/Legal"

if [[ ! "$SEMANTIC_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?([+][0-9A-Za-z.-]+)?$ ]]; then
  echo "Invalid semantic version: $SEMANTIC_VERSION" >&2
  exit 1
fi
if [[ ! "$SHORT_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Invalid short version: $SHORT_VERSION" >&2
  exit 1
fi
if [[ ! "$BUILD_NUMBER" =~ ^[0-9]+$ ]]; then
  echo "Invalid build number: $BUILD_NUMBER" >&2
  exit 1
fi
if [[ "$COMMIT_SHA" != "unknown" && ! "$COMMIT_SHA" =~ ^[0-9a-f]{7,40}$ ]]; then
  echo "Invalid commit SHA: $COMMIT_SHA" >&2
  exit 1
fi

if [ ! -x "$BIN" ]; then
  echo "Missing built binary: $BIN" >&2
  exit 1
fi

rm -rf "$APP"
mkdir -p "$MACOS" "$RESOURCES" "$LEGAL"
cp "$BIN" "$MACOS/aulycMail"
chmod 0755 "$MACOS/aulycMail"
if [ -f "$ROOT/build/menubar-icon.png" ]; then
  cp "$ROOT/build/menubar-icon.png" "$RESOURCES/MenuBarIcon.png"
fi
if [ -f "$ROOT/LICENSE" ]; then
  cp "$ROOT/LICENSE" "$LEGAL/LICENSE.txt"
fi
if [ -f "$ROOT/THIRD_PARTY_NOTICES.md" ]; then
  cp "$ROOT/THIRD_PARTY_NOTICES.md" "$LEGAL/THIRD_PARTY_NOTICES.md"
fi
if [ -f "$ROOT/frontend/src/assets/fonts/OFL.txt" ]; then
  cp "$ROOT/frontend/src/assets/fonts/OFL.txt" "$LEGAL/Nunito-OFL.txt"
fi

cat > "$CONTENTS/Info.plist" <<PLIST
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleName</key>
  <string>aulycMail</string>
  <key>CFBundleDisplayName</key>
  <string>aulycMail</string>
  <key>CFBundleExecutable</key>
  <string>aulycMail</string>
  <key>CFBundleIdentifier</key>
  <string>com.aulyc.aulycmail</string>
  <key>CFBundleVersion</key>
  <string>${BUILD_NUMBER}</string>
  <key>CFBundleGetInfoString</key>
  <string>aulycMail ${SEMANTIC_VERSION} (build ${BUILD_NUMBER})</string>
  <key>CFBundleShortVersionString</key>
  <string>${SHORT_VERSION}</string>
  <key>AULYCSemanticVersion</key>
  <string>${SEMANTIC_VERSION}</string>
  <key>AULYCCommitSHA</key>
  <string>${COMMIT_SHA}</string>
  <key>CFBundleIconFile</key>
  <string>AppIcon</string>
  <key>LSMinimumSystemVersion</key>
  <string>11.0</string>
  <key>NSHighResolutionCapable</key>
  <string>true</string>
  <key>NSHumanReadableCopyright</key>
  <string>Copyright &#xA9; 2026 aulyc</string>
  <key>CFBundleURLTypes</key>
  <array>
    <dict>
      <key>CFBundleURLName</key>
      <string>com.aulyc.mailto</string>
      <key>CFBundleURLSchemes</key>
      <array>
        <string>mailto</string>
      </array>
      <key>CFBundleTypeRole</key>
      <string></string>
    </dict>
  </array>
</dict>
</plist>
PLIST

echo "Packaged macOS app bundle at $APP"
