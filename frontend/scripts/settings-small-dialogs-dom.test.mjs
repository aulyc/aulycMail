// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const accounts = vi.hoisted(() => ({
  addAccount: vi.fn(),
  updateAccount: vi.fn(),
  removeAccount: vi.fn(),
}))
const toast = vi.hoisted(() => ({ add: vi.fn() }))
const guards = vi.hoisted(() => ({ open: vi.fn(), close: vi.fn() }))

vi.mock('$lib/stores/accounts.svelte', () => ({ accountStore: accounts }))
vi.mock('$lib/stores/toast', () => ({ addToast: toast.add }))
vi.mock('$lib/stores/dialogGuard', () => ({
  dialogGuardOpen: guards.open,
  dialogGuardClose: guards.close,
}))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('@iconify/svelte', async () => ({
  default: (await import('./fixtures/StaticStub.svelte')).default,
}))
vi.mock('$lib/components/ui/dialog', async () => {
  const root = (await import('./fixtures/DialogRootTestStub.svelte')).default
  const content = (await import('./fixtures/DialogContentTestStub.svelte')).default
  const snippet = (await import('./fixtures/SnippetTestStub.svelte')).default
  return { Root: root, Content: content, Header: snippet, Title: snippet }
})
vi.mock('$lib/components/ui/confirm-dialog/ConfirmDialog.svelte', async () => ({
  default: (await import('./fixtures/ConfirmDialogTestStub.svelte')).default,
}))
vi.mock('../src/lib/components/settings/AccountForm.svelte', async () => ({
  default: (await import('./fixtures/AccountFormDialogTestStub.svelte')).default,
}))
vi.mock('bits-ui', async () => {
  const root = (await import('./fixtures/DialogRootTestStub.svelte')).default
  const content = (await import('./fixtures/DialogContentTestStub.svelte')).default
  const snippet = (await import('./fixtures/SnippetTestStub.svelte')).default
  const overlay = (await import('./fixtures/StaticStub.svelte')).default
  return { Dialog: { Root: root, Portal: snippet, Overlay: overlay, Content: content } }
})

import AccountDialog from '../src/lib/components/settings/AccountDialog.svelte'
import CertificateDialog from '../src/lib/components/settings/CertificateDialog.svelte'
import ConnectionTestDialog from '../src/lib/components/settings/ConnectionTestDialog.svelte'
import DeleteAccountDialog from '../src/lib/components/settings/DeleteAccountDialog.svelte'

const mounted = []

