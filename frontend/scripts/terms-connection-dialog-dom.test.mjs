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

import '../src/lib/iconify-offline'
import ConnectionTestDialog from '../src/lib/components/settings/ConnectionTestDialog.svelte'
import TermsDialog from '../src/lib/components/TermsDialog.svelte'

const mounted = []

async function flushAsync() {
  await Promise.resolve()
  await tick()
  await Promise.resolve()
  await tick()
}

async function render(component, props) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(component, { target, props })
  mounted.push(instance)
  await flushAsync()
  return target
}

function buttonWithText(text) {
  return [...document.body.querySelectorAll('button')].find((button) => button.textContent.includes(text))
}

beforeEach(() => {
  document.body.innerHTML = ''
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
})

test('terms dialog opens both policies in app and accepts only after explicit agreement', async () => {
  const onAccept = vi.fn()
  await render(TermsDialog, { open: true, onAccept })
  const accept = buttonWithText('terms.accept')
  assert.ok(accept)
  assert.equal(accept.disabled, true)
  accept.click()
  assert.equal(onAccept.mock.calls.length, 0)

  const policy = buttonWithText('terms.privacyPolicy')
  const terms = buttonWithText('terms.termsOfUse')
  policy.click()
  await flushAsync()
  assert.match(document.body.textContent, /settingsAbout\.privacy\.title/)
  assert.match(document.body.textContent, /settingsAbout\.privacy\.noCollectionTitle/)
  buttonWithText('common.close').click()
  await flushAsync()

  terms.click()
  await flushAsync()
  assert.match(document.body.textContent, /settingsAbout\.terms\.title/)
  assert.match(document.body.textContent, /settingsAbout\.terms\.responsibilitiesTitle/)
  buttonWithText('common.close').click()
  await flushAsync()

  const agreement = document.querySelector('#agree-terms')
  agreement.click()
  await tick()
  assert.equal(accept.disabled, false)
  accept.click()
  assert.equal(onAccept.mock.calls.length, 1)
})

test('connection dialog renders testing, success, failure, and empty result states', async () => {
  const onClose = vi.fn()
  await render(ConnectionTestDialog, { open: true, testing: true, result: null, onClose })
  assert.match(document.body.textContent, /account\.testing/)
  assert.equal(buttonWithText('common.close').disabled, true)

  await render(ConnectionTestDialog, {
    open: true,
    testing: false,
    result: { success: true, message: 'Connected' },
    onClose,
  })
  assert.match(document.body.textContent, /Connected/)
  const enabledClose = [...document.body.querySelectorAll('button')]
    .filter((button) => button.textContent.includes('common.close'))
    .at(-1)
  assert.equal(enabledClose.disabled, false)
  enabledClose.click()
  await flushAsync()
  assert.ok(onClose.mock.calls.length >= 1)

  await render(ConnectionTestDialog, {
    open: true,
    testing: false,
    result: { success: false, message: 'Rejected' },
  })
  assert.match(document.body.textContent, /Rejected/)

  const emptyTarget = await render(ConnectionTestDialog, {
    open: true,
    testing: false,
    result: null,
  })
  assert.ok(emptyTarget)
})
