# Versioning Policy

This policy is the aulycmail project profile for the shared
`general-release-versioning` Skill. It defines how the common baseline maps to
the Apple Silicon macOS Wails application.

- Release Profile: `macos-arm64-app`.
- Architecture: Apple Silicon `arm64` only.
- Distribution: ad-hoc DMG for test releases; Developer ID, Hardened Runtime,
  notarization and Gatekeeper verification for formal releases.

## Single version source

`version.json` is authoritative:

```json
{
  "version": "0.3.92-dev",
  "build": 0
}
```

`tools/version-bump.mjs` derives and verifies:

- `wails.json` `info.productVersion`;
- `frontend/package.json` `version`;
- root entries in `frontend/package-lock.json`.

Do not edit those derived version fields manually. Use `make version-check` and
`make version-test` to detect drift and validate version behavior.

## SemVer and release classes

Stable releases use `MAJOR.MINOR.PATCH`. Supported development and prerelease
forms are `-dev`, `-alpha.N`, `-beta.N`, and `-rc.N`. Segments are integers, so
`2.0.9` advances to `2.0.10` for a patch.

| Class | Identity | Build | Commit/tag | Trust |
|---|---|---:|---|---|
| Local build/install | `-dev+<commit>[.dirty]` | `0` | none | ad-hoc app |
| Test release | `alpha.N`/`beta.N`/`rc.N` | positive | release commit + annotated tag | ad-hoc app/DMG, no notarization |
| Formal release | stable SemVer | positive | release commit + annotated tag | Developer ID, Hardened Runtime, notarization, Gatekeeper |

`make install-darwin` is always a local development installation, even when it
uses a production-mode binary. It is not a test release.

## Automatic version selection

Release preparation uses the current lifecycle first:

| Current version | Test | Formal |
|---|---|---|
| `0.3.92-dev` | `0.3.92-beta.1` | `0.3.92` |
| `0.3.92-beta.1` | `0.3.92-beta.2` | `0.3.92` |
| `0.3.92-rc.1` | `0.3.92-rc.2` | `0.3.92` |

From a stable version, `BREAKING CHANGE`/`type!:` selects major, `feat:`
selects minor, and other committed changes default to patch because this
repository contains historical non-Conventional commit subjects. Override an
ambiguous result with `RELEASE_BUMP=patch|minor|major`.

Every test or formal release increments the independent build number exactly
once. An unpublished, untagged failed preparation may reuse its version/build;
an already tagged or distributed identity never moves or repeats.

## Release commit and tag

Functional changes must be committed first. Release preparation creates
`chore: release <version>` containing only:

- `version.json`;
- `wails.json`;
- `frontend/package.json`;
- `frontend/package-lock.json`;
- `CHANGELOG.md`.

The tag is annotated, exactly equals the version, has no `v` prefix, points to
that release commit, and is never moved or overwritten.

Before tag creation, `make release-check` runs shared checks and builds a
production candidate with the same `arm64` target, Wails/Go production tags,
linker metadata, Bundle ID, minimum macOS version, and bundle packaging used by
the final artifact. The candidate first performs a clean `npm ci` from the
committed lockfile so dependency drift is rejected before the immutable tag is
created. Signing and notarization remain post-tag operations.

## Isolated tagged-source build

Final test and formal DMGs are not built in the caller's daily worktree.
`tools/release-worktree.mjs`:

1. verifies the caller is clean and the exact annotated tag points to `HEAD`;
2. creates a temporary detached Git worktree at the tag;
3. verifies tagged `version.json`, release commit, commit ID, and clean state;
4. installs frontend dependencies from the tagged lockfile when necessary;
5. builds and packages inside that worktree;
6. verifies the worktree remains clean after generation, build, and packaging;
7. removes only the owned temporary worktree.

The release provenance obtains tag/commit from that Git state and obtains
version, build, architecture, Bundle ID, minimum system version, signature,
Team ID, and Hardened Runtime from the real app. It obtains the artifact name
and SHA-256 from the real DMG. Formal notarization values come from the accepted
`notarytool` result and are independently checked with stapler and Gatekeeper.

## Runtime and artifact mapping

| Consumer | Value |
|---|---|
| About page, `--version`, IMAP client ID | exact runtime SemVer |
| `CFBundleShortVersionString` | numeric SemVer base |
| `CFBundleVersion` | `0` local; allocated release build |
| `AULYCSemanticVersion` | exact runtime SemVer |
| `AULYCCommitSHA` | exact source commit |
| Bundle identifier | `com.aulyc.aulycmail` |
| Minimum macOS version | `11.0` |
| Architecture | `arm64` |

Release files are immutable:

```text
aulycmail-<version>-build.<build>.dmg
aulycmail-<version>-build.<build>.manifest.json
```

The historical `.manifest.json` filename is retained for compatibility. It is
release provenance, not App product metadata, and cannot replace `Info.plist`.

The release provenance includes `releaseProfile: macos-arm64-app`, application,
version, build number, release channel,
tag, commit, dirty state, artifact, SHA-256, architecture, Bundle ID, Team ID,
minimum system version, signature type, Hardened Runtime, notarization state,
submission ID, and UTC build time. Test and formal manifests require
`dirty: false`.

Mail databases, attachments, settings, passwords, tokens, Keychain data, and
other user data are never release metadata or app-replacement payloads.
