# AGENTS.md

## Repository purpose

`aulycMail` is a local-first macOS desktop mail client. Preserve reliable
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
- `build/`: checked-in packaging inputs only; generated bundles live under
  hidden `.cache/build/` and `.cache/wails/` paths.
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
- Test release: `GOCACHE=/Users/crp/Projects/aulyc/aulycMail/.cache/go-build make release-test`.
- Test release and installation: `GOCACHE=/Users/crp/Projects/aulyc/aulycMail/.cache/go-build make release-test-install`.
- Formal release: `GOCACHE=/Users/crp/Projects/aulyc/aulycMail/.cache/go-build make release-formal SIGN_IDENTITY="Developer ID Application: nan ma (M9M7M2ARFD)" NOTARY_PROFILE=aulyc-notary`.
- Formal release and installation: use the same credentials with `make release-formal-install`.
- Formal release backup: remote `backup`, branch `main`; `make release-formal`
  must run the central GitHub preflight before changing release metadata, then
  atomically push and verify the formal release commit and annotated tag after
  the signed/notarized DMG is verified. The central gate also finalizes and verifies the
  GitHub source fields in the release provenance.

`测试发版`、`正式发版`和`完整发版`都不写入 `/Applications`。只有带
`-install` 的组合目标才执行“测试发版安装”或“正式发版安装”。
`install-test-release-dmg` 与 `install-release-dmg` 只安装既有 DMG，不构建或
创建发布。纯发版报告 `installationStatus: not-requested`。

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
- Do not commit `frontend/node_modules/`, `frontend/dist/`, `.cache/`, local
  databases, logs, DMGs, or `dist/` artifacts.
- Production builds, Wails development, and `make clean` remove the obsolete
  visible `build/bin/` output before continuing.

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
  Hardened Runtime and Apple notarization distributed through explicit
  `aulyc-dual-mirror-v1` public mirrors.
- Authoritative version and build-number source: `version.json`; every test and
  formal release requires a new positive build number.
- Product metadata: the built App's `Info.plist`, Mach-O executable, Wails
  metadata, embedded frontend assets, and code-signing identity.
- Product name, App bundle, and executable: `aulycMail`, `aulycMail.app`, and
  `aulycMail`. Preserve compatibility identities `com.aulyc.aulycmail`,
  `~/Library/Application Support/aulycmail`, the `aulycmail` Keychain service,
  and the `aulyc.local/aulycmail` Go module path.
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
  and the actual `/Applications/aulycMail.app` installation.
- Published tags and artifacts are immutable and must not be overwritten.
- Formal releases are incomplete until `backup/main` and the matching annotated
  tag are verified at the exact release commit and the provenance records
  `aulyc/aulycMail`, `main`, both remote commits, and the verification time.
  Test releases are not pushed by this project-owned backup step.
- Read `docs/VERSIONING.md` and `docs/RELEASE.md` before release work.

### Dual-mirror release policy

- Explicit policy: `aulyc-dual-mirror-v1` `1.7.0`; the Release Profile remains
  `macos-arm64-app`.
- Public source authority and GitHub distribution repository:
  `aulyc/aulycMail`; the public Gitee distribution mirror uses the same
  `aulyc/aulycMail` owner/repository identity. Gitee never receives source Git
  pushes. The `macos-compact` / `dual-manifest` channel publishes only the
  verified DMG as each Release attachment. GitHub `latest.json` uses the
  dedicated `release-channel` branch so publication never advances the source
  `main` branch; versioned provenance is stored under `updates/<version>/` on
  both update branches, while checksum sidecars remain local verification
  evidence.
- Project adapter: `bash tools/dual-mirror-release.sh
  <prepare|preflight|publish|verify> ...`; full contract:
  `docs/DUAL_MIRROR_RELEASE.md`.
- Updater: `internal/updater` plus the narrow `app/update.go` Wails bridge.
  Automatic checks are preference-controlled and throttled; manual checks
  always remain available. The updater accepts `dual-mirror-latest:2` with raw
  versioned-provenance URLs, retains read-only Schema v1 compatibility, and
  uses fixed GitHub-first/Gitee fallback with identical version/build/Commit/
  Bundle ID/arm64/SHA-256/provenance/Developer ID/notarization/Gatekeeper
  verification before it may stage an in-place replacement. Download,
  installation, and restart require explicit user confirmation.
- Only an explicitly authorized `publish` may write remote state. Missing
  mirrors, one-sided failure or conflicts must fail/record partial state;
  never push source to Gitee or overwrite an old release.
