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
vi.mock('@iconify/svelte', async () => ({
  default: (await import('./fixtures/StaticStub.svelte')).default,
}))
vi.mock('$lib/components/backup/BackupDirectoryPicker.svelte', async () => ({
  default: (await import('./fixtures/StaticStub.svelte')).default,
}))
vi.mock('$lib/components/ui/select', async () => {
  const root = (await import('./fixtures/SelectActionRootTestStub.svelte')).default
  const snippet = (await import('./fixtures/SnippetTestStub.svelte')).default
  const item = (await import('./fixtures/SelectItemTestStub.svelte')).default
  return { Root: root, Trigger: snippet, Value: snippet, Content: snippet, Item: item }
})

import BackupViewerSearchOverlay from '../src/lib/components/backup/BackupViewerSearchOverlay.svelte'
import BackupViewerToolbar from '../src/lib/components/backup/BackupViewerToolbar.svelte'

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

beforeEach(() => {
  document.body.innerHTML = ''
  vi.spyOn(window, 'requestAnimationFrame').mockImplementation((callback) => {
    callback(performance.now())
    return 1
  })
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('backup toolbar routes directory, scope, catalog, sort, filter, and close actions', async () => {
  const handlers = Object.fromEntries([
    'onLoadCatalog', 'onChooseDirectoryError', 'onRemoveDirectoryHistory', 'onOpenDirectory',
    'onSelectScope', 'onScrollToTop', 'onClearDirectory', 'onRefreshCatalog', 'onOpenSearch',
    'onBuildIndex', 'onToggleSortOrder', 'onClose',
  ].map((name) => [name, vi.fn()]))
  const target = await render(BackupViewerToolbar, {
    directory: '/backup',
    directoryMenuOpen: false,
    catalog: { messageCount: 3, needsIndex: true },
    selectedAccountEmail: '',
    selectedScope: { id: '', label: 'All accounts', count: 3 },
    accountScopes: [
      { id: '', label: 'All accounts', count: 3 },
      { id: 'secondary', label: 'Secondary', count: 1 },
    ],
    loadingCatalog: false,
    buildingIndex: false,
    errorMessage: 'synthetic warning',
    darkFilterEnabled: false,
    messageSortOrder: 'newest',
    scopeLabel: (scope) => scope?.label ?? 'missing',
    ...handlers,
  })
  assert.match(target.textContent, /All accounts/)
  assert.match(target.textContent, /synthetic warning/)
  assert.ok(target.querySelector('[aria-label="backupViewer.buildIndex"]'))

  target.querySelector('[data-select-secondary]').click()
  assert.deepEqual(handlers.onSelectScope.mock.calls.at(-1), ['secondary'])

  const toolbarActions = [
    ['backupViewer.scrollToTop', handlers.onScrollToTop],
    ['backupViewer.clearDirectory', handlers.onClearDirectory],
    ['backupViewer.refresh', handlers.onRefreshCatalog],
    ['backupViewer.search', handlers.onOpenSearch],
    ['backupViewer.buildIndex', handlers.onBuildIndex],
    ['backupViewer.showingNewest', handlers.onToggleSortOrder],
    ['common.close', handlers.onClose],
  ]
  for (const [label, handler] of toolbarActions) {
    target.querySelector(`[aria-label="${label}"]`).click()
    assert.equal(handler.mock.calls.length, 1)
  }

  const dark = target.querySelector('[aria-label="backupViewer.darkFilter"]')
  assert.equal(dark.getAttribute('aria-pressed'), 'false')
  dark.click()
  await flushAsync()
  assert.equal(dark.getAttribute('aria-pressed'), 'true')
})

test('backup toolbar disables unavailable actions and hides the index action', async () => {
  const noop = vi.fn()
  const target = await render(BackupViewerToolbar, {
    directory: '', directoryMenuOpen: false, catalog: null,
    selectedAccountEmail: '', selectedScope: undefined, accountScopes: [],
    loadingCatalog: true, buildingIndex: false, errorMessage: '', darkFilterEnabled: true,
    messageSortOrder: 'oldest', onLoadCatalog: noop, onChooseDirectoryError: noop,
    onRemoveDirectoryHistory: noop, onOpenDirectory: noop, onSelectScope: noop,
    onScrollToTop: noop, onClearDirectory: noop, onRefreshCatalog: noop,
    onOpenSearch: noop, onBuildIndex: noop, onToggleSortOrder: noop, onClose: noop,
    scopeLabel: () => 'All',
  })
  assert.equal(target.querySelector('[aria-label="backupViewer.buildIndex"]'), null)
  assert.equal(target.querySelector('[aria-label="backupViewer.refresh"]').disabled, true)
  assert.equal(target.querySelector('[aria-label="backupViewer.search"]').disabled, true)
  assert.ok(target.querySelector('[aria-label="backupViewer.showingOldest"]'))
  assert.equal(target.querySelector('[aria-label="backupViewer.darkFilter"]').getAttribute('aria-pressed'), 'true')
})

test('backup search overlay forwards input events, scope changes, movement, and result selection', async () => {
  const handlers = Object.fromEntries([
    'onClose', 'onSelectSearchScope', 'onSearchInput', 'onCompositionStart',
    'onCompositionEnd', 'onSearchKeydown', 'onSelectSearchResult',
  ].map((name) => [name, vi.fn()]))
  const target = await render(BackupViewerSearchOverlay, {
    open: true,
    accountScopes: [
      { id: '', label: 'All' },
      { id: 'secondary', label: 'Secondary' },
    ],
    searchScopeEmail: '',
    searchQuery: 'report',
    searchResults: [
      {
        key: 'one', subject: '', date: '2026-08-01', accountEmail: 'a@example.test',
        folderPath: 'Inbox', snippet: 'matched snippet', attachmentCount: 2,
      },
      {
        key: 'two', subject: 'Second result', date: '2026-08-02', accountEmail: 'b@example.test',
        folderPath: '', snippet: '', attachmentCount: 0,
      },
    ],
    searchLoading: false,
    searchActiveIndex: 0,
    searchInputEl: null,
    messageAttachmentCount: (message) => message.attachmentCount,
    formatShortDate: (value) => `DATE:${value}`,
    ...handlers,
  })
  assert.match(target.textContent, /backupViewer\.unknownSubject/)
  assert.match(target.textContent, /matched snippet/)
  assert.match(target.textContent, /DATE:2026-08-01/)
  assert.ok(target.querySelector('[title="backupViewer.attachments"]'))

  const input = target.querySelector('input')
  input.dispatchEvent(new InputEvent('input', { bubbles: true }))
  input.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }))
  input.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true }))
  input.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
  assert.equal(handlers.onSearchInput.mock.calls.length, 1)
  assert.equal(handlers.onCompositionStart.mock.calls.length, 1)
  assert.equal(handlers.onCompositionEnd.mock.calls.length, 1)
  assert.equal(handlers.onSearchKeydown.mock.calls.length, 1)

  const results = target.querySelectorAll('[data-backup-viewer-search] .overflow-y-auto button')
  results[1].dispatchEvent(new MouseEvent('mousemove', { bubbles: true }))
  await flushAsync()
  assert.match(results[1].className, /bg-muted/)
  results[1].click()
  assert.deepEqual(handlers.onSelectSearchResult.mock.calls.at(-1), [1])

  const selectedScope = target.querySelector('[aria-pressed="true"]')
  selectedScope.click()
  assert.deepEqual(handlers.onSelectSearchScope.mock.calls.at(-1), [''])
  target.querySelector('[aria-label="common.close"]').click()
  assert.equal(handlers.onClose.mock.calls.length, 1)
})

test('backup search overlay covers closed, loading, and no-result states', async () => {
  const common = {
    accountScopes: [], searchScopeEmail: '', searchResults: [], searchActiveIndex: 0,
    searchInputEl: null, onClose: vi.fn(), onSelectSearchScope: vi.fn(), onSearchInput: vi.fn(),
    onCompositionStart: vi.fn(), onCompositionEnd: vi.fn(), onSearchKeydown: vi.fn(),
    onSelectSearchResult: vi.fn(), messageAttachmentCount: () => 0, formatShortDate: String,
  }
  const closed = await render(BackupViewerSearchOverlay, {
    ...common, open: false, searchQuery: '', searchLoading: false,
  })
  assert.equal(closed.textContent, '')

  const loading = await render(BackupViewerSearchOverlay, {
    ...common, open: true, searchQuery: 'pending', searchLoading: true,
  })
  assert.doesNotMatch(loading.textContent, /backupViewer\.searchNoResults/)

  const empty = await render(BackupViewerSearchOverlay, {
    ...common, open: true, searchQuery: 'missing', searchLoading: false,
  })
  assert.match(empty.textContent, /backupViewer\.searchNoResults/)
})
