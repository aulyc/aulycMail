# BUILD

macOS-only slim build. (Linux/Windows/Flatpak targets have been removed from this fork.)

### 🔨 Building from Source (macOS)
---

**Prerequisites:**

```bash
# Go (1.25+) and Wails CLI
brew install go
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0

# Node.js + npm (for the frontend) and Xcode Command Line Tools
brew install node
xcode-select --install   # if not already installed

# Verify everything is ready
wails doctor
```

**Build & run:**

```bash
# Build (produces build/bin/aulycmail.app, ad-hoc signed)
make build

# Run
open build/bin/aulycmail.app

# Or install to /Applications
make install-darwin
```

> Mail accounts use IMAP/SMTP passwords. Providers such as Gmail and iCloud may
> require an app-specific password.
