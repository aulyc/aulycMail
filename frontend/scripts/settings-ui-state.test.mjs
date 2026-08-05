import assert from 'node:assert/strict'
import { beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => {
  const names = [
    'GetAccentBarUnread', 'GetAlwaysLoadImages', 'GetAutomaticUpdateChecks', 'GetAutostart', 'GetBackupSettings',
    'GetComposerFormat', 'GetDarkMailContent', 'GetDeveloperMode', 'GetEnhancedKeyboardNavigation',
    'GetLanguage', 'GetMarkAsReadDelay', 'GetMenuBarIcon', 'GetMessageListDensity',
    'GetMessageListSortOrder', 'GetNativeTitleBar', 'GetReadReceiptResponsePolicy',
    'GetRunBackground', 'GetStartHidden', 'GetThemeMode', 'GetUIState', 'SaveUIState',
    'SetAccentBarUnread', 'SetAlwaysLoadImages', 'SetAutomaticUpdateChecks', 'SetAutostart', 'SetBackupSettings',
    'SetComposerFormat', 'SetDarkMailContent', 'SetDeveloperMode', 'SetEnhancedKeyboardNavigation',
    'SetLanguage', 'SetMarkAsReadDelay', 'SetMenuBarIcon', 'SetMessageListDensity',
    'SetNativeTitleBar', 'SetReadReceiptResponsePolicy', 'SetRunBackground', 'SetStartHidden',
    'SetThemeMode',
  ]
  return Object.fromEntries(names.map((name) => [name, vi.fn()]))
})

const locale = vi.hoisted(() => ({
  set: vi.fn(),
  detect: vi.fn(() => 'en'),
  load: vi.fn(async () => {}),
  get: vi.fn((language) => ({ code: language })),
}))
const rememberBackupDirectory = vi.hoisted(() => vi.fn())

vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('$lib/i18n', () => ({ setLocale: locale.set, detectSystemLocale: locale.detect }))
vi.mock('$lib/i18n/dateFnsLocale', () => ({ loadDateFnsLocale: locale.load, getDateFnsLocale: locale.get }))
vi.mock('$lib/utils/backup-directory-history', () => ({ rememberBackupDirectory }))

import { SettingsDraft } from '../src/lib/components/settings/settingsDraft.svelte.ts'
import {
  getAccentBarUnread,
  getAlwaysLoadImages,
  getComposerFormat,
  getCurrentDateFnsLocale,
  getDarkMailContent,
  getDeveloperMode,
  getEnhancedKeyboardNavigation,
  getMessageListDensity,
  getMessageListSortOrder,
  getThemeMode,
  loadSettings,
  setAccentBarUnread,
  setAlwaysLoadImages,
  setComposerFormat,
  setDarkMailContent,
  setDeveloperMode,
  setEnhancedKeyboardNavigation,
  setLanguage,
  setMessageListDensity,
  setMessageListSortOrder,
  setThemeMode,
} from '../src/lib/stores/settings.svelte.ts'
import {
  DEFAULT_SIDEBAR_WIDTH,
  getActivePane,
  getUIState,
  getUIStateVersion,
  isAccountExpanded,
  loadUIState,
  saveUIState,
  setAccountExpanded,
  setActivePane,
  setFolderCollapsed,
} from '../src/lib/stores/uiState.svelte.ts'
import {
  getLayoutMode,
  getResponsiveView,
  hideSidebar,
  hideViewer,
  initLayout,
  isResponsive,
  showSidebar,
  showViewer,
} from '../src/lib/stores/layout.svelte.ts'
import {
  applyThemeFromMode,
  getIsDarkActive,
  handleMediaQueryChange,
  handleSystemThemeEvent,
  initTheme,
} from '../src/lib/stores/theme.svelte.ts'

beforeEach(() => {
  vi.clearAllMocks()
  for (const fn of Object.values(backend)) fn.mockResolvedValue(undefined)
  locale.detect.mockReturnValue('en')
  locale.get.mockImplementation((language) => ({ code: language }))
})

