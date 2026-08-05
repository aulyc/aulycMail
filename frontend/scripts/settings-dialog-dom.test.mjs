// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => {
  const names = [
    'GetAppInfo', 'QuitApp',
    'GetAccentBarUnread', 'GetAlwaysLoadImages', 'GetAutomaticUpdateChecks', 'GetAutostart', 'GetBackupSettings',
    'GetComposerFormat', 'GetDarkMailContent', 'GetDeveloperMode', 'GetEnhancedKeyboardNavigation',
    'GetLanguage', 'GetMarkAsReadDelay', 'GetMenuBarIcon', 'GetMessageListDensity',
    'GetNativeTitleBar', 'GetReadReceiptResponsePolicy', 'GetRunBackground', 'GetStartHidden',
    'GetThemeMode', 'SetAccentBarUnread', 'SetAlwaysLoadImages', 'SetAutostart',
    'SetAutomaticUpdateChecks', 'SetBackupSettings', 'SetComposerFormat', 'SetDarkMailContent', 'SetDeveloperMode',
    'SetEnhancedKeyboardNavigation', 'SetLanguage', 'SetMarkAsReadDelay', 'SetMenuBarIcon',
    'SetMessageListDensity', 'SetNativeTitleBar', 'SetReadReceiptResponsePolicy',
    'SetRunBackground', 'SetStartHidden', 'SetThemeMode',
  ]
  return Object.fromEntries(names.map((name) => [name, vi.fn()]))
})
const toast = vi.hoisted(() => ({ add: vi.fn() }))
const guards = vi.hoisted(() => ({ open: vi.fn(), close: vi.fn() }))
const keyboard = vi.hoisted(() => ({ scope: 'main', enhanced: true, setScope: vi.fn() }))
const actionMenu = vi.hoisted(() => ({ showForRoot: vi.fn() }))
const storeUpdates = vi.hoisted(() => ({
  density: vi.fn(), theme: vi.fn(), language: vi.fn(), composer: vi.fn(),
  images: vi.fn(), dark: vi.fn(), accent: vi.fn(), developer: vi.fn(), enhanced: vi.fn(),
}))

vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('$lib/stores/toast', () => ({ addToast: toast.add }))
vi.mock('$lib/stores/dialogGuard', () => ({
  dialogGuardOpen: guards.open,
  dialogGuardClose: guards.close,
}))
vi.mock('$lib/stores/keyboard.svelte', () => ({
  setKeyboardScope: (scope) => {
    keyboard.scope = scope
    keyboard.setScope(scope)
  },
  isInputElement: (target) => target instanceof HTMLElement && Boolean(target.closest('input, textarea, select, [contenteditable="true"], [role="combobox"]')),
}))
vi.mock('$lib/stores/settings.svelte', () => ({
  getEnhancedKeyboardNavigation: () => keyboard.enhanced,
  setMessageListDensity: storeUpdates.density,
  setThemeMode: storeUpdates.theme,
  setLanguage: storeUpdates.language,
  setComposerFormat: storeUpdates.composer,
  setAlwaysLoadImages: storeUpdates.images,
  setDarkMailContent: storeUpdates.dark,
  setAccentBarUnread: storeUpdates.accent,
  setDeveloperMode: storeUpdates.developer,
  setEnhancedKeyboardNavigation: storeUpdates.enhanced,
}))
vi.mock('$lib/stores/keyboardActionMenu.svelte', () => ({ keyboardActionMenu: actionMenu }))
vi.mock('$lib/utils/backup-directory-history', () => ({ rememberBackupDirectory: vi.fn() }))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('@iconify/svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))
vi.mock('$lib/components/ui/dialog', async () => ({
  Root: (await import('./fixtures/SettingsDialogRootTestStub.svelte')).default,
  Content: (await import('./fixtures/DialogContentTestStub.svelte')).default,
  Title: (await import('./fixtures/SnippetTestStub.svelte')).default,
}))
vi.mock('$lib/components/ui/confirm-dialog/ConfirmDialog.svelte', async () => ({
  default: (await import('./fixtures/ConfirmDialogTestStub.svelte')).default,
}))

vi.mock('../src/lib/components/settings/pages/GeneralSettingsPage.svelte', async () => ({ default: (await import('./fixtures/SettingsPageTestStub.svelte')).default }))
vi.mock('../src/lib/components/settings/pages/AppearanceSettingsPage.svelte', async () => ({ default: (await import('./fixtures/SettingsPageTestStub.svelte')).default }))
vi.mock('../src/lib/components/settings/pages/MailSettingsPage.svelte', async () => ({ default: (await import('./fixtures/SettingsPageTestStub.svelte')).default }))
vi.mock('../src/lib/components/settings/pages/AccountsSettingsPage.svelte', async () => ({ default: (await import('./fixtures/SettingsPageTestStub.svelte')).default }))
vi.mock('../src/lib/components/settings/backup/BackupSettingsPage.svelte', async () => ({ default: (await import('./fixtures/SettingsPageTestStub.svelte')).default }))
vi.mock('../src/lib/components/settings/activity/ActivityLogPage.svelte', async () => ({ default: (await import('./fixtures/SettingsPageTestStub.svelte')).default }))
vi.mock('../src/lib/components/settings/AboutTab.svelte', async () => ({ default: (await import('./fixtures/SettingsPageTestStub.svelte')).default }))

import SettingsDialog from '../src/lib/components/settings/SettingsDialog.svelte'

const mounted = []

function setBackendDefaults() {
  for (const fn of Object.values(backend)) fn.mockReset().mockResolvedValue(undefined)
  backend.GetAppInfo.mockResolvedValue({ version: '0.6.0-test', build: 42 })
  backend.GetReadReceiptResponsePolicy.mockResolvedValue('ask')
  backend.GetMarkAsReadDelay.mockResolvedValue(1000)
  backend.GetMessageListDensity.mockResolvedValue('standard')
  backend.GetThemeMode.mockResolvedValue('pop-dark')
  backend.GetRunBackground.mockResolvedValue(false)
  backend.GetStartHidden.mockResolvedValue(false)
  backend.GetAutostart.mockResolvedValue(false)
  backend.GetLanguage.mockResolvedValue('')
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
}

async function flushAsync() {
  for (let index = 0; index < 8; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function renderDialog(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(SettingsDialog, {
    target,
    props: { open: true, activePage: 'general', ...props },
  })
  mounted.push(instance)
  await flushAsync()
  return { instance, target }
}

function pageButton(target, page) {
  const button = target.querySelector(`[data-settings-page="${page}"]`)
  assert.ok(button, `missing settings page ${page}`)
  return button
}

function key(target, value, options = {}) {
  const event = new KeyboardEvent('keydown', {
    key: value,
    bubbles: true,
    cancelable: true,
    ...options,
  })
  target.dispatchEvent(event)
  return event
}

beforeEach(() => {
  document.body.innerHTML = ''
  setBackendDefaults()
  keyboard.scope = 'main'
  keyboard.enhanced = true
  keyboard.setScope.mockReset()
  toast.add.mockReset()
  guards.open.mockReset()
  guards.close.mockReset()
  actionMenu.showForRoot.mockReset()
  for (const fn of Object.values(storeUpdates)) fn.mockReset()
  vi.spyOn(window, 'requestAnimationFrame').mockImplementation((callback) => {
    callback(performance.now())
    return 1
  })
  if (!HTMLElement.prototype.scrollIntoView) HTMLElement.prototype.scrollIntoView = vi.fn()
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('loads every draft value, navigates by keyboard, saves, and requests restart', async () => {
  const { target } = await renderDialog()
  assert.equal(backend.GetAppInfo.mock.calls.length, 1)
  assert.equal(backend.GetBackupSettings.mock.calls.length, 1)
  assert.equal(keyboard.scope, 'settings')
  assert.equal(guards.open.mock.calls.length, 1)
  assert.match(target.textContent, /0\.6\.0-test/)

  const navigation = target.querySelector('[data-settings-region="navigation"]')
  navigation.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true }))
  navigation.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.equal(pageButton(target, 'appearance').getAttribute('aria-selected'), 'true')

  navigation.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }))
  await flushAsync()
  const content = target.querySelector('[data-settings-region="content"]')
  assert.equal(document.activeElement, content)
  const initial = target.querySelector('[data-settings-control-id="toggle-native-title"]')
  assert.equal(initial.dataset.settingsKeyboardSelected, 'true')

  content.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.match(target.textContent, /settings\.unsavedChanges/)
  const save = target.querySelector('[data-settings-footer-action="save"]')
  assert.equal(save.disabled, false)
  save.click()
  await flushAsync()
  await vi.waitFor(() => assert.ok(toast.add.mock.calls.length > 0))

  assert.deepEqual(backend.SetNativeTitleBar.mock.calls.at(-1), [true])
  assert.match(toast.add.mock.calls.at(-1)[0].message, /toast\.settingsSaved/)
  const restart = target.querySelector('[data-confirm-dialog]')
  assert.ok(restart)
  restart.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.equal(backend.QuitApp.mock.calls.length, 1)
})

