#!/bin/bash
# Package the already-built aulycmail binary into a macOS .app bundle.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUNDLE_VERSION="${BUNDLE_VERSION:-0.3.91}"
APP="$ROOT/build/bin/aulycmail.app"
BIN="$ROOT/build/bin/aulycmail"
CONTENTS="$APP/Contents"
MACOS="$CONTENTS/MacOS"
RESOURCES="$CONTENTS/Resources"
LEGAL="$RESOURCES/Legal"

if [ ! -x "$BIN" ]; then
  echo "Missing built binary: $BIN" >&2
  exit 1
fi

rm -rf "$APP"
mkdir -p "$MACOS" "$RESOURCES" "$LEGAL"
cp "$BIN" "$MACOS/aulycmail"
chmod 0755 "$MACOS/aulycmail"
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
  <string>aulycmail</string>
  <key>CFBundleExecutable</key>
  <string>aulycmail</string>
  <key>CFBundleIdentifier</key>
  <string>com.aulyc.aulycmail</string>
  <key>CFBundleVersion</key>
  <string>${BUNDLE_VERSION}</string>
  <key>CFBundleGetInfoString</key>
  <string>A lightweight desktop e-mail client</string>
  <key>CFBundleShortVersionString</key>
  <string>${BUNDLE_VERSION}</string>
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
