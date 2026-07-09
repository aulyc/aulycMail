# Background Email Backup Design

## Goals

- Starting a backup should return immediately so the settings dialog can close while the job continues in the app process.
- Only one backup run may be active at a time.
- Large mailboxes should avoid per-message connection and mailbox selection overhead.
- Per-message failures should be recorded and should not stop the remaining backup.
- UID rows that the server no longer returns should be separated as missing,
  not counted as hard failures.

## Backend Flow

1. `StartEmailBackup` claims the process-local backup tracker, emits initial progress, starts a goroutine, and returns `BackupRunState`.
2. The goroutine runs the same backup pipeline used by `RunEmailBackup`.
3. `GetBackupRunState` remains the recovery point for reopened UI. It exposes the last progress even after completion or failure.
4. Completion and error states are emitted through `backup:progress`.

## Export Speed

Messages are still written as standard `.eml` files, but export work is grouped by `accountID + folderID`:

- indexed files are skipped locally without IMAP work;
- missing files are processed in bounded chunks;
- each chunk selects the mailbox once and fetches many UIDs in one IMAP command;
- each body literal is streamed directly to a temp file and renamed only after a successful copy.
- each batch has an idle watchdog so a stalled IMAP FETCH is discarded instead
  of leaving the backup spinner running forever.

The first speed target is to remove redundant connection/select/FETCH setup for tens of thousands of messages. Parallel folder export is intentionally deferred to avoid overloading providers and to keep rate-limit behavior predictable.

## Failure Handling

- Existing index path traversal protections continue to guard restored paths.
- A failed message records account, folder, UID, subject, and error in the report.
- A missing message records the same details under `missingMessages`; rerunning
  the backup will retry those UIDs without re-exporting already indexed files.
- A fetch response with a small RFC822 size mismatch is accepted and logged, matching the current backup behavior.
- If the app process exits, the background job stops with the process. Completed files are preserved, and the next run resumes from the index once it is saved at the end of a run.
