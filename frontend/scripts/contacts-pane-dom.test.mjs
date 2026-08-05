// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const contacts = vi.hoisted(() => ({
  view: { selectedContactId: 'contact-1', detail: { id: 'contact-1', name: 'Ada' } },
  selectSource: vi.fn(), reload: vi.fn(), activate: vi.fn(),
}))
const toast = vi.hoisted(() => ({ error: vi.fn() }))
const pane = vi.hoisted(() => ({ shortcuts: [], events: [], unsub: vi.fn() }))

vi.mock('$contacts/stores/contactsView.svelte', () => ({
  contactsView: contacts.view,
  selectSource: contacts.selectSource,
  reloadContacts: contacts.reload,
  activateContact: contacts.activate,
}))
vi.mock('$lib/stores/toast', () => ({ toasts: toast }))
vi.mock('$lib/stores/paneShortcuts.svelte', () => ({
  registerPaneShortcut: (paneId, predicate, callback) => {
    const unregister = vi.fn()
    pane.shortcuts.push({ paneId, predicate, callback, unregister })
    return unregister
  },
}))
vi.mock('$wailsjs/runtime/runtime', () => ({
  EventsOn: (name, callback) => {
    pane.events.push({ name, callback })
    return pane.unsub
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
vi.mock('../src/lib/contacts/components/ContactsSidebar.svelte', async () => ({
  default: (await import('./fixtures/ContactsSidebarTestStub.svelte')).default,
}))
vi.mock('../src/lib/contacts/components/ContactList.svelte', async () => ({
  default: (await import('./fixtures/ContactListTestStub.svelte')).default,
}))
vi.mock('../src/lib/contacts/components/ContactDetail.svelte', async () => ({
  default: (await import('./fixtures/ContactDetailTestStub.svelte')).default,
}))
vi.mock('../src/lib/contacts/components/AddContactDialog.svelte', async () => ({
  default: (await import('./fixtures/AddContactDialogTestStub.svelte')).default,
}))
vi.mock('../src/lib/contacts/components/ContactEditDialog.svelte', async () => ({
  default: (await import('./fixtures/ContactEditDialogTestStub.svelte')).default,
}))
vi.mock('$lib/components/kit/PaneLayout.svelte', async () => ({
  default: (await import('./fixtures/SnippetTestStub.svelte')).default,
}))

import ContactsPane from '../src/lib/contacts/components/ContactsPane.svelte'

const mounted = []

async function flushAsync() {
  for (let index = 0; index < 7; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

beforeEach(() => {
  document.body.innerHTML = ''
  contacts.view.selectedContactId = 'contact-1'
  contacts.view.detail = { id: 'contact-1', name: 'Ada' }
  contacts.selectSource.mockReset()
  contacts.reload.mockReset().mockResolvedValue(undefined)
  contacts.activate.mockReset().mockResolvedValue(undefined)
  toast.error.mockReset()
  pane.shortcuts = []
  pane.events = []
  pane.unsub.mockReset()
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('registers events and shortcuts and owns add/edit dialog flows', async () => {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(ContactsPane, { target })
  mounted.push(instance)
  await flushAsync()

  assert.equal(contacts.reload.mock.calls.length, 1)
  assert.equal(pane.events[0].name, 'contacts:conflict')
  assert.equal(pane.shortcuts.length, 2)

  target.querySelector('[data-contacts-sidebar-select]').click()
  assert.equal(contacts.reload.mock.calls.length, 2)
  target.querySelector('[data-contact-add]').click()
  await flushAsync()
  assert.ok(target.querySelector('[data-add-contact-create]'))
  target.querySelector('[data-add-contact-create]').click()
  await flushAsync()
  assert.deepEqual(contacts.selectSource.mock.calls.at(-1), [''])
  assert.deepEqual(contacts.activate.mock.calls.at(-1), ['created-contact'])

  target.querySelector('[data-contact-edit]').click()
  await flushAsync()
  assert.equal(target.querySelector('[data-contact-edit-dialog]').dataset.contactId, 'contact-from-detail')
  pane.shortcuts[0].callback()
  await flushAsync()
  assert.equal(target.querySelector('[data-contact-edit-dialog]').dataset.contactId, 'contact-1')
  pane.shortcuts[1].callback()
  await flushAsync()
  assert.ok(target.querySelector('[data-add-contact-create]'))

  await pane.events[0].callback({ contactId: 'contact-1', message: 'conflict' })
  assert.equal(toast.error.mock.calls.at(-1)[0], 'contacts.toast.conflict')
  assert.deepEqual(contacts.activate.mock.calls.at(-1), ['contact-1'])
  const activationCount = contacts.activate.mock.calls.length
  await pane.events[0].callback({ contactId: 'other', message: 'conflict' })
  assert.equal(contacts.activate.mock.calls.length, activationCount)

  await unmount(mounted.pop())
  assert.equal(pane.unsub.mock.calls.length, 1)
  assert.equal(pane.shortcuts[0].unregister.mock.calls.length, 1)
  assert.equal(pane.shortcuts[1].unregister.mock.calls.length, 1)
})
