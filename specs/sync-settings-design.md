# Sync Settings Split

## Goal

Allow archive-friendly accounts to keep all local mail while daily sync stays fast.

## Settings

- Local retention: how long local message rows are kept. `0` means keep all.
- Daily sync mode: `incremental` checks new UIDs between scheduled full checks; `full` performs a full UID validation every sync.
- Full validation frequency: days between full UID validations in incremental mode. `0` means manual only.
- Offline body storage: `on_demand`, `recent`, or `all`. This controls app-side offline reading cache, not backup completeness.
- Offline body cache cleanup: clears saved body HTML/text and parsed attachment records for one account while keeping message index rows.

## Backend Behavior

- Existing `sync_period_days` remains for compatibility.
- New accounts default to keep-all retention, incremental sync, weekly full validation, and on-demand offline body storage.
- Upgraded accounts copy `sync_period_days` into local retention. Old keep-all accounts keep background-all offline body storage; other old accounts move to the fixed recent body window.
- Incremental sync skips full UID search when `UIDNEXT` is unchanged. When it changes, it searches only UIDs above the local high-water mark.
- Full validation still runs on schedule to detect old deletions, moves, and server anomalies.
- Clearing the offline body cache cancels running sync contexts for that account before deleting local body and attachment-cache rows.

## Frontend Behavior

The account form shows separate controls so local retention, daily speed, integrity checks, and offline body storage cost are explicit. Backup exports full `.eml` from the server and does not require offline body storage to be enabled.
