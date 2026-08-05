import assert from 'node:assert/strict'
import { render } from 'svelte/server'
import { test, vi } from 'vitest'

const accounts = vi.hoisted(() => ({ accounts: [], loading: false }))

function translator(key, options) {
  return options?.values ? `${key}:${JSON.stringify(options.values)}` : key
}

vi.mock('$lib/i18n', () => ({
  _: { subscribe(run) { run(translator); return () => {} } },
  supportedLocales: [
    { code: 'en', name: 'English' },
    { code: 'zh-CN', name: '简体中文' },
  ],
}))
vi.mock('svelte-i18n', () => ({
  _: { subscribe(run) { run(translator); return () => {} } },
}))
vi.mock('$lib/stores/accounts.svelte', () => ({ accountStore: accounts }))

import ComposerAttachmentList from '../src/lib/components/composer/ComposerAttachmentList.svelte'
import ActivityLogFilters from '../src/lib/components/settings/activity/ActivityLogFilters.svelte'
import ActivityLogItem from '../src/lib/components/settings/activity/ActivityLogItem.svelte'
import ActivityLogList from '../src/lib/components/settings/activity/ActivityLogList.svelte'
import BackupRunPanel from '../src/lib/components/settings/backup/BackupRunPanel.svelte'
import BackupScopePicker from '../src/lib/components/settings/backup/BackupScopePicker.svelte'
import AppearanceSettingsPage from '../src/lib/components/settings/pages/AppearanceSettingsPage.svelte'
import GeneralSettingsPage from '../src/lib/components/settings/pages/GeneralSettingsPage.svelte'
import ContactFieldsForm, { slotConstraintsFor } from '../src/lib/contacts/components/fields/ContactFieldsForm.svelte'
import EmailsField from '../src/lib/contacts/components/fields/EmailsField.svelte'

function settingsDraft(overrides = {}) {
  return {
    language: 'en',
    runBackground: false,
    menuBarIcon: false,
    autostart: false,
    enhancedKeyboardNavigation: true,
    developerMode: false,
    themeMode: 'pop-dark',
    darkMailContent: true,
    accentBarUnread: true,
    messageListDensity: 'compact',
    ...overrides,
  }
}

test('general and appearance pages render complete draft-backed setting controls', () => {
  const general = render(GeneralSettingsPage, { props: { draft: settingsDraft() } }).body
  assert.match(general, /settingsDescriptions.general/)
  assert.match(general, /settingsGeneral.language/)
  assert.match(general, /English/)
  assert.match(general, /settingsGeneral.runInBackground/)
  assert.match(general, /settingsGeneral.enhancedKeyboardNavigation/)
  assert.match(general, /settingsGeneral.developerMode/)

  const appearance = render(AppearanceSettingsPage, { props: { draft: settingsDraft() } }).body
  assert.match(appearance, /settingsDescriptions.appearance/)
  assert.match(appearance, /settingsGeneral.themeDark/)
  assert.match(appearance, /settingsGeneral.darkMailContent/)
  assert.match(appearance, /settingsGeneral.densityCompact/)

  const unknown = render(AppearanceSettingsPage, {
    props: { draft: settingsDraft({ themeMode: 'custom-theme', messageListDensity: 'custom-density' }) },
  }).body
  assert.match(unknown, /custom-theme/)
  assert.match(unknown, /custom-density/)
})

test('activity log item renders compact and expanded backup evidence', () => {
  const base = {
    id: 'log-1',
    type: 'backup',
    status: 'partial',
    createdAt: '2026-08-01T03:04:05Z',
    title: 'Backup',
    summary: 'summary',
    detail: 'One message could not be exported.',
    payload: {
      mode: 'incremental', total: 6, backedUp: 4, added: 3,
      skipped: 1, missing: 1, unavailable: 0, failed: 1, directory: '/backup',
    },
  }
  const compact = render(ActivityLogItem, { props: { log: base } }).body
  assert.match(compact, /aria-expanded="false"/)
  assert.match(compact, /activityLog.status.partial/)
  assert.doesNotMatch(compact, /One message could not be exported/)

  const expanded = render(ActivityLogItem, { props: { log: base, expanded: true } }).body
  assert.match(expanded, /aria-expanded="true"/)
  assert.match(expanded, /settingsBackup.checked/)
  assert.match(expanded, /settingsBackup.newlyBackedUp/)
  assert.match(expanded, /\/backup/)
  assert.match(expanded, /One message could not be exported/)
})