test('covers page callbacks, horizontal navigation, input mode, and action-menu routing', async () => {
  const { target } = await renderDialog()
  pageButton(target, 'backup').click()
  await flushAsync()
  target.querySelector('[data-test-action="open-activity"]').click()
  await flushAsync()
  assert.equal(pageButton(target, 'activity').getAttribute('aria-selected'), 'true')
  assert.equal(target.querySelector('[data-settings-page-stub]').dataset.initialType, 'backup')

  pageButton(target, 'accounts').click()
  await flushAsync()
  const content = target.querySelector('[data-settings-region="content"]')
  content.focus()
  await flushAsync()
  target.querySelector('[data-test-action="order-changed"]').click()
  await flushAsync()
  assert.equal(target.querySelector('[data-settings-horizontal-action="move-down"]').dataset.settingsKeyboardSelected, 'true')

  content.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.equal(target.querySelector('[data-settings-horizontal-action="move-up"]').dataset.settingsKeyboardSelected, 'true')
  content.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true }))
  await flushAsync()

  content.dispatchEvent(new KeyboardEvent('keydown', {
    key: 'F10', shiftKey: true, bubbles: true, cancelable: true,
  }))
  assert.equal(actionMenu.showForRoot.mock.calls.length, 1)

  const input = target.querySelector('input')
  input.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
  input.focus()
  input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.equal(document.activeElement, content)
})

test('reports load and save failures and restores main scope when closed', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.GetAppInfo.mockRejectedValueOnce(new Error('app info unavailable'))
  backend.GetThemeMode.mockRejectedValueOnce(new Error('settings unavailable'))
  let rendered = await renderDialog()
  assert.ok(toast.add.mock.calls.some(([entry]) => entry.message === 'toast.failedToLoadSettings'))
  assert.match(rendered.target.textContent, /version[^0-9]*—/i)

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  setBackendDefaults()
  backend.SetReadReceiptResponsePolicy.mockRejectedValueOnce(new Error('write unavailable'))
  const onClose = vi.fn()
  rendered = await renderDialog({ onClose })
  rendered.target.querySelector('[data-settings-control-id="toggle-native-title"]').click()
  await flushAsync()
  rendered.target.querySelector('[data-settings-footer-action="save"]').click()
  await flushAsync()
  assert.ok(toast.add.mock.calls.some(([entry]) => entry.message === 'toast.failedToSaveSettings'))
  assert.equal(rendered.target.querySelector('[data-confirm-dialog]'), null)

  rendered.target.querySelector('[data-settings-footer-action="cancel"]').click()
  await flushAsync()
  assert.equal(onClose.mock.calls.length, 1)
  assert.equal(keyboard.scope, 'main')
  assert.ok(guards.close.mock.calls.length >= 1)
  assert.ok(error.mock.calls.length >= 2)
})

test('restores activity pagination selection and honors pointer-focus and root-dismiss behavior', async () => {
  const onClose = vi.fn()
  const { target } = await renderDialog({ activePage: 'activity', onClose })
  const content = target.querySelector('[data-settings-region="content"]')
  content.focus()
  await flushAsync()

  target.querySelector('[data-settings-control-id="activity-load-more"]').click()
  await flushAsync()
  assert.equal(
    target.querySelector('[data-settings-control-id="activity-load-more"]').dataset.settingsKeyboardSelected,
    'true',
  )

  target.querySelector('[data-test-action="load-finished"]').click()
  await flushAsync()
  assert.equal(target.querySelector('[data-settings-footer-action="close"]').dataset.settingsKeyboardSelected, 'true')
  assert.equal(document.activeElement, content)

  const dialog = target.querySelector('[role="dialog"]')
  const backgroundPointer = new PointerEvent('pointerdown', { button: 0, bubbles: true, cancelable: true })
  dialog.dispatchEvent(backgroundPointer)
  assert.equal(backgroundPointer.defaultPrevented, true)
  const secondaryPointer = new PointerEvent('pointerdown', { button: 2, bubbles: true, cancelable: true })
  dialog.dispatchEvent(secondaryPointer)
  assert.equal(secondaryPointer.defaultPrevented, false)
  const buttonPointer = new PointerEvent('pointerdown', { button: 0, bubbles: true, cancelable: true })
  target.querySelector('[data-settings-footer-action="close"]').dispatchEvent(buttonPointer)
  assert.equal(buttonPointer.defaultPrevented, false)

  target.querySelector('[data-dialog-root-dismiss]').click()
  await flushAsync()
  assert.equal(onClose.mock.calls.length, 1)
  assert.equal(target.querySelector('[role="dialog"]'), null)
})

