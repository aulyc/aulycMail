# Build Directory

The build directory contains tracked packaging assets for aulycMail. Generated
application bundles are kept under hidden `.cache/` paths so macOS does not
present them as duplicates of the installed application. Build and clean
commands remove the obsolete generated `build/bin/` directory from older
checkouts.

## Structure

- `.cache/build/` - local production binary and application bundle
- `.cache/wails/` - Wails development build assets and output
- `darwin/` - macOS packaging metadata used by Wails

## macOS

The `darwin` directory contains the plist files used by the macOS build:

- `Info.plist` - production bundle metadata
- `Info.dev.plist` - development bundle metadata

Run the macOS build from the repository root:

```bash
make build
```
