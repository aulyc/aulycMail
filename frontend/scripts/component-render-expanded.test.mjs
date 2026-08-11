import assert from 'node:assert/strict'
import { createRawSnippet } from 'svelte'
import { render } from 'svelte/server'
import { beforeEach, test, vi } from 'vitest'

const layout = vi.hoisted(() => ({ mode: 'full', view: 'default', hide: vi.fn(), show: vi.fn() }))
const status = vi.hoisted(() => ({
  contact: { active: false, scanned: 0, total: 0, percentage: null },
  account: {
    isAnySyncing: false,
    isOnline: true,
    lastCompleteSyncTime: null,
    accounts: [],
    getSyncProgress: vi.fn(),
  },
  toasts: [],
}))

vi.mock('$lib/stores/layout.svelte', () => ({
  getLayoutMode: () => layout.mode,
  getResponsiveView: () => layout.view,
  hideSidebar: layout.hide,
  showSidebar: layout.show,
}))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('svelte-i18n', () => ({
  _: {
    subscribe(run) {
      run((key) => key)
      return () => {}
    },
  },
}))
vi.mock('$lib/stores/accounts.svelte', () => ({ accountStore: status.account }))
vi.mock('$lib/stores/settings.svelte', () => ({ getCurrentDateFnsLocale: () => undefined }))
vi.mock('$lib/stores/toast', () => ({
  toasts: {
    subscribe(run) {
      run(status.toasts)
      return () => {}
    },
  },
}))
vi.mock('$contacts/stores/contactRefresh.svelte', () => ({
  contactRefresh: status.contact,
  initContactRefreshEvents: vi.fn(),
}))

import BackupViewerMessageDetail from '../src/lib/components/backup/BackupViewerMessageDetail.svelte'
import ListHeader from '../src/lib/components/kit/ListHeader.svelte'
import ListRow from '../src/lib/components/kit/ListRow.svelte'
import PaneLayout from '../src/lib/components/kit/PaneLayout.svelte'
import ResponsiveSidebarToggle from '../src/lib/components/kit/ResponsiveSidebarToggle.svelte'
import SettingsPageHeader from '../src/lib/components/settings/shared/SettingsPageHeader.svelte'
import SettingsRow from '../src/lib/components/settings/shared/SettingsRow.svelte'
import SettingsSection from '../src/lib/components/settings/shared/SettingsSection.svelte'
import StatusBar from '../src/lib/components/status/StatusBar.svelte'
import ModalFrame from '../src/lib/components/ui/ModalFrame.svelte'

function snippet(html) {
  return createRawSnippet(() => ({ render: () => html }))
}

beforeEach(() => {
  layout.mode = 'full'
  layout.view = 'default'
  status.contact.active = false
  status.contact.scanned = 0
  status.contact.total = 0
  status.contact.percentage = null
  status.account.isAnySyncing = false
  status.account.isOnline = true
  status.account.lastCompleteSyncTime = null
  status.account.accounts = []
  status.account.getSyncProgress.mockReset()
  status.toasts.splice(0)
})

