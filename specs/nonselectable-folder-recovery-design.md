# Non-selectable IMAP Folder Recovery

## Goal

Treat IMAP mailboxes carrying `\Noselect` as hierarchy-only containers so they
cannot surface stale local messages, trigger impossible body downloads, or be
used as message destinations. Preserve existing local records until an explicit
data-retention decision is made.

## Acceptance Criteria

- Folder LIST synchronization persists whether each mailbox is selectable.
- `\Noselect` folders remain visible as expandable hierarchy nodes but cannot
  be selected, synchronized, searched, or used as move/copy/drop targets.
- A non-selectable folder contributes no own message or unread count; its badge
  may still aggregate counts from selectable descendants.
- Existing cached messages belonging to a newly recognized non-selectable
  folder are retained in SQLite but excluded from normal folder/conversation
  access, avoiding destructive migration behavior.
- Backend APIs reject direct attempts to operate on non-selectable folders even
  if an older frontend or stale UI state submits the folder ID.
- Selectable folders keep current behavior, including legitimate selectable
  parents that also have children.

## Backend and Data Design

- Add `folders.selectable INTEGER NOT NULL DEFAULT 1` in an additive migration.
  The migration does not delete or rewrite message rows.
- Map IMAP `\Noselect` to `selectable=false` during every LIST refresh and
  persist it with the rest of folder metadata.
- When a folder is non-selectable, persist its own `total_count` and
  `unread_count` as zero and skip local-count fallback logic. Descendant counts
  remain independent and are aggregated by the existing tree UI.
- Exclude non-selectable folders from automatic sync lists and reject manual
  message sync, body fetch, server search, and destination operations.
- Keep stale rows for backup/data-safety purposes. They become unreachable from
  normal folder queries because the folder itself cannot be opened.

## Frontend Design

- Render a non-selectable folder as an expandable directory row.
- Clicking the row toggles expansion instead of selecting a message list.
- Remove context-menu and drag/drop destination behavior from that row.
- Keyboard folder navigation skips the hierarchy-only row; its selectable
  children remain navigable whenever the row is expanded.
- If a previously selected folder becomes non-selectable after refresh, clear
  the stale selection so the viewer cannot continue issuing body requests.

## Security and Data Safety

- The migration is additive and transactional.
- No credentials, message bodies, or private headers are added to logs or API
  responses.
- Backend validation is authoritative; frontend disabling is only a usability
  layer.
- Historical message rows are not deleted automatically.

## Test Strategy

- Migration test for the new default and upgrade path.
- IMAP attribute unit test for `\Noselect` detection.
- Folder store and sync tests for persisted selectability and zero own counts.
- Backend error-path tests proving non-selectable folders cannot be synced or
  used as destinations.
- Frontend unit/type checks for selectable and directory-only row behavior.
- Full repository CI and production build.