test('moves among content, footer, explicit, adjacent, fallback, and outside horizontal targets', async () => {
  const { target } = await renderDialog()
  const navigation = target.querySelector('[data-settings-region="navigation"]')
  const content = target.querySelector('[data-settings-region="content"]')
  key(navigation, 'Tab')
  await flushAsync()

  target.querySelector('[data-settings-control-id="toggle-native-title"]').click()
  await flushAsync()
  assert.equal(target.querySelectorAll('[data-settings-footer-action]:not(:disabled)').length, 2)
  key(content, 'End')
  assert.equal(target.querySelector('[data-settings-footer-action="save"]').dataset.settingsKeyboardSelected, 'true')
  const saveAction = target.querySelector('[data-settings-footer-action="save"]')
  saveAction.focus()
  await flushAsync()
  const footerLeft = key(saveAction, 'ArrowLeft')
  assert.equal(footerLeft.defaultPrevented, true)
  const footerRight = key(target.querySelector('[data-settings-footer-action="cancel"]'), 'ArrowRight')
  assert.equal(footerRight.defaultPrevented, true)
  const footerUp = key(saveAction, 'ArrowUp')
  assert.equal(footerUp.defaultPrevented, true)
  const footerDown = key(saveAction, 'ArrowDown')
  assert.equal(footerDown.defaultPrevented, true)

  const firstGroup = target.querySelector('[data-settings-horizontal-context="synthetic-account"]')
  firstGroup.querySelector('[data-settings-horizontal-action="move-up"]').focus()
  await flushAsync()
  key(content, 'ArrowDown')
  assert.equal(target.querySelector('[data-settings-horizontal-action="first"]').dataset.settingsKeyboardSelected, 'true')

  const explicit = target.querySelector('[data-settings-horizontal-action="first"]')
  explicit.focus()
  key(content, 'ArrowUp')
  assert.equal(target.querySelector('[data-settings-control-id="activity-load-more"]').dataset.settingsKeyboardSelected, 'true')
  explicit.focus()
  key(content, 'ArrowDown')
  assert.equal(target.querySelector('[data-settings-footer-action="save"]').dataset.settingsKeyboardSelected, 'true')

  const outside = target.querySelector('[data-settings-horizontal-action="outside"]')
  outside.focus()
  key(content, 'ArrowDown')
  assert.ok(target.querySelector('[data-settings-keyboard-selected="true"]'))
  assert.notEqual(target.querySelector('[data-settings-keyboard-selected="true"]'), outside)
})

test('tracks select open/close lifecycle and leaves an open popup in input mode', async () => {
  const { target } = await renderDialog()
  const navigation = target.querySelector('[data-settings-region="navigation"]')
  const content = target.querySelector('[data-settings-region="content"]')
  key(navigation, 'Tab')
  await flushAsync()
  const select = target.querySelector('[data-settings-control-id="select-trigger"]')
  select.focus()
  await flushAsync()
  key(content, 'Enter')
  await flushAsync()
  assert.equal(select.getAttribute('aria-expanded'), 'true')
  assert.equal(document.activeElement, select)

  const escapeWhileOpen = key(select, 'Escape')
  assert.equal(escapeWhileOpen.defaultPrevented, false)
  assert.equal(document.activeElement, select)

  target.querySelector('[data-test-action="close-select"]').click()
  await flushAsync()
  assert.equal(select.getAttribute('aria-expanded'), 'false')
  assert.equal(document.activeElement, content)
  assert.equal(select.dataset.settingsKeyboardSelected, 'true')
})

test('disables enhanced navigation paths while preserving pointer actions', async () => {
  keyboard.enhanced = false
  const { target } = await renderDialog({ activePage: 'accounts' })
  const navigation = target.querySelector('[data-settings-region="navigation"]')
  const content = target.querySelector('[data-settings-region="content"]')
  assert.equal(navigation.dataset.regionActive, 'false')
  content.focus()
  content.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true }))
  key(content, 'ArrowDown')
  assert.equal(target.querySelector('[data-settings-keyboard-selected]'), null)

  target.querySelector('[data-test-action="order-changed"]').click()
  await flushAsync()
  assert.equal(document.activeElement, target.querySelector('[data-settings-horizontal-action="move-down"]'))
  assert.equal(actionMenu.showForRoot.mock.calls.length, 0)
})
