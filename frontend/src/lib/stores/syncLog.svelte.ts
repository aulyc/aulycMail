// In-memory sync/connection activity log. Listens to the same backend events
// the account store does (folder:synced / folder:syncError / network:*) and
// keeps a rolling, session-scoped list of successes and failures. Surfaced via
// the rail's log icon → SyncLogDialog. Replaces the inline per-account sync
// error message that used to sit under the account name.

// @ts-ignore - wailsjs runtime
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { accountStore } from './accounts.svelte'

export type SyncLogLevel = 'success' | 'error' | 'info'

export interface SyncLogEntry {
  id: number
  time: Date
  level: SyncLogLevel
  /** Resolved "account / folder" label (best-effort at event time). */
  target: string
  /** Error detail (only for errors). */
  detail?: string
}

const MAX_ENTRIES = 300

class SyncLog {
  entries = $state<SyncLogEntry[]>([])
  /** Number of error entries added since the log was last opened (rail badge). */
  unseenErrors = $state(0)
  private seq = 0
  private started = false

  /** Begin listening for backend sync events. Idempotent. */
  start() {
    if (this.started) return
    this.started = true

    EventsOn('folder:synced', (data: { accountId: string; folderId: string }) => {
      this.add('success', this.label(data.accountId, data.folderId))
    })
    EventsOn('folder:syncError', (data: { accountId: string; folderId: string; error: string }) => {
      this.add('error', this.label(data.accountId, data.folderId), data.error)
    })
  }

  private add(level: SyncLogLevel, target: string, detail?: string) {
    this.seq += 1
    const entry: SyncLogEntry = { id: this.seq, time: new Date(), level, target, detail }
    this.entries = [entry, ...this.entries].slice(0, MAX_ENTRIES)
    if (level === 'error') this.unseenErrors += 1
  }

  /** Resolve "account-email / folder-name" from IDs via the account store. */
  private label(accountId: string, folderId: string): string {
    const acct = accountStore.accounts.find((a) => a.account.id === accountId)
    const acctLabel = acct?.account.email || accountId
    let folderLabel = folderId
    if (acct) {
      for (const tree of acct.folders) {
        if (tree.folder?.id === folderId) { folderLabel = tree.folder.name; break }
        const child = (tree.children || []).find((c: any) => c.folder?.id === folderId)
        if (child?.folder) { folderLabel = child.folder.name; break }
      }
    }
    return `${acctLabel} / ${folderLabel}`
  }

  /** Called when the log dialog opens — clears the rail error badge. */
  markSeen() {
    this.unseenErrors = 0
  }

  clear() {
    this.entries = []
    this.unseenErrors = 0
  }
}

export const syncLog = new SyncLog()
