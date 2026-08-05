// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  GetIdentities: vi.fn(), CreateIdentity: vi.fn(), UpdateIdentity: vi.fn(),
  DeleteIdentity: vi.fn(), SetDefaultIdentity: vi.fn(),
}))
const toast = vi.hoisted(() => ({ add: vi.fn() }))

vi.mock('../wailsjs/go/app/App.js', () => backend)
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
vi.mock('../src/lib/components/settings/account/IdentityEditor.svelte', async () => ({
  default: (await import('./fixtures/IdentityEditorTestStub.svelte')).default,
}))
vi.mock('$lib/components/ui/confirm-dialog/ConfirmDialog.svelte', async () => ({
  default: (await import('./fixtures/ConfirmDialogTestStub.svelte')).default,
}))

import AccountIdentityTab from '../src/lib/components/settings/account/AccountIdentityTab.svelte'

const mounted = []

function identities() {
  return [
    { id: 'identity-default', email: 'default@example.test', name: 'Default Name', isDefault: true, signatureEnabled: false },
    { id: 'identity-html', email: 'html@example.test', name: 'HTML', isDefault: false, signatureEnabled: true, signatureHtml: '<p>Hi</p>' },
    { id: 'identity-plain', email: 'plain@example.test', name: 'Plain', isDefault: false, signatureEnabled: true, signatureText: 'Hi' },
  ]
}

async function flushAsync() {
  for (let index = 0; index < 8; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function renderTab(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(AccountIdentityTab, { target, props: { accountId: 'account-1', ...props } })
  mounted.push(instance)
  await flushAsync()
  return { target, instance }
}

function titledButtons(target, title) {
  return [...target.querySelectorAll(`button[title="${title}"]`)]
}

beforeEach(() => {
  document.body.innerHTML = ''
  backend.GetIdentities.mockReset().mockResolvedValue(identities())
  for (const fn of [backend.CreateIdentity, backend.UpdateIdentity, backend.DeleteIdentity, backend.SetDefaultIdentity]) {
    fn.mockReset().mockResolvedValue(undefined)
  }
  toast.add.mockReset()
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('loads identity badges, creates aliases, and sets a default identity', async () => {
  const { target } = await renderTab()
  assert.deepEqual(backend.GetIdentities.mock.calls[0], ['account-1'])
  assert.match(target.textContent, /identity\.signatureBadgeNone/)
  assert.match(target.textContent, /identity\.signatureBadgeHtml/)
  assert.match(target.textContent, /identity\.signatureBadgePlain/)

  const radioButtons = [...target.querySelectorAll('button[title]')].filter((button) => button.title.includes('identity.setAsDefaultAddress'))
  radioButtons[0].click()
  await flushAsync()
  assert.deepEqual(backend.SetDefaultIdentity.mock.calls[0], ['account-1', 'identity-html'])
  assert.equal(toast.add.mock.calls.at(-1)[0].type, 'success')

  const add = [...target.querySelectorAll('button')].find((button) => button.textContent.includes('identity.addEmailAddress'))
  add.click()
  await flushAsync()
  target.querySelector('[data-identity-save]').click()
  await flushAsync()
  assert.deepEqual(backend.CreateIdentity.mock.calls[0], ['account-1', { email: 'saved@example.test', name: 'Saved Name' }])
  assert.match(toast.add.mock.calls.at(-1)[0].message, /identity\.emailAdded/)
})

test('edits the default name, restores it on cancel, and persists it on save', async () => {
  const onDefaultDisplayNameChange = vi.fn()
  const { target } = await renderTab({
    defaultDisplayName: 'Live Default', displayNameLoaded: true, onDefaultDisplayNameChange,
  })
  titledButtons(target, 'common.edit')[0].click()
  await flushAsync()
  let editor = target.querySelector('[data-identity-editor]')
  assert.equal(editor.dataset.identityId, 'identity-default')
  assert.equal(editor.dataset.linkedName, 'Live Default')
  editor.querySelector('[data-identity-name]').click()
  assert.deepEqual(onDefaultDisplayNameChange.mock.calls.at(-1), ['Draft Name'])
  editor.querySelector('[data-identity-close]').click()
  assert.deepEqual(onDefaultDisplayNameChange.mock.calls.at(-1), ['Live Default'])

  titledButtons(target, 'common.edit')[0].click()
  await flushAsync()
  editor = target.querySelector('[data-identity-editor]')
  editor.querySelector('[data-identity-save]').click()
  await flushAsync()
  assert.deepEqual(backend.UpdateIdentity.mock.calls[0], ['identity-default', { email: 'saved@example.test', name: 'Saved Name' }])
  assert.deepEqual(onDefaultDisplayNameChange.mock.calls.at(-1), ['Saved Name'])
  assert.match(toast.add.mock.calls.at(-1)[0].message, /identity\.emailUpdated/)
})

test('deletes a non-default identity and reports delete/default failures', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const { target } = await renderTab()
  const deleteButtons = titledButtons(target, 'common.delete')
  deleteButtons[0].click()
  await flushAsync()
  target.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.deepEqual(backend.DeleteIdentity.mock.calls[0], ['identity-html'])
  assert.match(toast.add.mock.calls.at(-1)[0].message, /identity\.emailDeleted/)

  backend.SetDefaultIdentity.mockRejectedValueOnce(new Error('default unavailable'))
  const radioButtons = [...target.querySelectorAll('button[title]')].filter((button) => button.title.includes('identity.setAsDefaultAddress'))
  radioButtons[0].click()
  await flushAsync()
  assert.match(toast.add.mock.calls.at(-1)[0].message, /identity\.failedToSetDefault/)

  backend.DeleteIdentity.mockRejectedValueOnce(new Error('delete unavailable'))
  titledButtons(target, 'common.delete')[0].click()
  await flushAsync()
  target.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.equal(toast.add.mock.calls.at(-1)[0].type, 'error')
  assert.equal(error.mock.calls.length, 2)
})

test('renders empty state and reports initial load errors', async () => {
  backend.GetIdentities.mockResolvedValueOnce([])
  let rendered = await renderTab()
  assert.match(rendered.target.textContent, /identity\.noEmailAddresses/)
  await unmount(mounted.pop())

  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.GetIdentities.mockRejectedValueOnce(new Error('load unavailable'))
  await renderTab()
  assert.match(toast.add.mock.calls.at(-1)[0].message, /identity\.failedToLoadAddresses/)
  assert.equal(error.mock.calls.length, 1)
})
