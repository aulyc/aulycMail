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
# Set Microsoft and Google OAuth creds (optional — IMAP/SMTP works without them)
cp .env.example .env.local
# Fill in your own creds

# Build (produces build/bin/aulycmail.app, ad-hoc signed)
make build

# Run
open build/bin/aulycmail.app

# Or install to /Applications
make install-darwin
```

> OAuth credentials are optional. Without them, Gmail/Outlook one-click login is
> unavailable, but generic IMAP/SMTP accounts (including Gmail/iCloud via an
> app-specific password) work fine. See `.env.example` for how to obtain creds.
