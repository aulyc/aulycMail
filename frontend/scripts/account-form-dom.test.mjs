// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  GetAccountFoldersForMapping: vi.fn(),
  GetAutoDetectedFolders: vi.fn(),
  GetIdentities: vi.fn(),
  AcceptCertificate: vi.fn(),
  GetAllAccountIdentities: vi.fn(),
  GetTrustedCertificates: vi.fn(),
  RemoveTrustedCertificate: vi.fn(),
  GetFolders: vi.fn(),
  SubscribeFolder: vi.fn(),
  UnsubscribeFolder: vi.fn(),
  SubscribeAllFolders: vi.fn(),
  ClearOfflineBodyCache: vi.fn(),
}))
const accountStore = vi.hoisted(() => ({
  testConnection: vi.fn(),
  testAccountConnection: vi.fn(),
}))
const toast = vi.hoisted(() => ({ add: vi.fn() }))

vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('$lib/stores/accounts.svelte', () => ({ accountStore }))
vi.mock('$lib/stores/toast', () => ({ addToast: toast.add }))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('@iconify/svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))
vi.mock('$lib/components/ui/select', async () => ({
  Root: (await import('./fixtures/SelectRootTestStub.svelte')).default,
  Trigger: (await import('./fixtures/SnippetTestStub.svelte')).default,
  Value: (await import('./fixtures/SnippetTestStub.svelte')).default,
  Content: (await import('./fixtures/SnippetTestStub.svelte')).default,
  Item: (await import('./fixtures/SelectItemTestStub.svelte')).default,
  Group: (await import('./fixtures/SnippetTestStub.svelte')).default,
  GroupHeading: (await import('./fixtures/SnippetTestStub.svelte')).default,
}))
vi.mock('$lib/components/ui/switch/Switch.svelte', async () => ({
  default: (await import('./fixtures/SwitchTestStub.svelte')).default,
}))
vi.mock('../src/lib/components/settings/CertificateDialog.svelte', async () => ({
  default: (await import('./fixtures/CertificateDialogTestStub.svelte')).default,
}))
vi.mock('../src/lib/components/settings/ConnectionTestDialog.svelte', async () => ({
  default: (await import('./fixtures/ConnectionTestDialogTestStub.svelte')).default,
}))
vi.mock('../src/lib/components/settings/account/AccountIdentityTab.svelte', async () => ({
  default: (await import('./fixtures/AccountIdentityTabTestStub.svelte')).default,
}))
vi.mock('$lib/components/ui/confirm-dialog/ConfirmDialog.svelte', async () => ({
  default: (await import('./fixtures/ConfirmDialogTestStub.svelte')).default,
}))

import AccountForm from '../src/lib/components/settings/AccountForm.svelte'

const mounted = []

async function flushAsync() {
  for (let index = 0; index < 8; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function renderForm(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(AccountForm, { target, props })
  mounted.push(instance)
  await flushAsync()
  return { instance, target, form: target.querySelector('form') }
}

function setInput(target, id, value) {
  const input = target.querySelector(`#${id}`)
  assert.ok(input, `missing input ${id}`)
  input.value = String(value)
  input.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }))
  return input
}

function findButton(target, text) {
  return [...target.querySelectorAll('button')].find((button) => button.textContent.includes(text))
}

function submit(form) {
  form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
}

function editAccount(overrides = {}) {
  return {
    id: 'account-1',
    name: 'person@example.test',
    email: 'person@example.test',
    username: 'person@example.test',
    imapHost: 'imap.example.test',
    imapPort: 993,
    imapSecurity: 'tls',
    smtpHost: 'smtp.example.test',
    smtpPort: 465,
    smtpSecurity: 'tls',
    noOutgoingServer: false,
    smtpUsername: '',
    replyForwardIdentityId: '',
    localRetentionDays: 0,
    syncStrategy: 'incremental',
    fullCheckIntervalDays: 7,
    bodyDownloadPolicy: 'on_demand',
    syncInterval: 30,
    syncAllFolders: false,
    syncFoldersEnabled: false,
    readReceiptRequestPolicy: 'never',
    color: '#336699',
    sentFolderPath: 'Sent',
    draftsFolderPath: 'Drafts',
    trashFolderPath: 'Trash',
    spamFolderPath: 'Spam',
    archiveFolderPath: 'Archive',
    allMailFolderPath: '',
    starredFolderPath: '',
    ...overrides,
  }
}