test('settings store setters normalize legacy density and expose every runtime flag', () => {
  setMessageListDensity('micro')
  setMessageListSortOrder('oldest')
  setThemeMode('light-blue')
  setLanguage('zh-CN')
  setComposerFormat('rich')
  setAlwaysLoadImages(true)
  setDarkMailContent(true)
  setAccentBarUnread(true)
  setDeveloperMode(true)
  setEnhancedKeyboardNavigation(false)

  assert.equal(getMessageListDensity(), 'compact')
  assert.equal(getMessageListSortOrder(), 'oldest')
  assert.equal(getThemeMode(), 'light-blue')
  assert.equal(getComposerFormat(), 'rich')
  assert.equal(getAlwaysLoadImages(), true)
  assert.equal(getDarkMailContent(), true)
  assert.equal(getAccentBarUnread(), true)
  assert.equal(getDeveloperMode(), true)
  assert.equal(getEnhancedKeyboardNavigation(), false)
  assert.equal(getCurrentDateFnsLocale().code, 'zh-CN')
  assert.deepEqual(locale.set.mock.calls[0], ['zh-CN'])
  assert.deepEqual(locale.load.mock.calls[0], ['zh-CN'])

  setLanguage('')
  assert.equal(getCurrentDateFnsLocale().code, 'en')
})

test('settings load applies backend values, defaults, locale, and the error fallback', async () => {
  backend.GetMessageListDensity.mockResolvedValue('micro')
  backend.GetMessageListSortOrder.mockResolvedValue('')
  backend.GetThemeMode.mockResolvedValue('dark')
  backend.GetLanguage.mockResolvedValue('zh-CN')
  backend.GetComposerFormat.mockResolvedValue('')
  backend.GetAlwaysLoadImages.mockResolvedValue(null)
  backend.GetDarkMailContent.mockResolvedValue(true)
  backend.GetAccentBarUnread.mockResolvedValue(undefined)
  backend.GetDeveloperMode.mockResolvedValue(true)
  backend.GetEnhancedKeyboardNavigation.mockResolvedValue(undefined)
  backend.GetAutomaticUpdateChecks.mockResolvedValue(true)

  assert.equal(await loadSettings(), 'dark')
  assert.equal(getMessageListDensity(), 'compact')
  assert.equal(getMessageListSortOrder(), 'newest')
  assert.equal(getComposerFormat(), 'plain')
  assert.equal(getAlwaysLoadImages(), false)
  assert.equal(getDarkMailContent(), true)
  assert.equal(getAccentBarUnread(), false)
  assert.equal(getDeveloperMode(), true)
  assert.equal(getEnhancedKeyboardNavigation(), true)
  assert.deepEqual(locale.set.mock.calls.at(-1), ['zh-CN'])

  backend.GetThemeMode.mockRejectedValue(new Error('settings unavailable'))
  const log = vi.spyOn(console, 'error').mockImplementation(() => {})
  assert.equal(await loadSettings(), 'system')
  log.mockRestore()
})

