# Release Steps

1. `app/state.go` - Go app version
2. `frontend/package.json` - npm package version
3. `frontend/package-lock.json` - auto-updates with npm install
4. `CHANGELOG.md` - add new release entry
5. `wails.json` - product version

## GitHub DMG Release

Prerequisites on the release Mac:

- `Developer ID Application` certificate installed in the login keychain.
- `notarytool` credentials stored in Keychain, e.g. `aulycmail-notary`.
- Use a repository-local Go cache in sandboxed Codex sessions.

Create a signed, notarized drag-to-Applications DMG in `dist/aulycmail.dmg`:

```bash
GOCACHE=/Users/crp/Projects/aulycmail/.cache/go-build \
  make release-dmg \
  SIGN_IDENTITY="Developer ID Application: Your Name (TEAMID)" \
  NOTARY_PROFILE=aulycmail-notary
```

The DMG packaging script:

- stages `build/bin/aulycmail.app`;
- creates an `Applications -> /Applications` symlink;
- writes a small Finder icon-view layout with equal horizontal spacing;
- compresses as `UDZO`;
- signs the app and DMG with Developer ID when `SIGN_IDENTITY` is set;
- submits to Apple notarization and staples the ticket when `NOTARY_PROFILE` is
  set;
- verifies Gatekeeper status with `spctl`.

Publish the DMG and include its checksum:

```bash
shasum -a 256 dist/aulycmail.dmg
```

## Local Signed Reinstall

When reinstalling this Mac from a signed release build, package a DMG into
`dist/` first, then install from that DMG into `/Applications`:

```bash
GOCACHE=/Users/crp/Projects/aulycmail/.cache/go-build \
  make install-release-dmg \
  SIGN_IDENTITY="Developer ID Application: Your Name (TEAMID)" \
  NOTARY_PROFILE=aulycmail-notary
```

This target runs `release-dmg`, quits a running `aulycmail`, mounts
`dist/aulycmail.dmg`, verifies the DMG and app signatures, replaces
`/Applications/aulycmail.app`, verifies the installed app, and launches it.

Before replacing a local install for the first time, make a copy of the local
data directory while the app is not running:

```bash
rsync -a "$HOME/Library/Application Support/aulycmail/" \
  "$HOME/Desktop/aulycmail-data-backup-$(date +%Y%m%d-%H%M%S)/"
```

The local mail database lives in `~/Library/Application Support/aulycmail`, not
inside `/Applications/aulycmail.app`, so replacing the app bundle should not
remove local mail data. If macOS prompts for Keychain access after moving from
an ad-hoc build to Developer ID, allow the new signed build to access the
existing `aulycmail` keychain items.
