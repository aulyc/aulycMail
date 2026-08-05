// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key) => key)
      return () => {}
    },
  },
}))
vi.mock('@iconify/svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))
vi.mock('$lib/components/ui/dialog', async () => ({
  Root: (await import('./fixtures/DialogRootTestStub.svelte')).default,
  Content: (await import('./fixtures/DialogContentTestStub.svelte')).default,
  Header: (await import('./fixtures/SnippetTestStub.svelte')).default,
  Title: (await import('./fixtures/SnippetTestStub.svelte')).default,
}))
vi.mock('$lib/components/ui/select', async () => ({
  Root: (await import('./fixtures/InteractiveSelectRootTestStub.svelte')).default,
  Trigger: (await import('./fixtures/SnippetTestStub.svelte')).default,
  Value: (await import('./fixtures/SnippetTestStub.svelte')).default,
  Content: (await import('./fixtures/SnippetTestStub.svelte')).default,
  Item: (await import('./fixtures/SelectItemTestStub.svelte')).default,
}))
vi.mock('../src/lib/components/settings/account/SignatureEditor.svelte', async () => ({
  default: (await import('./fixtures/SignatureEditorTestStub.svelte')).default,
}))

import IdentityEditor from '../src/lib/components/settings/account/IdentityEditor.svelte'

const mounted = []

async function flushAsync() {
  for (let index = 0; index < 7; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function renderEditor(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(IdentityEditor, { target, props: { open: true, accountId: 'account-1', ...props } })
  mounted.push(instance)
  await flushAsync()
  return { target, instance }
}

function setInput(target, selector, value) {
  const input = target.querySelector(selector)
  assert.ok(input, `missing ${selector}`)
  input.value = value
  input.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }))
  return input
}

function choose(target, current, value) {
  const root = target.querySelector(`[data-select-root][data-value="${current}"]`)
  assert.ok(root, `missing select ${current}`)
  root.querySelector(`[data-select-value="${value}"]`).click()
}

function submit(target) {
  target.querySelector('form').dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
}

beforeEach(() => {
  document.body.innerHTML = ''
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('validates a new identity and saves a normalized plain signature', async () => {
  const onSave = vi.fn().mockResolvedValue(undefined)
  const onClose = vi.fn()
  const { target } = await renderEditor({ onSave, onClose })
  submit(target)
  await flushAsync()
  assert.match(target.textContent, /identity\.emailRequired/)
  assert.match(target.textContent, /identity\.displayNameRequired/)

  setInput(target, '#email', 'not-an-email')
  setInput(target, '#name', 'Synthetic Person')
  submit(target)
  await flushAsync()
  assert.match(target.textContent, /identity\.invalidEmailFormat/)

  setInput(target, '#email', ' person@example.test ')
  choose(target, 'html', 'plain')
  await flushAsync()
  const textarea = target.querySelector('#signatureText')
  const tooManyLines = Array.from({ length: 105 }, (_, index) => `line-${index}`).join('\r\n')
  setInput(target, '#signatureText', tooManyLines)
  assert.equal(textarea.value.split('\n').length, 100)
  const arrowEvent = new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true })
  const stopPropagation = vi.spyOn(arrowEvent, 'stopPropagation')
  textarea.dispatchEvent(arrowEvent)
  assert.equal(stopPropagation.mock.calls.length, 1)

  const checks = target.querySelectorAll('input[type="checkbox"]')
  checks[1].click()
  choose(target, 'none', 'asterisk')
  await flushAsync()
  submit(target)
  await flushAsync()

  assert.equal(onSave.mock.calls.length, 1)
  const config = onSave.mock.calls[0][0]
  assert.equal(config.email, 'person@example.test')
  assert.equal(config.name, 'Synthetic Person')
  assert.equal(config.signatureEnabled, true)
  assert.equal(config.signatureHtml, '')
  assert.equal(config.signatureText.split('\n').length, 100)
  assert.equal(config.signatureForReply, false)
  assert.equal(config.signatureSeparator, true)
  assert.equal(config.signatureSeparatorStyle, '*****')
  assert.equal(onClose.mock.calls.length, 1)
})

test('edits a default identity, syncs its linked name, and saves HTML/disabled modes', async () => {
  const onNameChange = vi.fn()
  const onSave = vi.fn().mockResolvedValue(undefined)
  const identity = {
    id: 'identity-1', email: 'default@example.test', name: 'Old Name', isDefault: true,
    signatureHtml: '<p>Old signature</p>', signatureText: '', signatureEnabled: true,
    signatureForNew: false, signatureForReply: true, signatureForForward: false,
    signatureSeparatorStyle: '-----', signatureSeparator: true,
  }
  const { target } = await renderEditor({ identity, linkedName: 'Linked Name', onNameChange, onSave })
  assert.equal(target.querySelector('#email'), null)
  assert.equal(target.querySelector('#name').value, 'Linked Name')
  setInput(target, '#name', 'Edited Name')
  assert.deepEqual(onNameChange.mock.calls.at(-1), ['Edited Name'])
  setInput(target, '[data-signature-editor-stub]', '<p>New signature</p>')
  submit(target)
  await flushAsync()
  let config = onSave.mock.calls.at(-1)[0]
  assert.equal(config.email, 'default@example.test')
  assert.equal(config.signatureHtml, '<p>New signature</p>')
  assert.equal(config.signatureEnabled, true)
  assert.equal(config.signatureSeparatorStyle, '-----')

  await unmount(mounted.pop())
  onSave.mockClear()
  const disabled = await renderEditor({ identity: { ...identity, signatureEnabled: false }, onSave })
  assert.match(disabled.target.textContent, /identity\.signatureStatusDisabled/)
  submit(disabled.target)
  await flushAsync()
  config = onSave.mock.calls[0][0]
  assert.equal(config.signatureEnabled, false)
  assert.equal(config.signatureHtml, '')
  assert.equal(config.signatureText, '')
})

test('cancels cleanly and surfaces save failures', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const onClose = vi.fn()
  const onSave = vi.fn().mockRejectedValue(new Error('save unavailable'))
  const { target } = await renderEditor({ onClose, onSave })
  setInput(target, '#email', 'person@example.test')
  setInput(target, '#name', 'Person')
  submit(target)
  await flushAsync()
  assert.match(target.textContent, /identity\.saveFailed/)
  assert.equal(error.mock.calls.length, 1)
  const cancel = [...target.querySelectorAll('button')].find((button) => button.textContent.includes('common.cancel'))
  cancel.click()
  assert.equal(onClose.mock.calls.length, 1)
})
