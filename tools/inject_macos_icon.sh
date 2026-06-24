#!/bin/bash
# Inject a macOS 26 asset-catalog app icon (Assets.car + CFBundleIconName) into
# a built .app. On macOS 26 (Tahoe) a traditional .icns renders undersized on
# the Liquid Glass plate; an asset-catalog icon fills it. Generates all icon
# sizes from build/appicon.png (a full-bleed square — the system rounds it).
#
# Usage: tools/inject_macos_icon.sh <path-to.app> [source-png]
set -euo pipefail
APP="$1"
SRC="${2:-build/appicon.png}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/AppIcon.appiconset" "$WORK/out"

python3 - "$SRC" "$WORK/AppIcon.appiconset" <<'PY'
import sys
from PIL import Image
src = Image.open(sys.argv[1]).convert('RGB')
for s in (16, 32, 64, 128, 256, 512, 1024):
    src.resize((s, s), Image.LANCZOS).save(f"{sys.argv[2]}/{s}.png")
PY

cat > "$WORK/AppIcon.appiconset/Contents.json" <<'JSON'
{ "images" : [
  {"idiom":"mac","scale":"1x","size":"16x16","filename":"16.png"},
  {"idiom":"mac","scale":"2x","size":"16x16","filename":"32.png"},
  {"idiom":"mac","scale":"1x","size":"32x32","filename":"32.png"},
  {"idiom":"mac","scale":"2x","size":"32x32","filename":"64.png"},
  {"idiom":"mac","scale":"1x","size":"128x128","filename":"128.png"},
  {"idiom":"mac","scale":"2x","size":"128x128","filename":"256.png"},
  {"idiom":"mac","scale":"1x","size":"256x256","filename":"256.png"},
  {"idiom":"mac","scale":"2x","size":"256x256","filename":"512.png"},
  {"idiom":"mac","scale":"1x","size":"512x512","filename":"512.png"},
  {"idiom":"mac","scale":"2x","size":"512x512","filename":"1024.png"}
], "info" : {"author":"xcode","version":1} }
JSON
echo '{"info":{"author":"xcode","version":1}}' > "$WORK/Contents.json"

xcrun actool --compile "$WORK/out" --app-icon AppIcon \
  --output-partial-info-plist "$WORK/partial.plist" \
  --platform macosx --minimum-deployment-target 11.0 \
  --errors --warnings "$WORK" >/dev/null

cp "$WORK/out/Assets.car" "$APP/Contents/Resources/Assets.car"
cp "$WORK/out/AppIcon.icns" "$APP/Contents/Resources/AppIcon.icns"
/usr/libexec/PlistBuddy -c "Add :CFBundleIconName string AppIcon" "$APP/Contents/Info.plist" 2>/dev/null \
  || /usr/libexec/PlistBuddy -c "Set :CFBundleIconName AppIcon" "$APP/Contents/Info.plist"
echo "Injected macOS asset-catalog icon (Assets.car + CFBundleIconName) into $APP"
