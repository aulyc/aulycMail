import assert from 'node:assert/strict'
import { get } from 'svelte/store'
import { test, vi } from 'vitest'

import { KEY as CONTACT_KEY } from '../src/lib/contacts/keyboard/shortcuts.ts'
import { detectProvider, getCustomProvider, providers } from '../src/lib/config/providers.ts'
import { isOptionCodeShortcut, resolveAppKeyboardPolicy } from '../src/lib/keyboard/keyboardPolicy.ts'
import { altOnly, ctrlOrMeta, KEY, noMods } from '../src/lib/keyboard/shortcuts.ts'
import { addToast, toasts } from '../src/lib/stores/toast.ts'
import {
  loadBackupDirectoryHistory,
  rememberBackupDirectory,
  removeBackupDirectory,
  subscribeBackupDirectoryHistory,
} from '../src/lib/utils/backup-directory-history.ts'
import { createDebouncer } from '../src/lib/utils/debounce.ts'

function keyboardEvent(key, overrides = {}) {
  return {
    key,
    code: '',
    altKey: false,
    ctrlKey: false,
    metaKey: false,
    shiftKey: false,
    ...overrides,
  }
}

test('shared keyboard predicates distinguish navigation and modifier contracts', () => {
  assert.equal(noMods(keyboardEvent('j')), true)
  assert.equal(noMods(keyboardEvent('j', { shiftKey: true })), false)
  assert.equal(ctrlOrMeta(keyboardEvent('a', { metaKey: true })), true)
  assert.equal(ctrlOrMeta(keyboardEvent('a', { ctrlKey: true, altKey: true })), false)
  assert.equal(altOnly(keyboardEvent('h', { altKey: true })), true)
  assert.equal(altOnly(keyboardEvent('h', { altKey: true, metaKey: true })), false)

  assert.equal(KEY.LIST_NEXT(keyboardEvent('j')), true)
  assert.equal(KEY.LIST_NEXT(keyboardEvent('ArrowDown')), true)
  assert.equal(KEY.LIST_NEXT(keyboardEvent('j', { shiftKey: true })), false)
  assert.equal(KEY.LIST_PREV(keyboardEvent('k')), true)
  assert.equal(KEY.LIST_NEXT_CHECK(keyboardEvent('J', { shiftKey: true })), true)
  assert.equal(KEY.LIST_PREV_CHECK(keyboardEvent('ArrowUp', { shiftKey: true })), true)
  assert.equal(KEY.LIST_TOGGLE_CHECK(keyboardEvent(' ')), true)
  assert.equal(KEY.LIST_OPEN(keyboardEvent('Enter')), true)
  assert.equal(KEY.LIST_SELECT_ALL(keyboardEvent('A', { metaKey: true })), true)
  assert.equal(KEY.LIST_DELETE(keyboardEvent('Backspace')), true)
  assert.equal(KEY.PANE_FOCUS_NEXT(keyboardEvent('l', { altKey: true })), true)
  assert.equal(KEY.PANE_FOCUS_PREV(keyboardEvent('ArrowLeft', { altKey: true })), true)
  assert.equal(KEY.SIDEBAR_NEXT(keyboardEvent('ArrowDown', { altKey: true })), true)
  assert.equal(KEY.SIDEBAR_PREV(keyboardEvent('k', { altKey: true })), true)
})

test('contacts keyboard predicates share the global modifier convention', () => {
  assert.equal(CONTACT_KEY.CONTACT_EDIT(keyboardEvent('e')), true)
  assert.equal(CONTACT_KEY.CONTACT_EDIT(keyboardEvent('e', { altKey: true })), false)
  assert.equal(CONTACT_KEY.CONTACT_NEW(keyboardEvent('n', { metaKey: true })), true)
  assert.equal(CONTACT_KEY.CONTACT_NEW(keyboardEvent('n', { ctrlKey: true, shiftKey: true })), false)
})

test('application keyboard policy keeps search always on and native mode untouched', () => {
  assert.equal(resolveAppKeyboardPolicy(keyboardEvent('f', { metaKey: true }), false), 'search')
  assert.equal(resolveAppKeyboardPolicy(keyboardEvent('F', { ctrlKey: true }), false), 'search')
  assert.equal(resolveAppKeyboardPolicy(keyboardEvent('f', { metaKey: true, altKey: true }), true), 'enhanced')
  assert.equal(resolveAppKeyboardPolicy(keyboardEvent('j'), true), 'enhanced')
  assert.equal(resolveAppKeyboardPolicy(keyboardEvent('j'), false), 'native')
  assert.equal(isOptionCodeShortcut(keyboardEvent('†', { code: 'KeyT', altKey: true }), 'KeyT'), true)
  assert.equal(isOptionCodeShortcut(keyboardEvent('t', { code: 'KeyT', altKey: true, ctrlKey: true }), 'KeyT'), false)
})