test('activity list covers loading, failure, empty, populated, and pagination states', () => {
  const base = {
    entries: [], total: 0, loading: true, loadFailed: false, type: '', problemOnly: false, date: '',
    get hasMore() { return this.entries.length < this.total },
    loadMore: vi.fn(), setFilter: vi.fn(), refresh: vi.fn(), clearCurrent: vi.fn(), clearAll: vi.fn(),
  }
  assert.match(render(ActivityLogList, { props: { store: base } }).body, /flex justify-center py-12/)
  base.loading = false
  base.loadFailed = true
  assert.match(render(ActivityLogList, { props: { store: base } }).body, /activityLog.loadFailed/)
  base.loadFailed = false
  assert.match(render(ActivityLogList, { props: { store: base } }).body, /activityLog.empty/)
  base.entries = [{
    id: 'sync-log', type: 'sync', status: 'success', createdAt: '2026-08-01T03:04:05Z',
    title: 'Inbox', summary: 'Synced', payload: { added: 2 },
  }]
  base.total = 3
  const populated = render(ActivityLogList, { props: { store: base } }).body
  assert.match(populated, /activityLog.syncSummary/)
  assert.match(populated, /activityLog.loadMore/)

  base.date = '2026-08-01'
  const filters = render(ActivityLogFilters, { props: { store: base } }).body
  assert.match(filters, /data-settings-initial-selection="true"/)
  assert.match(filters, /activityLog.allDates/)
  assert.match(filters, /activityLog.filters.failed/)
})

test('backup run and scope controls render disabled, running, selected, and all-account states', () => {
  let body = render(BackupRunPanel, {
    props: { store: { loading: false, running: false }, canStart: false, saveBeforeStart: false, onStart: vi.fn() },
  }).body
  assert.match(body, /disabled/)
  assert.match(body, /settingsBackup.startBackup/)
  body = render(BackupRunPanel, {
    props: { store: { loading: false, running: true }, canStart: true, saveBeforeStart: true, onStart: vi.fn() },
  }).body
  assert.match(body, /settingsBackup.viewProgress/)
  body = render(BackupRunPanel, {
    props: { store: { loading: false, running: false }, canStart: true, saveBeforeStart: true, onStart: vi.fn() },
  }).body
  assert.match(body, /settingsBackup.saveAndStart/)

  accounts.accounts = []
  assert.match(render(BackupScopePicker, { props: { scope: 'all', selectedAccountIds: [] } }).body, /settingsBackup.noAccounts/)
  accounts.accounts = [
    { account: { id: 'a', email: 'a@example.com' } },
    { account: { id: 'b', email: 'b@example.com' } },
    { account: { id: 'shared', email: 'shared@example.com', sharedMailboxParentId: 'a' } },
  ]
  body = render(BackupScopePicker, { props: { scope: 'all', selectedAccountIds: ['a', 'b'] } }).body
  assert.match(body, /settingsBackup.scopeAll/)
  assert.doesNotMatch(body, /shared@example.com/)
  body = render(BackupScopePicker, { props: { scope: 'selected', selectedAccountIds: ['a'] } }).body
  assert.match(body, /a@example.com/)
})

test('composer attachments render MIME icons, sizes, and remove controls while empty input renders nothing', () => {
  const empty = render(ComposerAttachmentList, { props: { attachments: [], onRemove: vi.fn() } }).body
  assert.doesNotMatch(empty, /data-keyboard-action-context/)
  const body = render(ComposerAttachmentList, {
    props: {
      attachments: [
        { filename: 'photo.png', contentType: 'image/png', size: 1024, data: '' },
        { filename: 'report.pdf', contentType: 'application/pdf', size: 2048, data: '' },
      ],
      onRemove: vi.fn(),
    },
  }).body
  assert.match(body, /photo.png/)
  assert.match(body, /report.pdf/)
  assert.match(body, /1\.0 KB/)
  assert.equal((body.match(/attachment.removeAttachment/g) || []).length, 2)
})

test('contact field components render validation, constraints, disabled state, and minimal slot contract', () => {
  assert.deepEqual(slotConstraintsFor('local'), {
    emails: { kind: 'none' }, phones: { kind: 'none' }, addresses: { kind: 'none' }, urls: { kind: 'none' }, impps: { kind: 'none' },
  })
  const emails = [{ email: 'bad', type: '', isPrimary: true }]
  let body = render(EmailsField, {
    props: { emails, errors: { 'email-0': 'Invalid email' }, constraint: { kind: 'max', max: 1, reason: 'Only one' } },
  }).body
  assert.match(body, /value="bad"/)
  assert.match(body, /aria-invalid="true"/)
  assert.match(body, /Invalid email/)
  assert.match(body, /title="Only one"/)

  body = render(ContactFieldsForm, {
    props: {
      nameInput: 'Alice', noteInput: 'Important', emails,
      errors: { name: 'Name required', 'email-0': 'Invalid email' }, saving: true,
    },
  }).body
  assert.match(body, /value="Alice"/)
  assert.match(body, /Name required/)
  assert.match(body, /Important/)
  assert.match(body, /disabled/)
})
