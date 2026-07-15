# Changelog

All notable changes to aulycmail are documented here.

## [Unreleased]

## [0.3.92] — 2026-07-15

### Changed

- Added a single SemVer source, independent build counter, immutable release-tag checks, and auditable DMG manifests.
- Added automatic test/formal release preparation, release commits, version selection, and channel-specific signing.
- Added installed-version verification across runtime, macOS bundle metadata, Git commit, and release checksum.
- Refined backup settings row layout and spacing.

## [0.3.0] — 2026

- Initial release of aulycmail, a lightweight email client for macOS.
- Database migration v47 removes legacy CardDAV/remote-contact records and OAuth token metadata; export any legacy remote contacts before upgrading if needed.