test('provider detection is case-insensitive and keeps manual setup last', () => {
  assert.equal(detectProvider('User@GMAIL.COM')?.id, 'gmail')
  assert.equal(detectProvider('user@pm.me')?.id, 'protonmail')
  assert.equal(detectProvider('user@unknown.example'), null)
  assert.equal(detectProvider('not-an-email'), null)
  assert.equal(getCustomProvider().id, 'custom')
  assert.equal(providers.at(-1)?.id, 'custom')
  assert.equal(providers.every((provider) => provider.imap.port > 0 && provider.smtp.port > 0), true)
})

test('debouncer replaces pending work, supports custom delay, and cancels cleanly', () => {
  vi.useFakeTimers()
  try {
    const calls = []
    const debouncer = createDebouncer(50)
    debouncer.schedule(() => calls.push('first'))
    debouncer.schedule(() => calls.push('replacement'))
    vi.advanceTimersByTime(49)
    assert.deepEqual(calls, [])
    vi.advanceTimersByTime(1)
    assert.deepEqual(calls, ['replacement'])

    debouncer.schedule(() => calls.push('custom'), 10)
    vi.advanceTimersByTime(10)
    assert.deepEqual(calls, ['replacement', 'custom'])

    debouncer.schedule(() => calls.push('cancelled'))
    debouncer.cancel()
    debouncer.cancel()
    vi.runAllTimers()
    assert.deepEqual(calls, ['replacement', 'custom'])
  } finally {
    vi.useRealTimers()
  }
})

test('toast store exposes typed helpers and removes entries after their duration', () => {
  vi.useFakeTimers()
  try {
    const firstId = toasts.success('Saved')
    assert.deepEqual(get(toasts).map(({ message, type }) => ({ message, type })), [
      { message: 'Saved', type: 'success' },
    ])
    assert.equal(get(toasts)[0].id, firstId)

    toasts.error('Failed')
    assert.equal(get(toasts)[0].type, 'error')
    toasts.info('Info')
    assert.equal(get(toasts)[0].type, 'info')
    toasts.warning('Warning')
    assert.equal(get(toasts)[0].type, 'warning')

    const shortId = addToast({ message: 'Short', type: 'info', duration: 10 })
    assert.equal(get(toasts)[0].id, shortId)
    vi.advanceTimersByTime(10)
    assert.deepEqual(get(toasts), [])

    const removableId = toasts.success('Remove')
    toasts.remove(removableId)
    assert.deepEqual(get(toasts), [])
  } finally {
    vi.runAllTimers()
    vi.useRealTimers()
  }
})

test('backup directory history normalizes, deduplicates, caps, and removes paths', () => {
  const entries = new Map()
  const storage = {
    getItem: (key) => entries.get(key) ?? null,
    setItem: (key, value) => entries.set(key, String(value)),
  }
  vi.stubGlobal('localStorage', storage)
  vi.stubGlobal('window', new EventTarget())
  try {
    assert.deepEqual(loadBackupDirectoryHistory(), [])
    assert.deepEqual(rememberBackupDirectory(' /backup/a '), ['/backup/a'])
    assert.deepEqual(rememberBackupDirectory('/backup/b'), ['/backup/b', '/backup/a'])
    assert.deepEqual(rememberBackupDirectory('/backup/a'), ['/backup/a', '/backup/b'])
    assert.deepEqual(removeBackupDirectory('/backup/a'), ['/backup/b'])
    assert.deepEqual(removeBackupDirectory(' '), ['/backup/b'])

    const many = Array.from({ length: 55 }, (_, index) => `/backup/${index}`)
    entries.set('aulycmail.backupViewer.directoryHistory', JSON.stringify([...many, '/backup/0', '', 42]))
    assert.equal(loadBackupDirectoryHistory().length, 50)

    entries.set('aulycmail.backupViewer.directoryHistory', '{broken')
    assert.deepEqual(loadBackupDirectoryHistory(), [])
    entries.set('aulycmail.backupViewer.directoryHistory', JSON.stringify({ path: '/backup/a' }))
    assert.deepEqual(loadBackupDirectoryHistory(), [])
  } finally {
    vi.unstubAllGlobals()
  }
})

test('backup directory subscribers receive direct and storage-driven updates and can unsubscribe', () => {
  const entries = new Map()
  const storage = {
    getItem: (key) => entries.get(key) ?? null,
    setItem: (key, value) => entries.set(key, String(value)),
  }
  class TestStorageEvent extends Event {
    constructor(type, init) {
      super(type)
      this.key = init.key
    }
  }
  vi.stubGlobal('localStorage', storage)
  vi.stubGlobal('window', new EventTarget())
  vi.stubGlobal('StorageEvent', TestStorageEvent)
  try {
    const updates = []
    const unsubscribe = subscribeBackupDirectoryHistory((paths) => updates.push(paths))
    rememberBackupDirectory('/backup/a')
    entries.set('aulycmail.backupViewer.directoryHistory', JSON.stringify(['/backup/b']))
    window.dispatchEvent(new StorageEvent('storage', { key: 'aulycmail.backupViewer.directoryHistory' }))
    window.dispatchEvent(new StorageEvent('storage', { key: 'unrelated' }))
    unsubscribe()
    rememberBackupDirectory('/backup/c')

    assert.deepEqual(updates, [['/backup/a'], ['/backup/b']])
  } finally {
    vi.unstubAllGlobals()
  }
})
