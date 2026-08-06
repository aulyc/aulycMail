# Changelog

All notable changes to aulycMail are documented here.

## [Unreleased]

- Clarified backup progress by separating existing-backup verification from
  message export and showing verification as an indeterminate stage.
- Added an explicit Aerion foundation acknowledgement and official project link
  to the About page.
- Matched the Settings help-popover arrow to its background, removing the bright
  triangle visible in dark appearance.

## [0.7.4] — 2026-08-06

- Stabilized DMG packaging by retrying Finder registration of newly mounted
  installer volumes before applying the Finder window layout.

## [0.7.4-beta.2] — 2026-08-06

- Matched the Settings sidebar's System Update title color to the adjacent
  version summary while preserving semantic colors for update availability.

## [0.7.4-beta.1] — 2026-08-06

- Improved the software-update confirmation with user-facing installation copy,
  a compact real-time progress bar, and a fixed-width restart button throughout
  download, verification, and installation.
- Added a titled, left-aligned system-update status beside the Settings version
  summary, including distinct up-to-date and new-version states.

## [0.7.3] — 2026-08-05

- Simplified Settings by auto-saving ordinary preferences, removing the bottom
  Save/Cancel/Close bar, and keeping a fixed close control available throughout
  the dialog while account and credential editors retain explicit transactions.
- Kept the About title and copyright visible while only its middle document
  content scrolls, and made auto-save failures restore backend-confirmed values.

## [0.7.2] — 2026-08-05

- Removed the left inset focus marker from document-style links in Settings,
  preventing rounded controls such as Software Update from showing a blue
  crescent while preserving clear keyboard selection feedback.

## [0.7.1] — 2026-08-05

- Fixed in-app update installation by resolving the containing App bundle
  correctly, running the replacement helper from the signed installed bundle,
  and requiring a readiness handshake before the current app exits.
- Distinguished update-installation failures from update-check failures in the
  Settings and About status text, and added a sanitized updater warning for
  diagnostics.
- Users running 0.6.0 or 0.7.0 must install this release's DMG manually once
  because the updater embedded in those versions cannot complete replacement;
  subsequent in-app updates use the repaired handoff.

## [0.7.0] — 2026-08-05

- Refined Settings keyboard focus indicators, redesigned the About page as a
  document-style overview, and added grouped SF Symbol icons to the macOS menu
  bar menu.

## [0.6.0] — 2026-08-05

- Added a signed in-app update flow with GitHub-first and Gitee-fallback
  discovery, full artifact trust verification, and explicit confirmation before
  download, installation, or restart.
- Hardened compose, draft, send, attachment, backup, account, and synchronization
  workflows, including consistent selectable-folder resolution for Drafts and
  Sent mail and safer lifecycle handling when the app runs in the background.
- Improved Settings, composer, contacts, message viewing, keyboard navigation,
  legal-document links, and update-status feedback across the Svelte interface.
- Expanded Go integration coverage and browser-like frontend interaction tests,
  and added fail-closed coverage, lint, dead-code, generated-binding, packaging,
  and release verification gates.

## [0.6.0-beta.23] — 2026-07-31

- Fixed scheduled background sync so INBOX, Drafts, and Sent always perform
  incremental UID checks even when an IMAP server returns stale STATUS
  snapshots, and the status bar advances only after the required account sync
  succeeds.

## [0.6.0-beta.22] — 2026-07-30

- Fixed Settings keyboard activation and focus handoff for the Clear Logs menu
  and nested information or confirmation dialogs, preventing inactive Return
  presses and macOS alert sounds.
- Corrected Activity Log vertical navigation so it visits expandable log rows
  before Load More and Close while the toolbar continues to use Left/Right.
- Removed the duplicate focus ring around the Enhanced Keyboard Navigation
  help icon.

## [0.6.0-beta.21] — 2026-07-30

- Improved Settings keyboard navigation across backup and activity-log controls:
  highlighted the Start Backup action, made the backup directory subcontrols
  follow the intended arrow-key order, restored focus when opening logs, and
  kept Load More selected until pagination is exhausted.
- Changed the activity-log toolbar to use Left/Right within the filter row and
  Up/Down to leave it, including direct navigation to Load More or Close.
- Moved Clear Offline Body Cache beside Test Connection in the account footer
  and report its result with a toast.