test('settings shared components render optional metadata, framing, help, and content snippets', () => {
  const header = render(SettingsPageHeader, {
    props: { description: 'General preferences', action: snippet('<button>Reset</button>') },
  }).body
  assert.match(header, /General preferences/)
  assert.match(header, /<button>Reset<\/button>/)
  assert.match(header, /data-settings-page-header/)
  assert.match(header, /data-settings-page-header-action/)
  assert.match(header, /\bpr-10\b/)

  const framed = render(SettingsSection, {
    props: {
      title: 'Appearance',
      description: 'Colors and density',
      children: snippet('<span>section-body</span>'),
    },
  }).body
  assert.match(framed, /Appearance/)
  assert.match(framed, /Colors and density/)
  assert.match(framed, /rounded-xl border border-border/)
  assert.match(framed, /section-body/)

  const unframed = render(SettingsSection, {
    props: { framed: false, bottomBorder: false, children: snippet('<span>plain</span>') },
  }).body
  assert.match(unframed, /border-t border-border\/75/)
  assert.doesNotMatch(unframed, /(?:^|\s)border-b(?:\s|")/)

  const row = render(SettingsRow, {
    props: {
      label: 'Theme',
      description: 'Choose the application theme',
      help: 'Follows your system when set to automatic.',
      helpLabel: 'Theme help',
      border: false,
      children: snippet('<select><option>System</option></select>'),
    },
  }).body
  assert.match(row, /data-keyboard-action-context="Theme"/)
  assert.match(row, /Choose the application theme/)
  assert.match(row, /aria-label="Theme help"/)
  assert.match(row, /<select><option>System<\/option><\/select>/)
  assert.doesNotMatch(row, /last:border-b-0/)
})

test('kit rows, headers, responsive toggles, and pane scrims render their state', () => {
  const selected = render(ListRow, {
    props: { selected: true, density: 'micro', children: snippet('<span>Alice</span>') },
  }).body
  assert.match(selected, /aria-selected="true"/)
  assert.match(selected, /px-3 py-2 gap-2/)
  assert.match(selected, /bg-primary\/20/)

  const normal = render(ListRow, {
    props: { density: 'large', children: snippet('<span>Bob</span>') },
  }).body
  assert.match(normal, /aria-selected="false"/)
  assert.match(normal, /px-6 py-5 gap-5/)
  assert.match(normal, /hover:bg-muted\/50/)

  const header = render(ListHeader, {
    props: {
      label: 'Contacts', count: 12, searchMode: true,
      search: snippet('<input aria-label="Search contacts">'),
      actions: snippet('<button>Add</button>'),
    },
  }).body
  assert.match(header, /Contacts/)
  assert.match(header, />12<\/span>/)
  assert.match(header, /aria-label="Search contacts"/)
  assert.match(header, /<button>Add<\/button>/)

  assert.doesNotMatch(render(ResponsiveSidebarToggle).body, /<button/)
  layout.mode = 'narrow'
  const toggle = render(ResponsiveSidebarToggle).body
  assert.match(toggle, /aria-label="aria.toggleSidebar"/)

  layout.view = 'sidebar'
  const pane = render(PaneLayout, { props: { children: snippet('<main>content</main>') } }).body
  assert.match(pane, /<main>content<\/main>/)
  assert.match(pane, /responsive-scrim-visible/)
})

test('modal frame renders only while open with the expected accessible contract', () => {
  assert.doesNotMatch(render(ModalFrame, { props: { panelClass: 'panel' } }).body, /role="dialog"/)
  const open = render(ModalFrame, {
    props: {
      open: true,
      panelClass: 'panel custom',
      backdropClass: 'backdrop',
      containerClass: 'container',
      labelledBy: 'dialog-title',
      children: snippet('<h2 id="dialog-title">Confirm</h2>'),
    },
  }).body
  assert.match(open, /class="fixed inset-0 container backdrop"/)
  assert.match(open, /role="dialog"/)
  assert.match(open, /aria-modal="true"/)
  assert.match(open, /aria-labelledby="dialog-title"/)
  assert.match(open, /<h2 id="dialog-title">Confirm<\/h2>/)
})

test('backup message detail covers loading, empty, metadata, attachments, HTML, text, and empty bodies', () => {
  const common = {
    detail: null,
    loadingDetail: false,
    detailHeaderTitle: '',
    attachmentsExpanded: true,
    savingAttachmentIndexes: new Set(),
    darkFilterStyle: '--backup-viewer-content-filter:none',
    darkFilterEnabled: false,
    onSaveAttachment: vi.fn(),
    formatDate: (value) => `DATE:${value}`,
  }
  const loading = render(BackupViewerMessageDetail, { props: { ...common, loadingDetail: true } }).body
  assert.match(loading, /backupViewer.loading/)
  const empty = render(BackupViewerMessageDetail, { props: common }).body
  assert.match(empty, /backupViewer.selectMessage/)

  const detail = {
    from: ['alice@example.com'], to: ['bob@example.com'], cc: ['cc@example.com'], bcc: ['bcc@example.com'],
    date: '2026-08-01', accountEmail: 'mail@example.com', folderPath: 'Inbox', size: 2048,
    attachments: [
      { index: 4, filename: 'report.pdf', contentType: 'application/pdf', size: 1024 },
      { filename: 'photo.png', contentType: 'image/png', size: 512 },
    ],
    hasHTML: true,
    bodyHTML: '<p>sanitized body</p>',
    bodyText: '',
  }
  const rendered = render(BackupViewerMessageDetail, {
    props: { ...common, detail, detailHeaderTitle: 'Quarterly report', savingAttachmentIndexes: new Set([4]), darkFilterEnabled: true },
  }).body
  assert.match(rendered, /Quarterly report/)
  assert.match(rendered, /alice@example.com/)
  assert.match(rendered, /cc@example.com/)
  assert.match(rendered, /DATE:2026-08-01/)
  assert.match(rendered, /mail@example.com \/ Inbox/)
  assert.match(rendered, /report.pdf/)
  assert.match(rendered, /disabled/)
  assert.match(rendered, /backup-viewer-dark-filter/)
  assert.match(rendered, /<p>sanitized body<\/p>/)

  const text = render(BackupViewerMessageDetail, {
    props: { ...common, detail: { ...detail, attachments: [], hasHTML: false, bodyHTML: '', bodyText: 'plain body' } },
  }).body
  assert.match(text, /<pre[^>]*>plain body<\/pre>/)
  const noBody = render(BackupViewerMessageDetail, {
    props: { ...common, detail: { ...detail, attachments: [], hasHTML: false, bodyHTML: '', bodyText: '' } },
  }).body
  assert.match(noBody, /backupViewer.noBody/)
})

test('status bar renders contact progress, account sync phases, offline state, and latest toast', () => {
  status.contact.active = true
  status.contact.scanned = 12
  status.contact.total = 10
  status.contact.percentage = 100
  let body = render(StatusBar).body
  assert.match(body, /contacts.status.refreshingProgress/)
  assert.match(body, /"scanned":10/)
  assert.match(body, /width: 100%/)

  status.contact.active = false
  status.account.isAnySyncing = true
  status.account.accounts = [{ account: { id: 'a', name: 'Primary' }, syncing: true }]
  status.account.getSyncProgress.mockReturnValue({ phase: 'headers', percentage: 65 })
  body = render(StatusBar).body
  assert.match(body, /Primary/)
  assert.match(body, /sidebar.fetchingHeaders/)
  assert.match(body, /width: 65%/)

  status.account.isAnySyncing = false
  status.account.isOnline = false
  status.toasts.push({ id: 'toast', type: 'error', message: 'Connection failed' })
  body = render(StatusBar).body
  assert.match(body, /sidebar.offline/)
  assert.match(body, /role="alert"/)
  assert.match(body, /Connection failed/)
})

test('status bar covers every remaining progress fallback, completion age, and toast presentation', () => {
  status.contact.active = true
  status.contact.scanned = 4
  status.contact.total = 0
  status.contact.percentage = null
  let body = render(StatusBar).body
  assert.match(body, /contacts.status.refreshing/)
  assert.doesNotMatch(body, /style="width:/)

  status.contact.active = false
  status.account.isAnySyncing = true
  status.account.accounts = [{ account: { id: 'a', name: 'Primary' }, syncing: true }]
  for (const [progress, label] of [
    [{ phase: 'folders', percentage: 10 }, 'sidebar.syncingFolders'],
    [{ phase: 'messages', percentage: 20 }, 'sidebar.fetchingMessageList'],
    [{ phase: 'body', percentage: 30 }, 'sidebar.syncingContent'],
    [null, 'sidebar.syncing'],
  ]) {
    status.account.getSyncProgress.mockReturnValue(progress)
    body = render(StatusBar).body
    assert.match(body, new RegExp(label))
  }

  status.account.accounts = [{ account: { id: 'idle', name: 'Idle' }, syncing: false }]
  body = render(StatusBar).body
  assert.match(body, /sidebar.syncing/)
  assert.doesNotMatch(body, />Idle</)

  status.account.isAnySyncing = false
  status.account.isOnline = true
  status.account.lastCompleteSyncTime = null
  body = render(StatusBar).body
  assert.match(body, /sidebar.notSynced/)

  status.account.lastCompleteSyncTime = new Date(Date.now() - 60_000)
  body = render(StatusBar).body
  assert.match(body, /sidebar.synced/)

  for (const type of ['success', 'info', 'warning']) {
    status.toasts.splice(0, status.toasts.length, { id: type, type, message: `${type} message` })
    body = render(StatusBar).body
    assert.match(body, new RegExp(`${type} message`))
    assert.match(body, /role="status"/)
  }
})
