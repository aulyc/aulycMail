# Versioning Policy

This policy applies to the macOS aulycmail application and its Wails frontend.

## 1. Single version source

`version.json` is the only version source:

```json
{
  "version": "0.3.92-dev",
  "build": 0
}
```

The version is canonical SemVer. The build is the last allocated release build
number. `tools/version-bump.mjs` synchronizes the derived values in
`wails.json`, `frontend/package.json`, and `frontend/package-lock.json`; do not
edit those derived fields by hand.

## 2. SemVer

Stable versions use `MAJOR.MINOR.PATCH`:

- `MAJOR`: incompatible behavior or a major architecture change;
- `MINOR`: a backward-compatible feature;
- `PATCH`: a bug, security, or performance fix.

Segments may exceed 9: `2.0.9` advances to `2.0.10`, not `2.1.0`.

Supported lifecycle forms are:

```text
MAJOR.MINOR.PATCH-dev
MAJOR.MINOR.PATCH-alpha.N
MAJOR.MINOR.PATCH-beta.N
MAJOR.MINOR.PATCH-rc.N
MAJOR.MINOR.PATCH
```

Published versions are never overwritten. Every new test package advances its
prerelease sequence or starts a new bumped base version.

## 3. Three build classes

### Local development installation

`make install-darwin` is only a developer convenience. It may represent a
dirty worktree, uses bundle build `0`, adds the commit and optional `.dirty`
marker to the runtime version, creates no release commit or tag, and does not
count as a release.

### Test release

`make release-test` creates and installs a test release:

- version: `MAJOR.MINOR.PATCH-beta.N` (or the next `rc.N` when already in RC);
- positive, globally increasing build number;
- dedicated `chore: release <version>` commit;
- immutable annotated no-`v` tag;
- ad-hoc signatures on both app and DMG;
- no Developer ID, Apple upload, notarization, or Gatekeeper acceptance claim.

### Formal release

`make release-formal` creates and installs a stable release:

- version: `MAJOR.MINOR.PATCH`;
- positive, globally increasing build number;
- dedicated release commit and immutable annotated tag;
- Developer ID signing, hardened runtime, Apple notarization, stapling, and
  Gatekeeper verification.

Both release classes produce a versioned DMG and manifest and must pass the same
repository quality gates.

## 4. Automatic version selection

All functional changes must already be committed before either release command
runs. Release preparation then selects the version automatically:

| Current version | Test release | Formal release |
|---|---|---|
| `0.3.92-dev` | `0.3.92-beta.1` | `0.3.92` |
| `0.3.92-beta.1` | `0.3.92-beta.2` | `0.3.92` |
| `0.3.92-rc.1` | `0.3.92-rc.2` | `0.3.92` |
| `2.0.9` + fixes | `2.0.10-beta.1` | `2.0.10` |
| `2.0.9` + feature | `2.1.0-beta.1` | `2.1.0` |
| `2.0.9` + breaking change | `3.0.0-beta.1` | `3.0.0` |

Automatic impact detection uses commits since the current stable tag:

- `BREAKING CHANGE` or `type!:` -> `MAJOR`;
- `feat:` -> `MINOR`;
- all other committed changes -> `PATCH`.

This repository contains older non-Conventional commit subjects, so unrecognized
commits intentionally default to `PATCH`. Override an ambiguous release with:

```bash
make release-test RELEASE_BUMP=minor
make release-formal RELEASE_BUMP=major \
  SIGN_IDENTITY="Developer ID Application: ..." \
  NOTARY_PROFILE=aulyc-notary
```

The override accepts `patch`, `minor`, or `major`. Once a version is already in
`dev`, `beta`, or `rc`, lifecycle promotion takes precedence over a new base
bump.

## 5. Build numbers

Build numbers are independent from SemVer and never decrease or repeat for a
published test or formal package. Release preparation increments the counter
exactly once. A failed release that has not created its tag reuses the same
unpublished version/build when retried; a published/tagged release always
advances to a new identity.

## 6. Release commits and tags

The release commands first refuse a dirty worktree. They then:

1. select the next version and build;
2. move `[Unreleased]` notes under an exact dated version heading;
3. synchronize all derived version files;
4. create `chore: release <version>` automatically;
5. run all quality gates;
6. create or verify the exact annotated tag;
7. rebuild the artifact from that tag.

Tags contain no `v`, exactly match the version, point at the release commit,
and are never moved, replaced, or reused.

## 7. Platform mapping and artifact identity

| Location | Value |
|---|---|
| About page, `--version`, IMAP client ID | exact runtime SemVer |
| `CFBundleShortVersionString` | numeric `MAJOR.MINOR.PATCH` base |
| `CFBundleVersion` | `0` for local builds; allocated build for releases |
| `AULYCSemanticVersion` | exact runtime SemVer |
| `AULYCCommitSHA` | exact source commit |
| Wails `productVersion` | numeric base |
| npm package metadata | canonical version |

Release artifacts use:

```text
aulycmail-<version>-build.<build>.dmg
aulycmail-<version>-build.<build>.manifest.json
```

The manifest records version, build, tag, commit, architecture, signature type,
Apple notarization submission ID when applicable, UTC build time, artifact
name, and SHA-256. The installer verifies the checksum, signatures, bundle
metadata, and runtime version. Gatekeeper assessment is mandatory only for the
Developer ID formal channel; the ad-hoc test channel is explicitly identified
and never reported as notarized.

User databases, settings, credentials, and mail data are never part of the app
replacement or release manifest.