beforeEach(() => {
  document.body.innerHTML = ''
  for (const fn of Object.values(backend)) fn.mockReset().mockResolvedValue(undefined)
  backend.GetAllAccountIdentities.mockResolvedValue([])
  backend.GetIdentities.mockResolvedValue([{ id: 'identity-1', name: 'Synthetic Person', isDefault: true }])
  backend.GetAccountFoldersForMapping.mockResolvedValue([
    { id: 'folder-sent', path: 'Sent', type: 'sent' },
    { id: 'folder-archive', path: 'Archive', type: 'archive' },
  ])
  backend.GetAutoDetectedFolders.mockResolvedValue({ sent: 'Sent', archive: 'Archive' })
  backend.GetFolders.mockResolvedValue([
    { id: 'inbox', path: 'INBOX', type: 'inbox', subscribed: true },
    { id: 'archive', path: 'Archive', type: 'archive', subscribed: false },
  ])
  backend.GetTrustedCertificates.mockResolvedValue([{
    host: 'imap.example.test', subject: 'Synthetic certificate', fingerprint: '00112233445566778899', notAfter: '2027-08-01T00:00:00Z',
  }])
  backend.ClearOfflineBodyCache.mockResolvedValue({ bodiesCleared: 7 })
  accountStore.testConnection.mockReset().mockResolvedValue({ success: true })
  accountStore.testAccountConnection.mockReset().mockResolvedValue({ success: true })
  toast.add.mockReset()
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('validates a new manual account, mirrors SMTP host, tests it, and submits normalized config', async () => {
  const onSubmit = vi.fn().mockResolvedValue(undefined)
  const { target, form } = await renderForm({ onSubmit })
  submit(form)
  await flushAsync()
  assert.match(target.textContent, /account\.displayNameRequired/)
  assert.match(target.textContent, /account\.emailRequired/)
  assert.match(target.textContent, /account\.passwordRequired/)
  assert.match(target.textContent, /account\.imapHostRequired/)

  setInput(target, 'displayName', 'Synthetic Person')
  setInput(target, 'email', 'person@example.test')
  setInput(target, 'password', 'synthetic-password')
  setInput(target, 'imapHost', 'imap.example.test')
  await flushAsync()
  assert.equal(target.querySelector('#smtpHost').value, 'smtp.example.test')

  findButton(target, 'account.testConnection').click()
  await flushAsync()
  assert.equal(accountStore.testConnection.mock.calls.length, 1)
  assert.match(target.querySelector('[data-connection-dialog]').textContent, /account\.connectionSuccessful/)

  submit(form)
  await flushAsync()
  assert.equal(onSubmit.mock.calls.length, 1)
  const config = onSubmit.mock.calls[0][0]
  assert.equal(config.name, 'person@example.test')
  assert.equal(config.displayName, 'Synthetic Person')
  assert.equal(config.username, 'person@example.test')
  assert.equal(config.smtpHost, 'smtp.example.test')
  assert.equal(config.authType, 'password')
  assert.equal(config.syncInterval, 30)
})

test('handles certificate-required connection tests and accept/decline outcomes', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const { target } = await renderForm()
  setInput(target, 'displayName', 'Synthetic Person')
  setInput(target, 'email', 'person@example.test')
  setInput(target, 'password', 'synthetic-password')
  setInput(target, 'imapHost', 'imap.example.test')
  await flushAsync()

  const certificate = { host: 'imap.example.test', fingerprint: 'synthetic' }
  accountStore.testConnection
    .mockResolvedValueOnce({ success: false, certificateRequired: true, certificate })
    .mockResolvedValueOnce({ success: true })
  findButton(target, 'account.testConnection').click()
  await flushAsync()
  assert.equal(target.querySelector('[data-certificate-dialog]').dataset.host, 'imap.example.test')
  target.querySelector('[data-cert-action="once"]').click()
  await flushAsync()
  assert.deepEqual(backend.AcceptCertificate.mock.calls.at(-1), ['imap.example.test', certificate, false])
  assert.equal(accountStore.testConnection.mock.calls.length, 2)

  accountStore.testConnection.mockResolvedValueOnce({ success: false, certificateRequired: true, certificate })
  findButton(target, 'account.testConnection').click()
  await flushAsync()
  target.querySelector('[data-cert-action="decline"]').click()
  await flushAsync()
  assert.match(target.querySelector('[data-connection-dialog]').textContent, /account\.certificateDeclined/)

  accountStore.testConnection.mockRejectedValueOnce(new Error('network unavailable'))
  findButton(target, 'account.testConnection').click()
  await flushAsync()
  assert.match(target.querySelector('[data-connection-dialog]').textContent, /account\.connectionTestFailed/)
  assert.equal(error.mock.calls.length, 1)
})

test('loads and updates an existing account, folders, certificates, cache, and save errors', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const onSubmit = vi.fn().mockRejectedValueOnce(new Error('save unavailable')).mockResolvedValueOnce(undefined)
  const onCancel = vi.fn()
  const { target, form } = await renderForm({ editAccount: editAccount(), onSubmit, onCancel })
  assert.equal(target.querySelector('#displayName').value, 'Synthetic Person')
  assert.equal(target.querySelector('#email').disabled, true)
  assert.ok(target.querySelector('[data-identity-tab]'))
  assert.deepEqual(backend.GetIdentities.mock.calls.at(-1), ['account-1'])

  findButton(target, 'account.testConnection').click()
  await flushAsync()
  assert.deepEqual(accountStore.testAccountConnection.mock.calls.at(-1), ['account-1'])

  findButton(target, 'account.folderMapping').click()
  await flushAsync()
  assert.deepEqual(backend.GetAccountFoldersForMapping.mock.calls.at(-1), ['account-1'])
  assert.deepEqual(backend.GetAutoDetectedFolders.mock.calls.at(-1), ['account-1'])
  assert.match(target.textContent, /account\.detected/)

  findButton(target, 'account.folderSync').click()
  await flushAsync()
  assert.deepEqual(backend.GetFolders.mock.calls.at(-1), ['account-1'])

  findButton(target, 'account.trustedCertificates').click()
  await flushAsync()
  assert.match(target.textContent, /Synthetic certificate/)
  assert.match(target.textContent, /00:11:22:33:44:55:66:77/)
  findButton(target, 'common.remove').click()
  await flushAsync()
  target.querySelector('[data-confirm-dialog] [data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.deepEqual(backend.RemoveTrustedCertificate.mock.calls.at(-1), ['00112233445566778899'])

  findButton(target, 'account.clearOfflineBodyCache').click()
  await flushAsync()
  const cacheDialog = [...target.querySelectorAll('[data-confirm-dialog]')].at(-1)
  cacheDialog.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.deepEqual(backend.ClearOfflineBodyCache.mock.calls.at(-1), ['account-1'])
  assert.match(toast.add.mock.calls.at(-1)[0].message, /"count":7/)

  submit(form)
  await flushAsync()
  assert.match(target.querySelector('[data-connection-dialog]').textContent, /account\.saveFailed/)
  submit(form)
  await flushAsync()
  assert.equal(onSubmit.mock.calls.length, 2)

  findButton(target, 'common.cancel').click()
  assert.equal(onCancel.mock.calls.length, 1)
  assert.ok(error.mock.calls.some(([message]) => message === 'Account save failed:'))
})

test('supports receive-only manual setup and keeps a manually edited SMTP host sticky', async () => {
  backend.GetAllAccountIdentities.mockResolvedValue([
    { account: { id: 'sendable', name: 'Sendable', color: '#123456', noOutgoingServer: false }, identities: [{ id: 'identity-send', name: 'Sender', email: 'sender@example.test' }] },
    { account: { id: 'receive-only', name: 'Receive only', noOutgoingServer: true }, identities: [{ id: 'identity-hidden', name: 'Hidden', email: 'hidden@example.test' }] },
  ])
  const onSubmit = vi.fn().mockResolvedValue(undefined)
  const { target, form } = await renderForm({ onSubmit, createdInDialog: true })
  setInput(target, 'displayName', 'Receive Only')
  setInput(target, 'email', 'receive@example.test')
  setInput(target, 'password', 'synthetic-password')
  setInput(target, 'imapHost', 'mail.example.test')
  setInput(target, 'smtpHost', 'outgoing.custom.test')
  setInput(target, 'imapHost', 'imap.changed.test')
  await flushAsync()
  assert.equal(target.querySelector('#smtpHost').value, 'outgoing.custom.test')

  const noOutgoing = target.querySelector('input[type="checkbox"]')
  noOutgoing.checked = true
  noOutgoing.dispatchEvent(new Event('change', { bubbles: true }))
  await flushAsync()
  assert.equal(target.querySelector('#smtpHost'), null)
  assert.match(target.textContent, /Sendable/)
  assert.doesNotMatch(target.textContent, /Receive only/)

  submit(form)
  await flushAsync()
  assert.equal(onSubmit.mock.calls.length, 1)
  assert.equal(onSubmit.mock.calls[0][0].noOutgoingServer, true)
  assert.equal(onSubmit.mock.calls[0][0].smtpUsername, '')
  assert.match(findButton(target, 'common.close').textContent, /common\.close/)
})

test('keeps folder, certificate, cache, and connection failures observable and recoverable', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.GetIdentities.mockRejectedValue(new Error('identities unavailable'))
  backend.GetAllAccountIdentities.mockRejectedValue(new Error('identity groups unavailable'))
  backend.GetAccountFoldersForMapping.mockRejectedValue(new Error('mapping unavailable'))
  backend.GetFolders
    .mockRejectedValueOnce(new Error('folders unavailable'))
    .mockResolvedValue([
      { id: 'inbox', path: 'INBOX', type: 'inbox', subscribed: true },
      { id: 'archive', path: 'Archive', type: 'archive', subscribed: false },
    ])
  backend.GetTrustedCertificates.mockRejectedValue(new Error('certificates unavailable'))
  backend.ClearOfflineBodyCache.mockRejectedValue(new Error('cache unavailable'))
  backend.SubscribeAllFolders.mockRejectedValue(new Error('subscribe all unavailable'))
  backend.SubscribeFolder.mockRejectedValue(new Error('subscribe unavailable'))

  const certificate = { host: 'imap.example.test', fingerprint: 'permanent-cert' }
  accountStore.testAccountConnection
    .mockResolvedValueOnce({ success: false, certificateRequired: true, certificate })
    .mockResolvedValueOnce({ success: false, error: '' })

  const { target } = await renderForm({ editAccount: editAccount() })
  await flushAsync()
  assert.ok(error.mock.calls.some(([message]) => message === 'Failed to load display name:'))
  assert.ok(error.mock.calls.some(([message]) => message === 'Failed to load identity groups for Reply/Forward-with picker:'))

  setInput(target, 'displayName', 'Fallback Person')

  findButton(target, 'account.testConnection').click()
  await flushAsync()
  target.querySelector('[data-cert-action="permanent"]').click()
  await flushAsync()
  assert.deepEqual(backend.AcceptCertificate.mock.calls.at(-1), ['imap.example.test', certificate, true])
  assert.match(target.querySelector('[data-connection-dialog]').textContent, /account\.connectionFailed/)

  findButton(target, 'account.folderMapping').click()
  await flushAsync()
  assert.match(target.textContent, /account\.noFoldersAvailable/)

  findButton(target, 'account.folderSync').click()
  await flushAsync()
  const switchesAfterFolderOpen = target.querySelectorAll('input[type="checkbox"]')
  assert.equal(switchesAfterFolderOpen.length >= 2, true)
  const enableFolderSync = switchesAfterFolderOpen[1]
  enableFolderSync.checked = true
  enableFolderSync.dispatchEvent(new Event('change', { bubbles: true }))
  await flushAsync()
  const switchesAfterEnable = target.querySelectorAll('input[type="checkbox"]')
  const syncAll = switchesAfterEnable[2]
  syncAll.checked = true
  syncAll.dispatchEvent(new Event('change', { bubbles: true }))
  await flushAsync()
  assert.equal(backend.SubscribeAllFolders.mock.calls.length, 1)
  syncAll.checked = false
  syncAll.dispatchEvent(new Event('change', { bubbles: true }))
  await flushAsync()

  const archiveSubscription = [...target.querySelectorAll('input[type="checkbox"]')].find((input) => input.closest('label')?.textContent.includes('Archive'))
  assert.ok(archiveSubscription)
  archiveSubscription.checked = true
  archiveSubscription.dispatchEvent(new Event('change', { bubbles: true }))
  await flushAsync()
  assert.deepEqual(backend.SubscribeFolder.mock.calls.at(-1), ['account-1', 'archive'])

  findButton(target, 'account.trustedCertificates').click()
  await flushAsync()
  assert.match(target.textContent, /account\.noTrustedCerts/)

  findButton(target, 'account.clearOfflineBodyCache').click()
  await flushAsync()
  const cacheDialog = [...target.querySelectorAll('[data-confirm-dialog]')].at(-1)
  cacheDialog.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.equal(toast.add.mock.calls.at(-1)[0].type, 'error')
  assert.match(toast.add.mock.calls.at(-1)[0].message, /account\.offlineBodyCacheClearFailed/)

  for (const expected of [
    'Failed to load folders for mapping:',
    'Failed to load folders for sync:',
    'Failed to subscribe to all folders:',
    'Failed to update folder subscription:',
    'Failed to load trusted certificates:',
    'Failed to clear offline body cache:',
  ]) {
    assert.ok(error.mock.calls.some(([message]) => message === expected), `missing logged error ${expected}`)
  }
})
