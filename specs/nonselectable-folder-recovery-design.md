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
- Backup runs do not retry IMAP SELECT/FETCH for retained rows whose folder is
  non-selectable. Rows without an existing indexed `.eml` are reported as
  `unavailable`, separately from transport/parser failures.
- An indexed `.eml` in the configured backup directory is a trusted local
  recovery source for an individual message body and its attachment metadata.
- On application startup, a persisted selection that now points at a
  non-selectable folder is cleared before the message list or viewer loads.
- When neither the server nor an indexed `.eml` can provide the raw message,
  the UI explains that only the local envelope record remains; it does not
  imply that another retry can reconstruct the body or attachments.

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
- Backup enumeration reads folder selectability. Existing indexed files remain
  valid and are counted as skipped; unindexed rows in a non-selectable folder
  are counted as unavailable and never sent to the IMAP raw-message streamer.
- Local recovery resolves a message through the configured backup index using
  account ID, folder ID, UIDVALIDITY, and UID. The indexed relative path is
  validated to remain inside the configured directory before it is opened.
- If that obsolete mailbox identity has no file, recovery may reuse an indexed
  copy from another folder in the same account only when the normalized RFC
  `Message-ID` matches. The raw file still passes identity validation before
  parsing or attachment extraction. For legacy rows without `Message-ID`, the
  backend may resolve a counterpart only when the complete cached envelope
  (subject, date, sender, recipients, reply-to, and size) produces exactly one
  selectable-folder match. Zero or multiple matches remain unavailable; a
  subject/date-only match is diagnostic and is never used automatically.
- Recovered raw MIME is identity-checked, parsed, sanitized through the same
  body pipeline as an IMAP fetch, and persisted atomically with replacement
  attachment metadata. Successful recovery clears the prior body-failed flag.
- Selectable folders keep server-first fetching and may fall back to an indexed
  `.eml` after a server failure. Non-selectable folders use only the indexed
  local recovery source.

## Frontend Design

- Render a non-selectable folder as an expandable directory row.
- Clicking the row toggles expansion instead of selecting a message list.
- Remove context-menu and drag/drop destination behavior from that row.
- Keyboard folder navigation skips the hierarchy-only row; its selectable
  children remain navigable whenever the row is expanded.
- If a previously selected folder becomes non-selectable after refresh, clear
  the stale selection so the viewer cannot continue issuing body requests.
- Apply the same validation to persisted startup state after account folders
  load; only restore a folder that still exists and is selectable.
- Show backup `unavailable` separately from `failed` in progress, completion,
  and activity summaries so retained metadata is not presented as an execution
  failure.

## Security and Data Safety

- The migration is additive and transactional.
- No credentials, message bodies, or private headers are added to logs or API
  responses.
- Backend validation is authoritative; frontend disabling is only a usability
  layer.
- Historical message rows are not deleted automatically.
- Backup fallback accepts no arbitrary file path from a message or frontend
  request. It reads only the configured directory, parses the application-owned
  index, rejects absolute/traversing paths, and verifies message identity.
- Recovery has the same raw-message size limit and HTML sanitization as network
  body fetching. Raw MIME, bodies, attachment bytes, and full private headers
  are not logged.
- Body and attachment metadata replacement is transactional; a parse or
  persistence failure leaves the previous local record intact.

## Test Strategy

- Migration test for the new default and upgrade path.
- IMAP attribute unit test for `\Noselect` detection.
- Folder store and sync tests for persisted selectability and zero own counts.
- Backend error-path tests proving non-selectable folders cannot be synced or
  used as destinations.
- Frontend unit/type checks for selectable and directory-only row behavior.
- Backup regression test proving a non-selectable, unindexed row increments
  `unavailable` without calling the raw-message streamer, while an existing
  indexed file or same-`Message-ID` copy is still skipped.
- Backup locator tests for identity lookup, missing entries, and path traversal.
- MIME recovery tests for identity validation, sanitized body persistence,
  attachment replacement, and clearing `body_failed`.
- Startup-state regression test proving non-selectable or missing folder IDs are
  discarded while selectable folder state is restored.
- Full repository CI and production build.