test('UI state load normalizes legacy values and save clamps widths before persistence', async () => {
  vi.useFakeTimers()
  try {
    const version = getUIStateVersion()
    backend.GetUIState.mockResolvedValue({
      selectedAccountId: 'account',
      selectedFolderId: 'inbox',
      selectedFolderName: '',
      selectedFolderType: '',
      selectedThreadId: '',
      selectedConversationAccountId: '',
      selectedConversationFolderId: '',
      sidebarWidth: 240,
      listWidth: 999,
      expandedAccounts: { account: false },
      unifiedInboxExpanded: false,
      collapsedFolders: { archive: true },
      activeExtension: 'contacts',
    })
    const loaded = await loadUIState()
    assert.equal(getUIStateVersion(), version + 1)
    assert.equal(loaded.sidebarWidth, DEFAULT_SIDEBAR_WIDTH)
    assert.equal(loaded.listWidth, 600)
    assert.equal(loaded.selectedFolderName, 'Inbox')
    assert.equal(loaded.selectedFolderType, 'inbox')
    assert.equal(loaded.unifiedInboxExpanded, false)
    assert.equal(isAccountExpanded('account'), false)
    assert.equal(isAccountExpanded('new-account'), true)
    assert.equal(getActivePane(), 'contacts')

    saveUIState({ sidebarWidth: 50, listWidth: 900 })
    setAccountExpanded('account', true)
    setFolderCollapsed('archive', false)
    setActivePane('')
    await vi.runAllTimersAsync()
    const saved = backend.SaveUIState.mock.calls.at(-1)[0]
    assert.equal(saved.sidebarWidth, 180)
    assert.equal(saved.listWidth, 600)
    assert.equal(saved.expandedAccounts.account, true)
    assert.equal(saved.collapsedFolders.archive, false)
    assert.equal(saved.activeExtension, 'mail')
    assert.equal(getUIState().activeExtension, 'mail')
    assert.equal(getActivePane(), 'mail')

    backend.SaveUIState.mockRejectedValue(new Error('save failed'))
    const log = vi.spyOn(console, 'error').mockImplementation(() => {})
    saveUIState({ selectedFolderId: null })
    await vi.runAllTimersAsync()
    log.mockRestore()
  } finally {
    vi.useRealTimers()
  }
})

test('UI state load failure preserves the current state and still signals completion', async () => {
  const before = getUIState()
  const version = getUIStateVersion()
  backend.GetUIState.mockRejectedValue(new Error('load failed'))
  const log = vi.spyOn(console, 'error').mockImplementation(() => {})
  const loaded = await loadUIState()
  log.mockRestore()
  assert.equal(loaded, before)
  assert.equal(getUIStateVersion(), version + 1)
})

test('layout initialization keeps the requested three-pane full layout at every width', () => {
  const listeners = []
  vi.stubGlobal('window', {
    matchMedia: vi.fn((query) => ({
      media: query,
      matches: true,
      addEventListener: (type, listener) => listeners.push([type, listener]),
    })),
  })
  try {
    initLayout()
    assert.equal(window.matchMedia.mock.calls.length, 2)
    assert.equal(listeners.length, 2)
    assert.equal(getLayoutMode(), 'full')
    assert.equal(getResponsiveView(), 'default')
    assert.equal(isResponsive(), false)
    showViewer()
    hideViewer()
    showSidebar()
    hideSidebar()
    assert.equal(getResponsiveView(), 'default')
  } finally {
    vi.unstubAllGlobals()
  }
})

test('theme resolution applies explicit, media, portal, and backend event choices safely', async () => {
  let currentTheme = ''
  const toggles = []
  const documentElement = {
    setAttribute: (name, value) => {
      if (name === 'data-theme') currentTheme = value
    },
    classList: { toggle: (name, active) => toggles.push([name, active]) },
  }
  vi.stubGlobal('document', { documentElement })
  vi.stubGlobal('getComputedStyle', () => ({
    colorScheme: ['dark', 'pop-dark', 'source-dark'].includes(currentTheme) ? 'dark' : 'light',
  }))
  vi.stubGlobal('window', { matchMedia: vi.fn(() => ({ matches: true })) })
  try {
    applyThemeFromMode('pop-dark')
    assert.equal(currentTheme, 'pop-dark')
    assert.equal(getIsDarkActive(), true)

    setThemeMode('system')
    applyThemeFromMode('system')
    assert.equal(currentTheme, 'dark')
    handleMediaQueryChange(false)
    assert.equal(currentTheme, 'light')

    await initTheme('system', async () => 'dark')
    assert.equal(currentTheme, 'dark')
    handleMediaQueryChange(false)
    assert.equal(currentTheme, 'dark')
    handleSystemThemeEvent('invalid')
    assert.equal(currentTheme, 'dark')
    handleSystemThemeEvent('light')
    assert.equal(currentTheme, 'light')

    setThemeMode('source-dark')
    handleSystemThemeEvent('dark')
    assert.equal(currentTheme, 'light')
    await initTheme('source-dark', async () => { throw new Error('portal missing') })
    assert.equal(currentTheme, 'source-dark')
    assert.equal(toggles.at(-1)[1], true)
  } finally {
    vi.unstubAllGlobals()
  }
})