async function flushAsync() {
  for (let index = 0; index < 7; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function render(component, props) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(component, { target, props })
  mounted.push(instance)
  await flushAsync()
  return target
}

function buttonWithText(target, text) {
  const button = [...target.querySelectorAll('button')].find((item) => item.textContent.includes(text))
  assert.ok(button, `missing button containing ${text}`)
  return button
}

beforeEach(() => {
  document.body.innerHTML = ''
  accounts.addAccount.mockReset().mockResolvedValue({
    id: 'created', name: 'Synthetic Account', email: 'synthetic@example.test',
  })
  accounts.updateAccount.mockReset().mockResolvedValue({
    id: 'created', name: 'Updated Account', email: 'synthetic@example.test',
  })
  accounts.removeAccount.mockReset().mockResolvedValue(undefined)
  toast.add.mockReset()
  guards.open.mockReset()
  guards.close.mockReset()
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('account dialog creates then updates an account in the same guarded flow', async () => {
  const onSuccess = vi.fn()
  const onClose = vi.fn()
  const target = await render(AccountDialog, { open: true, onSuccess, onClose })
  assert.match(target.textContent, /account\.addTitle/)
  assert.equal(guards.open.mock.calls.length, 1)

  target.querySelector('[data-account-form-submit]').click()
  await flushAsync()
  assert.deepEqual(accounts.addAccount.mock.calls.at(-1), [{
    name: 'Synthetic Account', email: 'synthetic@example.test', provider: 'imap',
  }])
  assert.match(target.textContent, /account\.editTitle/)
  assert.equal(target.querySelector('[data-account-form-dialog]').dataset.createdInDialog, 'true')
  assert.equal(toast.add.mock.calls.at(-1)[0].message, 'toast.accountCreated')

  target.querySelector('[data-account-form-submit]').click()
  await flushAsync()
  assert.deepEqual(accounts.updateAccount.mock.calls.at(-1), [
    'created',
    { name: 'Synthetic Account', email: 'synthetic@example.test', provider: 'imap' },
  ])
  assert.equal(target.querySelector('[role="dialog"]'), null)
  assert.equal(onClose.mock.calls.length, 1)
  assert.equal(onSuccess.mock.calls.length, 2)
  assert.equal(toast.add.mock.calls.at(-1)[0].message, 'toast.accountSaved')
  assert.ok(guards.close.mock.calls.length >= 1)
})

test('account dialog shows shared editing title and cancels without persistence', async () => {
  const onClose = vi.fn()
  const target = await render(AccountDialog, {
    open: true,
    editAccount: {
      id: 'shared', name: 'Shared', email: 'shared@example.test', sharedMailboxParentId: 'parent',
    },
    onClose,
  })
  assert.match(target.textContent, /account\.editSharedMailboxTitle/)
  target.querySelector('[data-account-form-cancel]').click()
  await flushAsync()
  assert.equal(target.querySelector('[role="dialog"]'), null)
  assert.equal(onClose.mock.calls.length, 1)
  assert.equal(accounts.updateAccount.mock.calls.length, 0)
})

test('delete dialog removes, cancels, ignores absent accounts, and reports errors', async () => {
  const onClose = vi.fn()
  const onSuccess = vi.fn()
  let target = await render(DeleteAccountDialog, {
    open: true,
    account: { id: 'account-1', name: 'Primary', email: 'primary@example.test' },
    onClose,
    onSuccess,
  })
  assert.match(target.textContent, /primary@example\.test/)
  target.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.deepEqual(accounts.removeAccount.mock.calls, [['account-1']])
  assert.equal(onSuccess.mock.calls.length, 1)
  assert.equal(onClose.mock.calls.length, 1)

  target = await render(DeleteAccountDialog, {
    open: true,
    account: { id: 'account-2', name: 'Secondary', email: 'secondary@example.test' },
    onClose,
  })
  target.querySelector('[data-confirm-action="cancel"]').click()
  await flushAsync()
  assert.equal(accounts.removeAccount.mock.calls.length, 1)

  target = await render(DeleteAccountDialog, { open: true, account: null })
  target.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.equal(accounts.removeAccount.mock.calls.length, 1)

  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  accounts.removeAccount.mockRejectedValueOnce(new Error('delete failed'))
  target = await render(DeleteAccountDialog, {
    open: true,
    account: { id: 'account-3', name: 'Failure', email: 'failure@example.test' },
  })
  target.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.equal(toast.add.mock.calls.at(-1)[0].message, 'toast.failedToDelete')
  assert.equal(error.mock.calls.length, 1)
})

test('connection dialog renders loading, success, failure, and close behavior', async () => {
  let target = await render(ConnectionTestDialog, {
    open: true, testing: true, result: null,
  })
  assert.match(target.textContent, /account\.testing/)
  assert.equal(buttonWithText(target, 'common.close').disabled, true)

  target = await render(ConnectionTestDialog, {
    open: true, testing: false, result: { success: true, message: 'connected' },
  })
  assert.match(target.textContent, /connected/)
  assert.match(target.innerHTML, /text-green-600/)

  const onClose = vi.fn()
  target = await render(ConnectionTestDialog, {
    open: true, testing: false, result: { success: false, message: 'rejected' }, onClose,
  })
  assert.match(target.innerHTML, /text-destructive/)
  buttonWithText(target, 'common.close').click()
  await flushAsync()
  assert.equal(target.querySelector('[role="dialog"]'), null)
  assert.equal(onClose.mock.calls.length, 1)
})

test('certificate dialog formats evidence and exposes all three trust decisions', async () => {
  const onDecline = vi.fn()
  const onAcceptOnce = vi.fn()
  const onAcceptPermanently = vi.fn()
  const target = await render(CertificateDialog, {
    open: true,
    certificate: {
      subject: 'mail.example.test',
      issuer: 'Synthetic CA',
      fingerprint: 'a1b2c3d4',
      notBefore: '2026-01-01T00:00:00Z',
      notAfter: '2026-12-31T00:00:00Z',
      isExpired: true,
      dnsNames: ['mail.example.test', 'imap.example.test'],
      errorReason: 'untrusted issuer',
    },
    onDecline,
    onAcceptOnce,
    onAcceptPermanently,
  })
  assert.match(target.textContent, /A1:B2:C3:D4/)
  assert.match(target.textContent, /mail\.example\.test, imap\.example\.test/)
  assert.match(target.textContent, /certificate\.expired/)
  buttonWithText(target, 'certificate.decline').click()
  buttonWithText(target, 'certificate.acceptOnce').click()
  buttonWithText(target, 'certificate.acceptAlways').click()
  assert.equal(onDecline.mock.calls.length, 1)
  assert.equal(onAcceptOnce.mock.calls.length, 1)
  assert.equal(onAcceptPermanently.mock.calls.length, 1)

  const empty = await render(CertificateDialog, {
    open: true,
    certificate: {
      subject: '', issuer: '', fingerprint: '', notBefore: '', notAfter: '',
      isExpired: false, dnsNames: [], errorReason: '',
    },
    onDecline: vi.fn(), onAcceptOnce: vi.fn(), onAcceptPermanently: vi.fn(),
  })
  assert.match(empty.textContent, /N\/A/)

  const noCertificate = await render(CertificateDialog, {
    open: true, certificate: null,
    onDecline: vi.fn(), onAcceptOnce: vi.fn(), onAcceptPermanently: vi.fn(),
  })
  assert.doesNotMatch(noCertificate.textContent, /certificate\.title/)
})
