# Development Guide

This document describes the current local development and CI-neutral workflow
for the Apple Silicon macOS build of aulycMail. Version and release behavior is
documented separately in [VERSIONING.md](VERSIONING.md) and
[RELEASE.md](RELEASE.md).

## Environment

The project uses Go, Wails, Node.js/npm, Svelte, TypeScript, SQLite, and the
macOS command-line developer tools. On the maintained machine, use:

```bash
export PATH=/Users/crp/.nvm/versions/node/v24.13.0/bin:/Users/crp/go/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin
export GOCACHE=/Users/crp/Projects/aulyc/aulycMail/.cache/go-build
```

The repository intentionally targets macOS `arm64` only. Production builds
fail on a non-Apple-Silicon host.

Install frontend dependencies when needed:

```bash
make frontend-deps
```

For reproducible release linting, install the official precompiled
`golangci-lint` v2.12.2 binary into the Go bin directory already present on
`PATH`:

```bash
curl -sSfL https://golangci-lint.run/install.sh | \
  sh -s -- -b "$(go env GOPATH)/bin" v2.12.2
golangci-lint version --json
```

Do not rely on an unpinned Homebrew upgrade for the release tool. Local
development remains usable without it: `make check-go` reports the missing
tool and falls back to `go vet`. Releases are fail-closed: `make release-check`
requires the machine-readable version to equal `2.12.2`, validates
`.golangci.yml` as a v2 configuration, and runs `golangci-lint run` without a
fallback.

## Architecture

- `main.go` and `preflight.go` assemble the process, application lifecycle, and
  Wails runtime.
- `app/` exposes the narrow Go API consumed by Wails and coordinates domain
  stores and services.
- `internal/` owns SQLite persistence, migrations, mail protocols, sync,
  credentials, logging, and macOS integration.
- `frontend/src/` owns Svelte presentation and interaction state. It calls Go
  only through `frontend/wailsjs/` bindings or Wails events.
- `tools/` owns reproducible version, release, package, and artifact validation.

Do not move persistence or credential logic into the frontend. Do not expose
database handles or secrets through the Wails bridge. Prefer explicit DTOs and
validated exported methods at the bridge boundary.

## Development commands

```text
make dev             Hot-reload development
make dev-race        Development with Go race detection
make generate        Regenerate and normalize Wails bindings
make build           Local production build with development identity
make install-darwin  Local build/install/launch; not a release
```

`make install-darwin` replaces `/Applications/aulycMail.app` and removes a
legacy `/Applications/aulycmail.app` bundle when present. It must not
touch mail databases, settings, or credentials under the user's Application
Support or Keychain storage.

## Quality commands

```text
make fmt-check       Non-mutating gofmt verification
make check-go        fmt-check + Go tests + golangci-lint (or explicit go vet fallback)
make check-frontend  unit tests + svelte-check + i18n + ESLint + knip
make security-audit  npm audit for all production and development dependencies
make check           version checks/tests + check-go + check-frontend
make ci              clean npm ci + security audit + make check + production build
make release-golangci-lint  exact v2.12.2 + config verification + release lint
make release-backup-preflight  verify private backup remote and formal branch
```

Use the smallest relevant gate while iterating, then `make ci` before handing
off build-affecting changes. `make ci` is platform-neutral as a command entry,
but its production build requires the supported macOS Apple Silicon host. It
recreates `frontend/node_modules` from `package-lock.json` and runs
`npm run security:audit` immediately after that clean install.

No CI provider is configured in this repository. The `backup` Git remote is a
private offsite source/tag backup, not a CI provider. After a CI platform is
selected, that provider should invoke `make ci`. Developer ID signing,
notarization, installation, and publication must remain outside normal
pull-request CI.

## Generated files

`version.json` is authoritative. The version fields in `wails.json`,
`frontend/package.json`, and `frontend/package-lock.json` are synchronized by
`tools/version-bump.mjs` and checked by `make version-check`.

`frontend/wailsjs/` is generated and tracked. After changing a Go exported API,
model, or Wails bridge:

```bash
make generate
git diff -- frontend/wailsjs
make check
```

The normalizer removes tool-only whitespace drift. Review semantic binding
changes and commit them with the Go API change. Local production bundles are
generated under `.cache/build/`; Wails development bundles use `.cache/wails/`.
Those paths, `frontend/dist/`, and `dist/` remain untracked. Keeping every
project-built `.app` below hidden `.cache/` prevents macOS from presenting it
as a duplicate of the installed `/Applications/aulycMail.app`. Production
builds, Wails development, and `make clean` also remove the obsolete visible
`build/bin/` output left by older checkouts.

## macOS file-open integration

The packaged App declares `public.data` with `Viewer` role and `Alternate`
handler rank. This makes aulycMail available from Finder's **Open With** menu
for regular files without claiming to be their default editor. Wails routes the
native `OnFileOpen` callback into a short backend batch so a multi-file Finder
selection opens one new message with all selected files attached.

File-open requests received during startup remain queued until the frontend has
loaded accounts and calls `NotifyStartupComplete`. If a composer is already
open, the frontend keeps the new request queued until that composer closes;
this prevents an external file-open action from replacing unsaved mail. File
contents are still read only through the existing Go attachment bridge—the
webview never reads arbitrary filesystem paths directly.

## Database and user-data safety

Migrations live in `internal/database/migrations.go` and run transactionally.
New migrations must advance monotonically, preserve databases created by every
supported previous schema, and include upgrade tests using temporary SQLite
files. Never delete or recreate a user's database to make a migration pass.

Tests use synthetic messages, accounts, settings, credentials, and temporary
directories. User mail, attachments, databases, settings, passwords, tokens,
and Keychain data must never enter fixtures, artifacts, manifests, or logs.

Settings editing follows `draft-only-until-Save`: UI drafts may change inside
the dialog, but persisted and live behavior changes only after Save succeeds.

## Release-tool development

Release tools are covered by `make version-test` and therefore by `make check`.
Tests create disposable Git repositories and annotated fixture tags under the
system temporary directory; they never create tags in the real repository.
Run `bash -n tools/*.sh` after changing shell release or packaging scripts.