test('settings draft loads normalized values and tracks ordinary and backup dirtiness', async () => {
  backend.GetReadReceiptResponsePolicy.mockResolvedValue('always')
  backend.GetMarkAsReadDelay.mockResolvedValue(-1)
  backend.GetMessageListDensity.mockResolvedValue('compact')
  backend.GetThemeMode.mockResolvedValue('unsupported-theme')
  backend.GetRunBackground.mockResolvedValue(0)
  backend.GetStartHidden.mockResolvedValue(1)
  backend.GetAutostart.mockResolvedValue(true)
  backend.GetLanguage.mockResolvedValue('zh-CN')
  backend.GetComposerFormat.mockResolvedValue('')
  backend.GetNativeTitleBar.mockResolvedValue(true)
  backend.GetAlwaysLoadImages.mockResolvedValue(1)
  backend.GetDarkMailContent.mockResolvedValue(0)
  backend.GetAccentBarUnread.mockResolvedValue(true)
  backend.GetMenuBarIcon.mockResolvedValue(false)
  backend.GetDeveloperMode.mockResolvedValue(true)
  backend.GetEnhancedKeyboardNavigation.mockResolvedValue(undefined)
  backend.GetBackupSettings.mockResolvedValue({
    directory: '/backup', scope: 'selected', selectedAccountIds: ['b', 'a'],
  })

  const draft = new SettingsDraft()
  await draft.load()
  assert.equal(draft.loading, false)
  assert.equal(draft.readReceiptResponsePolicy, 'always')
  assert.equal(draft.markAsReadDelaySeconds, -1)
  assert.equal(draft.themeMode, 'pop-dark')
  assert.equal(draft.startHidden, true)
  assert.equal(draft.composerFormat, 'rich')
  assert.equal(draft.enhancedKeyboardNavigation, true)
  assert.equal(draft.automaticUpdateChecks, true)
  assert.equal(draft.originalNativeTitleBar, true)
  assert.equal(draft.dirty, false)
  draft.backupSelectedAccountIds = ['a', 'b']
  assert.equal(draft.backupDirty, false)
  draft.themeMode = 'light-blue'
  assert.equal(draft.dirty, true)
})

