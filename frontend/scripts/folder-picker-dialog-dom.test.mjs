// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({ GetFolders: vi.fn() }))
const settings = vi.hoisted(() => ({ enhanced: true }))

vi.mock('../wailsjs/go/app/App', () => backend)
vi.mock('$lib/stores/settings.svelte', () => ({
  getEnhancedKeyboardNavigation: () => settings.enhanced,
}))
vi.mock('$lib/i18n', () => ({
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
  return { Root: root, Content: content, Header: snippet, Title: snippet, Footer: snippet }
})
vi.mock('$lib/components/ui/select', async () => {
  const root = (await import('./fixtures/FolderPickerSelectRootTestStub.svelte')).default
  const snippet = (await import('./fixtures/SnippetTestStub.svelte')).default
  const item = (await import('./fixtures/SelectItemTestStub.svelte')).default
  return { Root: root, Trigger: snippet, Value: snippet, Content: snippet, Item: item }
})

import FolderPickerDialog from '../src/lib/components/common/FolderPickerDialog.svelte'

const mounted = []

function folder(id, name, path, type = 'folder') {
  return { id, name, path, type }
}

async function flushAsync() {
  for (let index = 0; index < 6; index += 1) {
    await Promise.resolve()
    await tick()
  }
  await new Promise((resolve) => setTimeout(resolve, 0))
  await tick()
}

async function renderPicker(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const onSelect = props.onSelect ?? vi.fn()
  const instance = mount(FolderPickerDialog, {
    target,
    props: {
      open: true,
      title: 'Move message',
      initialAccountId: 'primary',
      accounts: [{ id: 'primary', name: 'Primary' }],
      onSelect,
      ...props,
    },
  })
  mounted.push(instance)
  await flushAsync()
  return { target, onSelect }
}

function inputText(input, value) {
  input.value = value
  input.dispatchEvent(new InputEvent('input', { bubbles: true }))
}

beforeEach(() => {
  document.body.innerHTML = ''
  backend.GetFolders.mockReset().mockResolvedValue([
    folder('inbox', 'Inbox', 'Inbox', 'inbox'),
    folder('projects', 'Child', 'Projects.Child'),
    folder('archive', 'Archive', 'Archive', 'archive'),
  ])
  settings.enhanced = true
  if (!HTMLElement.prototype.scrollIntoView) HTMLElement.prototype.scrollIntoView = vi.fn()
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('loads, sorts, excludes, searches, and selects folders by keyboard and pointer', async () => {
  const { target, onSelect } = await renderPicker({ excludeFolderId: 'inbox' })
  assert.deepEqual(backend.GetFolders.mock.calls, [['primary']])
  assert.match(target.textContent, /Move message/)
  assert.equal(target.querySelector('[title="Inbox"]'), null)

  const options = [...target.querySelectorAll('[role="option"]')]
  assert.deepEqual(options.map((button) => button.textContent.trim()), ['Archive', 'Child'])
  assert.equal(options[0].getAttribute('aria-selected'), 'true')

  const search = target.querySelector('input')
  inputText(search, 'projects')
  await flushAsync()
  assert.deepEqual(
    [...target.querySelectorAll('[role="option"]')].map((button) => button.textContent.trim()),
    ['Projects / Child'],
  )

  search.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
  assert.deepEqual(onSelect.mock.calls.at(-1), ['projects', 'Child', 'primary'])

  inputText(search, '')
  await flushAsync()
  search.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true, cancelable: true }))
  search.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
  assert.deepEqual(onSelect.mock.calls.at(-1), ['projects', 'Child', 'primary'])

  target.querySelectorAll('[role="option"]')[0].click()
  assert.deepEqual(onSelect.mock.calls.at(-1), ['archive', 'Archive', 'primary'])
})

test('reloads on account change and ignores stale responses', async () => {
  let resolvePrimary
  backend.GetFolders.mockImplementation((accountId) => {
    if (accountId === 'primary') return new Promise((resolve) => { resolvePrimary = resolve })
    return Promise.resolve([folder('sent', 'Sent', '[Gmail]/Sent', 'sent')])
  })
  const { target, onSelect } = await renderPicker({
    accounts: [
      { id: 'primary', name: 'Primary' },
      { id: 'secondary', name: 'Secondary' },
    ],
  })
  assert.match(target.textContent, /common\.loading/)

  target.querySelector('[data-select-secondary]').click()
  await flushAsync()
  assert.deepEqual(backend.GetFolders.mock.calls, [['primary'], ['secondary']])
  assert.match(target.textContent, /Sent/)

  resolvePrimary([folder('late', 'Late primary', 'Late')])
  await flushAsync()
  assert.doesNotMatch(target.textContent, /Late primary/)
  target.querySelector('[role="option"]').click()
  assert.deepEqual(onSelect.mock.calls.at(-1), ['sent', 'Sent', 'secondary'])
})

test('handles empty, failed, disabled-keyboard, and cancel states', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.GetFolders.mockRejectedValueOnce(new Error('offline'))
  const { target, onSelect } = await renderPicker()
  assert.match(target.textContent, /contextMenu\.noFoldersAvailable/)
  assert.equal(error.mock.calls.length, 1)

  const search = target.querySelector('input')
  settings.enhanced = false
  search.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
  assert.equal(onSelect.mock.calls.length, 0)

  const cancel = [...target.querySelectorAll('button')].find((button) => button.textContent.includes('common.cancel'))
  cancel.click()
  await flushAsync()
  assert.equal(target.querySelector('[role="dialog"]'), null)
})
