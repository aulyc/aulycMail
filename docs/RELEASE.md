# Release Process

See [VERSIONING.md](VERSIONING.md) for the version contract.

## Common prerequisite

Commit all feature, fix, test, and documentation changes first:

```bash
git status --short
```

Both release entrypoints refuse a dirty worktree. They automatically update the
version/build, promote `CHANGELOG.md` notes, create the dedicated release
commit, run quality gates, create the annotated no-`v` tag, build the DMG,
install it, and verify the installed version.

If automatic commit classification is ambiguous, pass
`RELEASE_BUMP=patch|minor|major`. The default is `auto`.

## Test release: ad-hoc signed

Run:

```bash
GOCACHE=/Users/crp/Projects/aulycmail/.cache/go-build \
  make release-test
```

This normally converts `0.3.92-dev` to `0.3.92-beta.1`, increments the build
number, creates `chore: release 0.3.92-beta.1`, runs all checks, creates tag
`0.3.92-beta.1`, rebuilds the exact tagged app, ad-hoc signs the app and DMG,
and installs the test DMG into `/Applications`.

The manifest records `signatureType: "adhoc"` and has no notarization ID. The
installer requires the explicit internal `--allow-adhoc` path, verifies the
ad-hoc signatures, checksum, bundle version/build/commit, and binary
`--version`. It deliberately skips Gatekeeper assessment because an ad-hoc
signature is not an Apple trust assertion. The normal/formal installer refuses
this signature type.

Use this channel for internal testing only. It does not upload anything to
Apple and must never be described as signed with Developer ID or notarized.

## Formal release: Developer ID and notarization

Prerequisites:

- `Developer ID Application` certificate installed in the login keychain;
- `notarytool` credentials stored under the `aulyc-notary` Keychain profile.

Run:

```bash
GOCACHE=/Users/crp/Projects/aulycmail/.cache/go-build \
  make release-formal \
  SIGN_IDENTITY="Developer ID Application: nan ma (M9M7M2ARFD)" \
  NOTARY_PROFILE=aulyc-notary
```

When the current version is a beta/RC, preparation promotes the same base to
stable (`0.3.92-beta.1 -> 0.3.92`) and allocates a new build. The command then
creates the stable release commit/tag, reruns every gate, Developer ID signs
the app and DMG, uploads the DMG to Apple, requires notarization status
`Accepted`, staples and validates the ticket, verifies Gatekeeper, installs the
DMG, and verifies the installed runtime identity.

Credentials remain in Keychain and are never written to logs or manifests.

## Automatic release gates

Both channels run:

- version source and derived-file verification;
- release version/build/changelog verification;
- clean-worktree verification;
- version and release-preparation unit tests;
- all Go tests and Go lint/vet;
- frontend unit tests, Svelte checks, i18n checks, ESLint, and knip;
- production frontend and Go build;
- exact annotated tag verification.

Formal release adds Developer ID, notarization, stapling, and Gatekeeper gates.

## Safe retry behavior

- Before a tag exists, a failed gate reuses the same unpublished version and
  build. After fixes are committed, rerunning creates a fresh release commit
  without needlessly consuming another version.
- If the exact tag already exists at the current release commit, rerunning
  verifies and reuses it; the tag is never moved.
- After a release is published/tagged, subsequent changes create a new beta/RC
  or stable version and increment the build.

## Lower-level targets

The all-in-one commands above are the normal interface. These targets remain
available for diagnosis or controlled recovery:

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

`make install-darwin` remains a local developer installation and is not a test
release.

Replacing `/Applications/aulycmail.app` does not replace local mail data in
`~/Library/Application Support/aulycmail`.
