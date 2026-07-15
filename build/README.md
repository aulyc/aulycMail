# Build Directory

The build directory contains packaging assets and generated build output for
aulycMail.

## Structure

- `bin/` - generated desktop application output
- `darwin/` - macOS packaging metadata used by Wails

## macOS

The `darwin` directory contains the plist files used by the macOS build:

- `Info.plist` - production bundle metadata
- `Info.dev.plist` - development bundle metadata

Run the macOS build from the repository root:

```bash
make build
```
