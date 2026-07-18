# Changelog

All notable changes to aulycMail are documented here.

## [Unreleased]

## [0.4.0] — 2026-07-18

- Refined the backup settings layout with a header action, compact latest-backup
  summary, horizontal progress feedback, and expandable activity details that
  keep only one row open at a time.
- Added a reproducible npm dependency security-audit command and CI gate so a
  clean frontend install is audited before project checks and production builds.

## [0.3.101] — 2026-07-18

- Replaced the duplicate inline backup result panel and horizontal progress bar
  with a circular progress dialog that can continue in the background and be
  reopened while the backup runs.
- Standardized About-page information and close actions so they activate on a
  completed click instead of immediately on pointer down.

## [0.3.100] — 2026-07-18

- Prevented the formal GitHub gate from treating the version-derived
  `wails.json` update as release-standards drift after release preparation.

## [0.3.99] — 2026-07-18

- Moved local and Wails-generated app bundles into hidden `.cache` paths so
  macOS does not list project builds as duplicates of the installed app.
- Removed transient macOS `.fseventsd` metadata from generated DMG installers
  and made packaging fail if it reappears in the final image.
- Clarified backup results as checked, backed up, and not backed up, with
  explicit reasons for server-missing messages, unreadable sources, and
  processing failures while preserving older activity-log compatibility.
- Added the standard Command-Q shortcut to the macOS status-bar Quit action.

## [0.3.98] — 2026-07-17

- Renamed the product, app bundle, executable, and new release artifacts to
  `aulycMail` while preserving the existing bundle ID, local data, Keychain
  service, and historical release compatibility.
- Simplified backup status presentation by removing redundant button and task
  icons, showing “备份中…” while running, adding the total to recent summaries,
  and removing the duplicate current/total counter.
- Required the pinned `golangci-lint` v2.12.2 gate before test and formal
  release candidate builds, while retaining the explicit `go vet` fallback for
  local development.
- Updated the About page website to `www.aulyc.com`, removed its redundant
  product tagline, and eliminated the doubled separator above “Load more” in
  the activity log.
- Fixed folders excluded from background synchronization, such as Junk, so
  opening them catches up from the server and reconciles unread badges with
  the local message list without starting duplicate syncs.

## [0.3.97] — 2026-07-15

- Restored message bodies, original sources, and attachment downloads for
  legacy local records left under IMAP hierarchy-only folders by resolving
  their verified `.eml` counterparts from the backup index.
- Separated server-unavailable messages from true backup failures so backup
  totals and activity logs accurately report unavailable, missing, and failed
  records.
- Prevented startup from restoring missing or non-selectable folders and now
  loads the current folder tree before applying the saved selection.

## [0.3.96] — 2026-07-15

- Fixed IMAP `\\Noselect` containers so they behave as hierarchy-only folders:
  descendant unread badges remain accurate while misleading own cached message
  totals are suppressed.
- Preserved cached messages under hierarchy-only containers without exposing
  them as selectable mailboxes or repeatedly attempting unavailable body
  downloads.
- Updated vulnerable frontend build and editor dependencies within their
  existing compatible major versions.

## [0.3.95] — 2026-07-15

- Fixed release verification for mounted DMG paths whose volume names contain
  spaces.

## [0.3.94] — 2026-07-15

- Fixed formal release validation so clean frontend dependency installs are
  verified before an immutable tag is created.

## [0.3.93] — 2026-07-15

- Stable release.

## [0.3.92] — 2026-07-15

### Changed

- Added a single SemVer source, independent build counter, immutable release-tag checks, and auditable DMG manifests.
- Added automatic test/formal release preparation, release commits, version selection, and channel-specific signing.
- Added installed-version verification across runtime, macOS bundle metadata, Git commit, and release checksum.
- Refined backup settings row layout and spacing.
- Fixed formal release builds from clean tagged checkouts where ignored frontend output is initially absent.

## [0.3.0] — 2026

- Initial release of aulycMail, a lightweight email client for macOS.
- Database migration v47 removes legacy CardDAV/remote-contact records and OAuth token metadata; export any legacy remote contacts before upgrading if needed.
