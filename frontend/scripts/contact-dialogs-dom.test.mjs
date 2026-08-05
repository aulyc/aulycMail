// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const contacts = vi.hoisted(() => ({ create: vi.fn(), update: vi.fn() }))
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
const guards = vi.hoisted(() => ({ open: vi.fn(), close: vi.fn() }))

vi.mock('$contacts/stores/contactsView.svelte', () => ({
  createContact: contacts.create,
  updateContact: contacts.update,
}))
vi.mock('$lib/stores/toast', () => ({ toasts: toast }))
vi.mock('$lib/stores/dialogGuard', () => ({
  dialogGuardOpen: guards.open,
  dialogGuardClose: guards.close,
}))
vi.mock('svelte-i18n', () => ({
  _: {
    subscribe(run) {
      run((key) => key)
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
  return { Root: root, Content: content, Header: snippet, Title: snippet, Description: snippet }
})

import AddContactDialog from '../src/lib/contacts/components/AddContactDialog.svelte'
import ContactEditDialog from '../src/lib/contacts/components/ContactEditDialog.svelte'

const mounted = []

async function flushAsync() {
  for (let index = 0; index < 6; index += 1) {
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

function setValue(input, value) {
  assert.ok(input, 'missing input')
  input.value = value
  input.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }))
}

function textButton(target, text) {
  const result = [...target.querySelectorAll('button')].find((item) => item.textContent.includes(text))
  assert.ok(result, `missing button containing ${text}`)
  return result
}

function saveButton(target) {
  return textButton(target, 'contacts.common.save')
}

function addEmailButton(target) {
  return textButton(target, 'contacts.edit.addEmail')
}

function editContact(overrides = {}) {
  return {
    id: 'contact-1',
    name: 'Ada Lovelace',
    note: 'Original note',
    emails: ['fallback@example.test'],
    emailItems: [{ email: 'ADA@EXAMPLE.TEST', type: 'work', isPrimary: true }],
    ...overrides,
  }
}

beforeEach(() => {
  document.body.innerHTML = ''
  contacts.create.mockReset().mockResolvedValue('created-contact')
  contacts.update.mockReset().mockResolvedValue(undefined)
  toast.success.mockReset()
  toast.error.mockReset()
  guards.open.mockReset()
  guards.close.mockReset()
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('add dialog validates empty and malformed addresses, then creates normalized contact data', async () => {
  const onCreated = vi.fn()
  const onClose = vi.fn()
  const target = await render(AddContactDialog, { open: true, onCreated, onClose })
  assert.match(target.textContent, /contacts\.add\.title/)
  assert.equal(guards.open.mock.calls.length, 1)

  saveButton(target).click()
  await flushAsync()
  assert.match(target.textContent, /contacts\.add\.errorEmailRequired/)
  assert.equal(contacts.create.mock.calls.length, 0)

  const firstEmail = target.querySelector('input[type="email"]')
  setValue(firstEmail, '@invalid')
  saveButton(target).click()
  await flushAsync()
  assert.match(target.textContent, /contacts\.add\.errorEmailInvalid/)
  assert.equal(firstEmail.getAttribute('aria-invalid'), 'true')

  setValue(target.querySelector('#cf-name'), '  Ada Lovelace  ')
  setValue(target.querySelector('#cf-note'), '  Synthetic note  ')
  setValue(firstEmail, '')
  addEmailButton(target).click()
  await flushAsync()
  const emailInputs = target.querySelectorAll('input[type="email"]')
  assert.equal(emailInputs.length, 2)
  setValue(emailInputs[1], '  ADA@EXAMPLE.TEST  ')
  saveButton(target).click()
  await flushAsync()

  assert.equal(contacts.create.mock.calls.length, 1)
  const input = contacts.create.mock.calls[0][0]
  assert.equal(input.sourceId, 'local:manual')
  assert.equal(input.email, 'ada@example.test')
  assert.equal(input.name, 'Ada Lovelace')
  assert.equal(input.note, 'Synthetic note')
  assert.deepEqual(
    input.emails.map(({ email, type, isPrimary }) => ({ email, type, isPrimary })),
    [{ email: 'ada@example.test', type: '', isPrimary: true }],
  )
  assert.equal(toast.success.mock.calls.at(-1)[0], 'contacts.toast.added')
  assert.deepEqual(onCreated.mock.calls, [['created-contact']])
  assert.equal(onClose.mock.calls.length, 1)
  assert.equal(target.querySelector('[role="dialog"]'), null)
})

test('add dialog reports duplicate and unexpected failures and resets when mounted open again', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  contacts.create
    .mockRejectedValueOnce(new Error('contact already exists'))
    .mockRejectedValueOnce(new Error('backend unavailable'))

  let target = await render(AddContactDialog, { open: true })
  setValue(target.querySelector('input[type="email"]'), 'duplicate@example.test')
  saveButton(target).click()
  await flushAsync()
  assert.match(target.textContent, /contacts\.add\.errorEmailExists/)
  assert.equal(toast.error.mock.calls.length, 0)
  assert.equal(target.querySelector('#cf-name').disabled, false)

  setValue(target.querySelector('input[type="email"]'), 'failure@example.test')
  saveButton(target).click()
  await flushAsync()
  assert.equal(toast.error.mock.calls.at(-1)[0], 'contacts.toast.failedAdd: backend unavailable')
  assert.equal(error.mock.calls.length, 1)

  target = await render(AddContactDialog, { open: true })
  assert.equal(target.querySelector('#cf-name').value, '')
  assert.equal(target.querySelector('input[type="email"]').value, '')
  assert.doesNotMatch(target.textContent, /contacts\.add\.errorEmailExists/)
  textButton(target, 'contacts.common.cancel').click()
  await flushAsync()
  assert.equal(target.querySelector('[role="dialog"]'), null)
})

test('edit dialog hydrates structured emails, validates fields, and saves a normalized patch', async () => {
  const onClose = vi.fn()
  const target = await render(ContactEditDialog, {
    open: true,
    contact: editContact(),
    onClose,
  })
  assert.equal(target.querySelector('#cf-name').value, 'Ada Lovelace')
  assert.equal(target.querySelector('#cf-note').value, 'Original note')
  assert.equal(target.querySelector('input[type="email"]').value, 'ADA@EXAMPLE.TEST')
  assert.equal(guards.open.mock.calls.length, 1)

  setValue(target.querySelector('#cf-name'), '   ')
  setValue(target.querySelector('input[type="email"]'), 'missing-at-sign')
  saveButton(target).click()
  await flushAsync()
  assert.match(target.textContent, /contacts\.edit\.nameRequired/)
  assert.match(target.textContent, /contacts\.edit\.emailInvalid/)
  assert.equal(contacts.update.mock.calls.length, 0)

  setValue(target.querySelector('#cf-name'), '  Updated Ada  ')
  setValue(target.querySelector('#cf-note'), '  Updated note  ')
  setValue(target.querySelector('input[type="email"]'), '  UPDATED@EXAMPLE.TEST  ')
  addEmailButton(target).click()
  await flushAsync()
  saveButton(target).click()
  await flushAsync()
  assert.deepEqual(contacts.update.mock.calls, [[
    'contact-1',
    {
      name: 'Updated Ada',
      note: 'Updated note',
      emails: [{ email: 'updated@example.test', type: 'work', isPrimary: true }],
    },
  ]])
  assert.equal(toast.success.mock.calls.at(-1)[0], 'contacts.toast.updated')
  assert.equal(onClose.mock.calls.length, 1)
  assert.equal(target.querySelector('[role="dialog"]'), null)
})

test('edit dialog supports fallback and empty email data, cancellation, missing ids, and save errors', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  let target = await render(ContactEditDialog, {
    open: true,
    contact: editContact({ emailItems: [], emails: ['FIRST@EXAMPLE.TEST', 'second@example.test'] }),
  })
  assert.deepEqual(
    [...target.querySelectorAll('input[type="email"]')].map((input) => input.value),
    ['FIRST@EXAMPLE.TEST', 'second@example.test'],
  )
  textButton(target, 'contacts.common.cancel').click()
  await flushAsync()
  assert.equal(target.querySelector('[role="dialog"]'), null)

  target = await render(ContactEditDialog, {
    open: true,
    contact: editContact({ id: '', emailItems: [], emails: [] }),
  })
  assert.equal(target.querySelectorAll('input[type="email"]').length, 0)
  assert.equal(saveButton(target).disabled, true)
  saveButton(target).click()
  assert.equal(contacts.update.mock.calls.length, 0)

  contacts.update.mockRejectedValueOnce(new Error('update unavailable'))
  target = await render(ContactEditDialog, { open: true, contact: editContact() })
  saveButton(target).click()
  await flushAsync()
  assert.equal(toast.error.mock.calls.at(-1)[0], 'contacts.toast.failedUpdate: update unavailable')
  assert.equal(error.mock.calls.length, 1)
  assert.equal(target.querySelector('#cf-name').disabled, false)

  target = await render(ContactEditDialog, { open: true, contact: null })
  assert.equal(saveButton(target).disabled, true)
})
