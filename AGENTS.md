# AGENTS.md

## Repository purpose

`aulycmail` is a local-first macOS desktop mail client. Preserve reliable
IMAP/SMTP behavior, local mail data, user settings, credentials, and release
provenance over convenience or broad rewrites.

## Stack and platform

- Go backend and application orchestration.
- Wails bridge between Go and the webview.
- Svelte 5 and TypeScript frontend.
- SQLite local persistence.
- Native macOS packaging and integration.
- Supported target: macOS on Apple Silicon (`arm64`) only.

## Repository map

- `main.go`, `preflight.go`: process startup, single-instance/preflight, Wails wiring.
- `app/`: Wails-facing application API and cross-domain orchestration.
- `internal/`: domain stores, SQLite, IMAP/SMTP, credentials, sync, logging, and platform code.
- `frontend/src/`: Svelte UI, frontend state, presentation, and user interaction.
- `frontend/wailsjs/`: generated Wails JavaScript/TypeScript bindings.
- `tools/`: versioning, release, macOS packaging, verification, and generated-file helpers.
- `build/`: checked-in packaging inputs; generated bundles live under `build/bin/`.
- `docs/`: current development, versioning, release, privacy, and operational documentation.

## Architecture boundaries

1. `internal/` owns persistence, mail protocols, credentials, platform behavior,
   and domain rules; it must not depend on the Svelte frontend.
2. `app/` is the Wails bridge and orchestration layer. Keep exported bridge
   methods narrow, validate inputs, and do not expose database handles,
   credential material, or internal persistence details.
3. `frontend/src/` owns UI behavior only. Access backend capabilities through
   generated Wails bindings and events, not through direct database, filesystem,
   Keychain, IMAP, or SMTP access.
4. After changing an exported Go API, exported model, or Wails bridge surface,
   run `make generate`, inspect `frontend/wailsjs/`, and include the required
   generated binding changes with the source change.

## Command mapping

- Development: `make dev`; suspected Go data race: `make dev-race`.
- Generate Wails bindings: `make generate`.
- Local production build: `make build`.
- Go-only gate: `make check-go`.
- Release Go lint gate: `make release-golangci-lint`.
- Frontend-only gate: `make check-frontend`.
- Shared development gate: `make check`.
- Full non-release CI gate and production build: `make ci`.
- Local development installation: `make install-darwin`; this is not a release.
- Test release: `GOCACHE=/Users/crp/Projects/aulycmail/.cache/go-build make release-test`.
- Formal release: `GOCACHE=/Users/crp/Projects/aulycmail/.cache/go-build make release-formal SIGN_IDENTITY="Developer ID Application: nan ma (M9M7M2ARFD)" NOTARY_PROFILE=aulyc-notary`.

Do not install, release, tag, sign, notarize, or publish unless the user
explicitly requests that operation.

## Quality gates by change scope

- Go-only: `make check-go`.
- `make check-go` runs `golangci-lint` when available and otherwise reports an
  explicit development-only fallback to `go vet`.
- Frontend-only: `make check-frontend`.
- Version/release tools, generated bindings, cross-stack behavior, dependencies,
  build configuration, or shared contracts: `make check`.
- Before handoff of a build-affecting change: `make ci`.
- Documentation-only: run relevant link/text checks plus `git diff --check`; run
  broader gates when commands, contracts, or generated-file rules changed.
- Release identity: `make release-check` before creating or accepting a tag.
  It must find `golangci-lint` exactly at v2.12.2 via `version --json`, verify
  `.golangci.yml`, and run `golangci-lint run`; missing or mismatched tools fail
  without fallback. The required version must not be Make-command-line
  overridable.

Report any gate not run and why. Avoid rerunning a subset already covered by a
successful aggregate target unless diagnosing a failure.

## Generated files and build outputs

- `version.json` is the only version source.
- Version fields in `wails.json`, `frontend/package.json`, and
  `frontend/package-lock.json` are derived; never edit them manually. Use the
  version tools and verify with `make version-check`.
- Treat `frontend/wailsjs/` as generated-but-reviewed source. Regenerate it only
  through Wails plus `tools/normalize_wails_bindings.sh`.
- Do not commit `frontend/node_modules/`, `frontend/dist/`, `build/bin/`, local
  caches, databases, logs, DMGs, or `dist/` artifacts.

## Data, settings, and credentials

- Database migrations must remain compatible with existing user data, be
  monotonic and transactional, and include migration/upgrade tests. Never fix a
  migration by deleting or recreating a user's database or mail data.
- Do not add user mail, databases, attachments, settings, passwords, tokens,
  Keychain values, or credential fallbacks to build artifacts, fixtures,
  manifests, screenshots, or logs.
- Use synthetic data and temporary databases in tests.
- Settings UI remains `draft-only-until-Save`: editing controls must not mutate
  persisted or live application behavior until Save succeeds, unless a new
  requirement explicitly changes that contract.

## Logging and sensitive information

- Log operational state and opaque identifiers only when needed.
- Never log message bodies, attachment contents, passwords, tokens, private
  keys, Keychain data, or complete credential-bearing configuration.
- Sanitize errors before exposing them to the frontend or release logs.

## Documentation

- Update documentation only when behavior, commands, architecture, generated
  files, or contracts change.
- Keep `docs/DEVELOPMENT.md`, `docs/VERSIONING.md`, and `docs/RELEASE.md`
  aligned with the actual Makefile and scripts.
- Treat the Makefile and scripts as executable truth; stop and resolve any
  documentation mismatch before release.

## Versioning and Release Profile

- Load and follow the global `general-release-versioning` Skill for every
  version, package, installation, test-release, or formal-release task.
- Release profile: `macos-arm64-app`.
- Architecture and distribution: Apple Silicon `arm64` only; test releases use
  ad-hoc App/DMG signing, while formal releases use a Developer ID DMG with
  Hardened Runtime and Apple notarization.
- Authoritative version and build-number source: `version.json`; every test and
  formal release requires a new positive build number.
- Product metadata: the built App's `Info.plist`, Mach-O executable, Wails
  metadata, embedded frontend assets, and code-signing identity.
- Release provenance: historical `*.manifest.json` files generated beside each
  DMG. They are release provenance evidence, not product metadata; the filename
  remains for compatibility and every new file must declare
  `releaseProfile: macos-arm64-app`.
- Local `make install-darwin` may use dirty development identity, build `0`, and
  ad-hoc signing; it is never a test release.
- Test and formal releases require a clean release-only metadata commit, a
  matching immutable annotated tag without `v`, a pre-tag production candidate,
  and final artifacts built from an isolated detached worktree at that exact tag.
- `make release-check` applies the fail-closed `golangci-lint` v2.12.2 gate to
  every subsequent test and formal release before the shared checks and
  production candidate build.
- Test releases are ad-hoc signed, never notarized, and never claim Gatekeeper
  trust. Formal releases require Developer ID, Hardened Runtime, notarization
  `Accepted`, stapling, stapler validation, and Gatekeeper verification.
- Release provenance must be derived from and cross-checked against Git, the DMG,
  Info.plist, the executable, architecture tools, code signing, notarization,
  and the actual `/Applications/aulycmail.app` installation.
- Published tags and artifacts are immutable and must not be overwritten.
- Read `docs/VERSIONING.md` and `docs/RELEASE.md` before release work.
