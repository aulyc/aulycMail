# aulycmail Security Best Practices Report

Date: 2026-07-01

## Scope

Reviewed and remediated the local Wails desktop mail client codebase with focus on:

- Go backend: TLS certificate trust, credential handling, URL opening, attachment saving, IMAP/SMTP/network clients, SQL construction.
- Svelte/Wails frontend: sanitized email rendering, iframe boundary, postMessage handling, browser URL opening, sensitive logging.
- Repository controls: dependency/security environment flags and obvious dangerous constructs.

## Remediation Summary

All seven findings from the initial review have been addressed in code and covered by targeted checks or tests.

1. Fixed host-scoped TOFU certificate trust.
2. Fixed unsafe attachment filename handling for batch/default saves.
3. Removed the email-viewer URL fallback that bypassed backend scheme checks.
4. Added URL log redaction for backend and frontend email-link paths.
5. Retired the extension OAuth HTTP-client path when OAuth support was removed.
6. Added strict parent-side iframe message validation.
7. Added bounded attachment extraction reads.

## Fixed Findings

### SBP-01 High: TOFU certificate trust is now scoped to host

Status: Fixed.

Changes:

- `internal/certificate/store.go:29` now checks trust by `host + fingerprint`.
- `internal/certificate/store.go:58` normalizes and validates host/fingerprint before permanent trust.
- `internal/certificate/store.go:81` stores session trust by host/fingerprint.
- `internal/certificate/verifier.go:39` passes the active TLS host into the trust check.
- `internal/database/migrations.go:522` creates new installs with `UNIQUE(host, fingerprint)`.
- `internal/database/migrations.go:1341` adds migration v42 to convert existing databases to host-scoped certificate trust.

Tests:

- `internal/certificate/certificate_test.go` covers session and permanent trust scoping.
- `internal/database/database_test.go` covers duplicate fingerprints across different hosts and duplicate host/fingerprint rejection.

### SBP-02 High: Save-all attachments no longer trust raw email filenames

Status: Fixed.

Changes:

- `internal/email/download.go:246` adds `SafeAttachmentFilename`.
- `internal/email/download.go:277` adds collision-safe `UniqueAttachmentPath`.
- `app/attachment.go:382` now uses `email.UniqueAttachmentPath(saveDir, att.Filename)` instead of joining raw filenames.
- `app/attachment.go:178`, `app/attachment.go:193`, and `app/attachment.go:405` use sanitized default filenames for save dialogs and portal defaults.
- `internal/email/download.go:326` applies the same safe path generation for default attachment downloads.

Tests:

- `internal/email/download_test.go` covers Unix traversal, Windows traversal, absolute paths, control characters, empty base names, collision handling, and default-path saving.

### SBP-03 High: Email links can no longer bypass backend URL policy

Status: Fixed.

Changes:

- `frontend/src/lib/components/viewer/EmailBody.svelte:157` adds parsed allowlist validation for `http:`, `https:`, and `mailto:`.
- `frontend/src/lib/components/viewer/EmailBody.svelte:513` now calls only backend `OpenURL`; direct `BrowserOpenURL` fallback was removed.
- `app/app.go:1052` now parses URL schemes instead of using prefix matching.

Tests:

- `app/app_test.go` covers accepted and rejected URL schemes, including uppercase HTTPS, malformed HTTP, `file:`, `data:`, and relative paths.
- `npm run check` verifies the Svelte/TypeScript changes.

### SBP-04 Medium: URL logging is redacted

Status: Fixed.

Changes:

- `app/app.go:971`, `app/app.go:983`, `app/app.go:1020`, and `app/app.go:1024` log redacted URLs.
- `app/app.go:1028` strips query, fragment, userinfo, and disallowed scheme payloads.
- `app/compose.go` no longer logs full invalid external `mailto:` URLs.
- `frontend/src/lib/components/viewer/EmailBody.svelte:169` redacts frontend email-link logging.

Tests:

- `app/app_test.go` covers HTTPS query stripping, userinfo removal, mailto query stripping, disallowed scheme redaction, and relative URL redaction.

### SBP-05 Medium: Extension OAuth HTTP clients retired

Status: Fixed.

Changes:

- The extension OAuth broker was removed with the rest of OAuth support.
- No extension-owned authenticated HTTP client is constructed by current code.

Tests:

- Covered by the repository-wide Go test/build checks after OAuth removal.

### SBP-06 Medium: Email iframe message handling is schema-validated

Status: Fixed.

Changes:

- `frontend/src/lib/components/viewer/EmailBody.svelte:107` adds strict message parsing by type.
- `frontend/src/lib/components/viewer/EmailBody.svelte:531` rejects messages that fail source or schema validation before taking action.
- `frontend/src/lib/components/viewer/EmailBody.svelte:599` and `frontend/src/lib/components/viewer/EmailBody.svelte:794` document why `targetOrigin: '*'` remains necessary for `srcdoc` iframes with opaque origins.

Tests:

- `npm run check` passes.
- `npm run build` passes.

### SBP-07 Medium: Attachment extraction is bounded

Status: Fixed.

Changes:

- `internal/email/download.go:22` defines `MaxAttachmentContentSize` at 50 MiB.
- `internal/email/download.go:340` adds bounded reads with `io.LimitReader`.
- `internal/email/attachment.go:121` and `internal/email/attachment.go:157` use bounded reads for MIME and TNEF extraction.
- `internal/email/attachment.go:170` skips oversized TNEF attachment payloads.
- `internal/email/download.go:357` enforces the limit after transfer decoding.

Tests:

- `internal/email/download_test.go` covers oversized bounded reads.
- `go test ./internal/email` passes.

## Verification

Commands run successfully:

- `GOCACHE=/tmp/aulycmail-go-build-cache go test ./app ./internal/certificate ./internal/email ./internal/database`
- `GOCACHE=/tmp/aulycmail-go-build-cache go test ./...`
- `npm run check` in `frontend/`
- `npm run build` in `frontend/`
- `git diff --check` for the modified security files

Non-failing warnings handled after remediation:

- Go duplicate `-lobjc` linker warning is suppressed through the Darwin Makefile build/test environment.
- Frontend Browserslist data was refreshed with `npx update-browserslist-db@latest`.
- Icon generation no longer reports missing `mdi:foo` or `simple-icons:fastmail`.
- Vite no longer reports dynamic/static Wails import chunking or oversized chunk warnings.