test('settings draft save persists all values, enforces menu-bar background mode, and refreshes stores', async () => {
  const draft = new SettingsDraft()
  backend.GetReadReceiptResponsePolicy.mockResolvedValue('ask')
  backend.GetMarkAsReadDelay.mockResolvedValue(1000)
  backend.GetMessageListDensity.mockResolvedValue('standard')
  backend.GetThemeMode.mockResolvedValue('pop-dark')
  backend.GetRunBackground.mockResolvedValue(false)
  backend.GetStartHidden.mockResolvedValue(false)
  backend.GetAutostart.mockResolvedValue(false)
  backend.GetLanguage.mockResolvedValue('')
  backend.GetComposerFormat.mockResolvedValue('plain')
  backend.GetNativeTitleBar.mockResolvedValue(false)
  backend.GetAlwaysLoadImages.mockResolvedValue(false)
  backend.GetDarkMailContent.mockResolvedValue(false)
  backend.GetAccentBarUnread.mockResolvedValue(false)
  backend.GetMenuBarIcon.mockResolvedValue(false)
  backend.GetDeveloperMode.mockResolvedValue(false)
  backend.GetEnhancedKeyboardNavigation.mockResolvedValue(true)
  backend.GetAutomaticUpdateChecks.mockResolvedValue(true)
  backend.GetBackupSettings.mockResolvedValue({ directory: '', scope: 'all', selectedAccountIds: [] })
  await draft.load()

  draft.markAsReadDelaySeconds = 1.2346
  draft.messageListDensity = 'compact'
  draft.themeMode = 'light-blue'
  draft.runBackground = false
  draft.menuBarIcon = true
  draft.language = 'zh-CN'
  draft.composerFormat = 'rich'
  draft.alwaysLoadImages = true
  draft.darkMailContent = true
  draft.accentBarUnread = true
  draft.developerMode = true
  draft.enhancedKeyboardNavigation = false
  draft.automaticUpdateChecks = false
  draft.backupDirectory = ' /backup/new '
  draft.backupScope = 'selected'
  draft.backupSelectedAccountIds = ['account']
  await draft.saveAll()

  assert.deepEqual(backend.SetMarkAsReadDelay.mock.calls[0], [1235])
  assert.deepEqual(backend.SetRunBackground.mock.calls[0], [true])
  assert.deepEqual(backend.SetLanguage.mock.calls[0], ['zh-CN'])
  assert.deepEqual(backend.SetAutomaticUpdateChecks.mock.calls[0], [false])
  assert.deepEqual(backend.SetBackupSettings.mock.calls[0], [{
    directory: '/backup/new', scope: 'selected', selectedAccountIds: ['account'],
  }])
  assert.deepEqual(rememberBackupDirectory.mock.calls[0], ['/backup/new'])
  assert.equal(draft.runBackground, true)
  assert.equal(draft.saving, false)
  assert.equal(draft.dirty, false)
  assert.equal(getMessageListDensity(), 'compact')
  assert.equal(getThemeMode(), 'light-blue')
  assert.equal(getComposerFormat(), 'rich')
  assert.equal(getAlwaysLoadImages(), true)
  assert.equal(getDarkMailContent(), true)
  assert.equal(getAccentBarUnread(), true)
  assert.equal(getDeveloperMode(), true)
  assert.equal(getEnhancedKeyboardNavigation(), false)
})

test('settings draft keeps newer edits dirty while an earlier autosave is in flight', async () => {
  backend.GetReadReceiptResponsePolicy.mockResolvedValue('ask')
  backend.GetMarkAsReadDelay.mockResolvedValue(1000)
  backend.GetMessageListDensity.mockResolvedValue('standard')
  backend.GetThemeMode.mockResolvedValue('pop-dark')
  backend.GetRunBackground.mockResolvedValue(false)
  backend.GetStartHidden.mockResolvedValue(false)
  backend.GetAutostart.mockResolvedValue(false)
  backend.GetLanguage.mockResolvedValue('en')
  backend.GetComposerFormat.mockResolvedValue('rich')
  backend.GetNativeTitleBar.mockResolvedValue(false)
  backend.GetAlwaysLoadImages.mockResolvedValue(false)
  backend.GetDarkMailContent.mockResolvedValue(false)
  backend.GetAccentBarUnread.mockResolvedValue(false)
  backend.GetMenuBarIcon.mockResolvedValue(false)
  backend.GetDeveloperMode.mockResolvedValue(false)
  backend.GetEnhancedKeyboardNavigation.mockResolvedValue(true)
  backend.GetAutomaticUpdateChecks.mockResolvedValue(true)
  backend.GetBackupSettings.mockResolvedValue({ directory: '', scope: 'all', selectedAccountIds: [] })

  const draft = new SettingsDraft()
  await draft.load()
  let finishThemeSave
  backend.SetThemeMode.mockImplementationOnce(() => new Promise((resolve) => { finishThemeSave = resolve }))

  draft.themeMode = 'light-blue'
  const saving = draft.saveAll()
  await vi.waitFor(() => assert.equal(backend.SetThemeMode.mock.calls.length, 1))
  draft.themeMode = 'pop-dark'
  finishThemeSave()
  await saving

  assert.equal(draft.dirty, true)
})

test('settings draft leaves saving state clean when persistence fails', async () => {
  const draft = new SettingsDraft()
  backend.SetReadReceiptResponsePolicy.mockRejectedValue(new Error('write failed'))
  await assert.rejects(draft.saveAll(), /write failed/)
  assert.equal(draft.saving, false)
})