- Renamed the activity-log cleanup action to Clear Logs, aligned its trigger
  and menu widths, and removed the redundant native summary hover tooltip.

## [0.6.0-beta.20] — 2026-07-28

- Fixed account reordering so keyboard focus and the selected action remain on
  the moved account after its row is re-rendered.

## [0.6.0-beta.19] — 2026-07-28

- Changed account-row action navigation so Left/Right switches actions within
  one account and Up/Down keeps the same action while moving between accounts.
- Replaced the inline Enhanced Keyboard Navigation explanation with an
  accessible question-mark popover.
- Fixed Escape so an open Settings dropdown closes before keyboard browsing
  resumes.

## [0.6.0-beta.18] — 2026-07-28

- Kept one blue focus indicator visible around a Settings dropdown while its
  menu is open, without restoring the previous duplicate keyboard outline.

## [0.6.0-beta.17] — 2026-07-27

- Added a saved Enhanced Keyboard Navigation setting that keeps the four-region
  orange Tab model when enabled, disables app-specific keyboard actions when
  disabled, and leaves Command+F search available in both states.
- Added a searchable Shift+F10 action list for every visible action in the
  current region, completed composer toolbar and attachment keyboard access,
  and fixed macOS Option-letter shortcuts by matching physical key codes.

## [0.6.0-beta.16] — 2026-07-27

- Test release.

## [0.6.0-beta.15] — 2026-07-27

- Changed Settings footer navigation so Left/Right switches between Cancel and
  Save, while Up/Down from either action returns to the settings controls.
- Added a destructive confirmation before removing an address or domain from
  the remote-image whitelist.
- Fixed account switching in Contacts so the selected row and scroll position
  start at the first contact in the visible A–Z or Z–A order.

## [0.6.0-beta.14] — 2026-07-26

- Kept open Settings dropdown triggers free of a second native focus ring, and
  made the Save action fill only when it is the current keyboard selection.
- Kept the Settings unsaved indicator synchronized after Save by making the
  saved draft baselines reactive.

## [0.6.0-beta.13] — 2026-07-25

- Fixed Settings Select detection to use the rendered trigger contract, so
  Enter opens the highlighted dropdown instead of only moving DOM focus to it.

## [0.6.0-beta.12] — 2026-07-24

- Fixed Settings dropdown activation so one Enter opens the native menu, and
  prevented native focus from leaving a second blue outline behind when
  keyboard selection moves between controls.

## [0.6.0-beta.11] — 2026-07-24

- Fixed Settings dropdown keyboard activation so one Enter opens the selected
  control, Enter confirms and closes it, and subsequent Up/Down moves between
  controls instead of reopening the previous dropdown.
- Made Cancel and Save distinct Up/Down navigation stops and added a visible
  keyboard-selection outline around the active footer action.

## [0.6.0-beta.10] — 2026-07-24

- Changed Settings right-pane keyboard navigation to select the actual form
  control with a blue inset outline instead of tinting the entire settings row.
- Made Enter open the selected dropdown in native input mode, then restore
  control navigation after confirmation or Escape; the final setting now moves
  into a keyboard-navigable Cancel/Save action group.

## [0.6.0-beta.9] — 2026-07-24

- Added All Contacts and Refresh as one wrapping Contacts sidebar action group:
  Up/Down enters or leaves it, Left/Right switches actions, and Enter or Space activates one.
- Kept the Compose and All Contacts solid highlights exclusive to keyboard
  selection, so selecting a mailbox or contact account no longer highlights the top action.
- Synchronized the feature-rail selection with the visible Mail or Contacts
  pane after keyboard focus leaves the restored Settings entry.
- Kept the Settings category background synchronized with the blue active text
  during keyboard navigation, without leaving a stale pointer-hover highlight.
- Made message-list keyboard selection update instantly instead of leaving a
  300 ms color trail, while retaining the existing pointer hover transition.

## [0.6.0-beta.8] — 2026-07-22

- Distinguished Junk and Spam unread counts with neutral gray sidebar badges
  while preserving the Dock badge's existing exclusion of junk mail.

## [0.6.0-beta.7] — 2026-07-22

- Fixed scheduled no-change folder probes leaving account sync spinners and the
  status bar active after backend work had already completed.

## [0.6.0-beta.6] — 2026-07-22

