# Release Process

Read [VERSIONING.md](VERSIONING.md) first. The global
`general-release-versioning` Skill supplies the shared baseline; this document
describes the executable aulycmail flow.

## Non-release quality entrypoints

```bash
make check   # version, release-tool, Go, and frontend gates
make ci      # make check plus a complete production build
```

These commands do not create a release commit/tag, sign with Developer ID,
notarize, install, or publish. A future CI provider should invoke `make ci` for
normal changes. Formal signing, notarization, installation, and publication
must not run in ordinary pull-request CI.

## Common release sequence

Both release channels perform the same identity steps:

1. Refuse uncommitted functional changes.
2. Select version/build and update only allowed release metadata.
3. Create `chore: release <version>` and require a clean worktree.
4. Run `make release-check`, including `make check` and a same-configuration
   production candidate build.
5. Create or verify the immutable annotated tag exactly matching the version.
6. Create a temporary detached worktree at that exact tag.
7. Verify tag, commit, `version.json`, release-only commit contents, and clean
   isolated source before and after generation/build/package.
8. Build the final app and DMG in the isolated worktree. The caller worktree is
   never used as the final artifact source.
9. Derive and validate the manifest against Git and the actual artifact.
10. Verify the DMG and contained app, install to
    `/Applications/aulycmail.app`, verify the installed app, then launch.

Existing same-version DMGs or manifests are not overwritten. To reuse an
existing artifact, verify and install that unchanged artifact. If content or
identity differs, prepare a new prerelease or patch version/build.

## Test release

```bash
GOCACHE=/Users/crp/Projects/aulycmail/.cache/go-build \
  make release-test
```

The test channel uses `alpha.N`, `beta.N`, or `rc.N`, a positive build number,
and ad-hoc signatures on both app and DMG. It never supplies Developer ID or
notary credentials, never uploads to Apple, records `notarized: false`, and
does not run or claim formal Gatekeeper trust. Installation requires the
explicit internal `--allow-adhoc` path.

## Formal release

Prerequisites:

- the Developer ID Application certificate is available in Keychain;
- `notarytool` credentials exist under the `aulyc-notary` Keychain profile.

```bash
GOCACHE=/Users/crp/Projects/aulycmail/.cache/go-build \
  make release-formal \
  SIGN_IDENTITY="Developer ID Application: nan ma (M9M7M2ARFD)" \
  NOTARY_PROFILE=aulyc-notary
```

The isolated tagged app is Developer ID signed with Hardened Runtime. The DMG
is signed, submitted with `notarytool --wait`, and must return `Accepted`.
Stapling, `stapler validate`, DMG Gatekeeper assessment, contained-app
assessment, installed-app assessment, and manifest verification must all pass.
Credentials remain in Keychain and are not written to logs or manifests.

## Manifest and artifact verification

Every test/formal manifest contains:

```text
application, version, buildNumber, releaseChannel, tag, commit, dirty,
artifact, sha256, architecture, bundleIdentifier, teamIdentifier,
minimumSystemVersion, signatureType, hardenedRuntime, notarized,
notarizationSubmissionId, builtAt
```

Validation does not trust those values alone. The release and installation
tools cross-check:

- annotated Git tag, target commit, tagged `version.json`, and release commit;
- DMG filename and SHA-256;
- app Info.plist version/build/commit, Bundle ID, and minimum macOS version;
- app executable `--version`;
- `lipo` and `file` arm64 results;
- app and DMG code-signing type and Team ID;
- Hardened Runtime flags;
- formal stapler and Gatekeeper results;
- the final `/Applications/aulycmail.app` path and installed identity.

## Failure and retry

- Before a tag exists, fix and commit the failure; the unpublished prepared
  version/build may be reused and new Unreleased notes folded into it.
- An existing tag is accepted only when it is annotated and still points to the
  same release commit. It is never moved or replaced.
- A partial failed package created by the current run is removed safely. An
  artifact that already existed before the run is never overwritten.
- After a tag or artifact is distributed, fixes require a new prerelease or
  stable patch identity and a new build number.
- Temporary worktree cleanup is restricted to the path created by the release
  tool and must not modify the caller worktree.

## Lower-level recovery commands

Use these only for diagnosis or controlled recovery:

```text
make prepare-test-release
make prepare-formal-release
make release-check
make release-tag
make test-release-dmg
make release-dmg
make install-test-release-dmg
make install-release-dmg
```

`make install-darwin` remains a local development install and does not allocate
a release identity. Replacing the app bundle does not replace local mail data
under `~/Library/Application Support/aulycmail` or Keychain credentials.