- Added right-side Settings control navigation so Up/Down selects visible rows
  and Enter or Space enters the selected control without overriding native input behavior.
- Restored focus to the feature rail after closing Settings so its left-edge
  selection marker remains without drawing a full outline around the gear.
- Included directory-only IMAP groups such as “Other folders” in sidebar
  arrow-key navigation, with Enter or Space available to expand and collapse them.
- Unified account headers, directory groups, and folders under one blue sidebar
  keyboard marker so the previous mailbox no longer stays highlighted separately.
- Added Compose and Sync as one wrapping sidebar action group: Up/Down enters
  or leaves it, Left/Right switches actions, and Enter or Space activates one.
- Synchronized Settings category arrow-key navigation with the active page so
  the highlighted category and displayed content move together immediately.
- Prevented the Contacts list from remaining on a permanent loading placeholder
  when a bridge or detail request stalls, while keeping existing rows visible
  during background refreshes and offering a retry after a timed-out first load.
- Reduced background mail energy use without changing configured sync cadence
  or first-IDLE delivery latency: account work is coordinated, automatic bursts
  are coalesced, manual sync takes priority, and incomplete remote probes fall
  back to the existing full sync scope.
- Reduced maintenance churn by throttling backup progress events, loading backup
  index keys in bulk, and resuming FTS indexing from committed batches.
- Added sanitized IMAP IDLE health categories without recording credentials,
  server addresses, or complete connection errors.

## [0.6.0-beta.5] — 2026-07-20

- Merged feature-rail keyboard selection and activation so Up/Down immediately
  switches to the next destination, with one blue selection marker instead of
  a separate shadow-only state.

## [0.6.0-beta.4] — 2026-07-20

- Made Up/Down in the feature navigation rail immediately switch Mail or
  Contacts, or open Settings, while that region is active.

## [0.6.0-beta.3] — 2026-07-20

- Removed the obsolete pane-focus flash so changing keyboard regions no longer
  draws a transient blue inset frame.
- Made the Dock unread badge follow the current macOS notification badge
  permission and refresh when the app becomes active.

## [0.6.0-beta.2] — 2026-07-20

- Simplified keyboard navigation visuals by keeping the top region accent and
  selection backgrounds while removing persistent inset frames, and kept mail
  sidebar DOM focus on the region container to prevent stale native outlines.

## [0.6.0-beta.1] — 2026-07-20

- Aligned the Backup Viewer directory picker with the right edge of the message
  list.
- Restored consistent modal keyboard behavior, including Escape-to-close, Tab
  focus containment and restoration, mail-search Escape handling, and scoped
  folder-picker navigation keys.
- Unified Mail and Contacts around four persistent keyboard regions with
  cyclic Tab navigation, independent region/selection indicators, input and
  IME safeguards, and two-region Settings navigation.
- Kept non-empty mail lists synchronized with a visible detail after Escape,
  folder changes, refreshes, and deletion; removed Conversation Viewer Tab
  message switching and converted Backup Viewer messages to roving tabindex.

## [0.5.3] — 2026-07-19

- Moved the Backup Viewer attachment marker to the left edge of message rows.
- Aligned the Backup Viewer account controls with the left edge of the message
  detail cards.

## [0.5.2] — 2026-07-18

- Stabilized the Backup Progress dialog height by reserving its current-target
  row across active and idle states.

## [0.5.1] — 2026-07-18

- Adopted release-core 2.0.1 host-credential verification rules for
  fail-closed formal releases.

## [0.5.0] — 2026-07-18

- Added Finder **Open With** integration that creates a new message and
  attaches the selected regular files without replacing an existing draft.
- Moved the selected-account checkmark to the left edge of each Backup Viewer
  mailbox option while preserving aligned labels and message counts.
- Stabilized Backup Viewer message rows with a single-line timestamp and one
  right-edge attachment marker instead of a redundant paperclip icon.

## [0.4.1] — 2026-07-18

- Refined backup and activity-log settings with compact aligned selectors,
  inline backup composition notes, consistent status typography, and a single
  close action on pages without editable settings, plus aligned About-page
  link icons and labels.
- Prevented a high-volume synchronization day from evicting earlier activity
  history by applying the 1,000-entry limit per activity type and UTC day while
  retaining the existing 30-day age limit.

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
