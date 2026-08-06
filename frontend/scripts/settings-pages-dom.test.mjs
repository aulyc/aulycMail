// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const activity = vi.hoisted(() => ({ instances: [] }))

vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key) => key)
      return () => {}
    },
  },
  supportedLocales: [
    { code: 'en', name: 'English' },
    { code: 'zh-CN', name: '简体中文' },
  ],
}))
vi.mock('@iconify/svelte', async () => ({
  default: (await import('./fixtures/StaticStub.svelte')).default,
}))
vi.mock('$lib/components/ui/select', async () => {
  const root = (await import('./fixtures/SettingsSelectActionRootTestStub.svelte')).default
  const snippet = (await import('./fixtures/SnippetTestStub.svelte')).default
  const item = (await import('./fixtures/SelectItemTestStub.svelte')).default
  return { Root: root, Trigger: snippet, Value: snippet, Content: snippet, Item: item }
})
vi.mock('$lib/components/ui/bool-select/BoolSelect.svelte', async () => ({
  default: (await import('./fixtures/SwitchTestStub.svelte')).default,
}))
vi.mock('../src/lib/components/settings/ImagesTab.svelte', async () => ({
  default: (await import('./fixtures/StaticStub.svelte')).default,
}))
vi.mock('../src/lib/components/settings/activity/activityLogs.svelte', () => ({
  ActivityLogsStore: class {
    type = ''
    start = vi.fn()
    refresh = vi.fn().mockResolvedValue(undefined)
    stop = vi.fn()
    constructor() { activity.instances.push(this) }
  },
}))
vi.mock('../src/lib/components/settings/activity/ActivityLogFilters.svelte', async () => ({
  default: (await import('./fixtures/StaticStub.svelte')).default,
}))
vi.mock('../src/lib/components/settings/activity/ActivityLogList.svelte', async () => ({
  default: (await import('./fixtures/StaticStub.svelte')).default,
}))

import SettingsPagesHarness from './fixtures/SettingsPagesHarness.svelte'
import ActivityLogPage from '../src/lib/components/settings/activity/ActivityLogPage.svelte'

const mounted = []

async function flushAsync() {
  for (let index = 0; index < 6; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function renderPage(page) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(SettingsPagesHarness, { target, props: { page } })
  mounted.push(instance)
  await flushAsync()
  return target
}

function draftState(target) {
  return target.querySelector('[data-settings-draft]').dataset
}

function switchFor(target, label) {
  const row = target.querySelector(`[data-keyboard-action-context="${label}"]`)
  assert.ok(row, `missing row ${label}`)
  return row.querySelector('[data-switch]')
}

function changeSwitch(input, checked) {
  input.checked = checked
  input.dispatchEvent(new Event('change', { bubbles: true }))
}

beforeEach(() => {
  document.body.innerHTML = ''
  activity.instances.length = 0
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
})

test('general settings keep background and menu-bar requirements consistent', async () => {
  const target = await renderPage('general')
  const language = target.querySelector('[data-settings-select][data-value="en"]')
  language.querySelector('[data-settings-select-change]').click()
  await flushAsync()
  assert.equal(draftState(target).language, 'zh-CN')

  changeSwitch(switchFor(target, 'settingsGeneral.runInBackground'), false)
  await flushAsync()
  assert.equal(draftState(target).runBackground, 'false')
  assert.equal(draftState(target).menuBarIcon, 'false')

  changeSwitch(switchFor(target, 'settingsGeneral.menuBarIcon'), true)
  await flushAsync()
  assert.equal(draftState(target).menuBarIcon, 'true')
  assert.equal(draftState(target).runBackground, 'true')

  changeSwitch(switchFor(target, 'settingsGeneral.autostartOnLogin'), true)
  changeSwitch(switchFor(target, 'settingsGeneral.enhancedKeyboardNavigation'), false)
  changeSwitch(switchFor(target, 'settingsGeneral.developerMode'), true)
  await flushAsync()
  assert.equal(draftState(target).autostart, 'true')
  assert.equal(draftState(target).enhancedKeyboardNavigation, 'false')
  assert.equal(draftState(target).developerMode, 'true')
})

test('general settings help opens on the trigger right with vertical centers aligned', async () => {
  const target = await renderPage('general')
  const trigger = target.querySelector('[data-settings-help-trigger]')
  assert.ok(trigger)
  trigger.click()
  await flushAsync()

  const popover = document.body.querySelector('[data-settings-help-popover]')
  assert.ok(popover)
  assert.equal(popover.dataset.side, 'right')
  assert.equal(popover.dataset.align, 'center')
})

test('appearance settings update theme, mail rendering, accent, and density drafts', async () => {
  const target = await renderPage('appearance')
  target.querySelector('[data-settings-select][data-value="pop-dark"] [data-settings-select-change]').click()
  await flushAsync()
  assert.equal(draftState(target).themeMode, 'light-blue')
  assert.equal(switchFor(target, 'settingsGeneral.darkMailContent').disabled, true)

  changeSwitch(switchFor(target, 'settingsGeneral.accentBarUnread'), true)
  target.querySelector('[data-settings-select][data-value="standard"] [data-settings-select-change]').click()
  await flushAsync()
  assert.equal(draftState(target).darkMailContent, 'true')
  assert.equal(draftState(target).accentBarUnread, 'true')
  assert.equal(draftState(target).density, 'large')
})

test('mail settings update composer and receipt-policy drafts', async () => {
  const target = await renderPage('mail')
  target.querySelector('[data-settings-select][data-value="rich"] [data-settings-select-change]').click()
  target.querySelector('[data-settings-select][data-value="ask"] [data-settings-select-change]').click()
  await flushAsync()
  assert.equal(draftState(target).composerFormat, 'plain')
  assert.equal(draftState(target).receiptPolicy, 'always')
  assert.ok(target.querySelector('[data-stub="static-component"]'))
})

test('activity page applies its initial filter and owns the log-listener lifecycle', async () => {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(ActivityLogPage, { target, props: { initialType: 'backup' } })
  mounted.push(instance)
  await flushAsync()
  const store = activity.instances.at(-1)
  assert.equal(store.type, 'backup')
  assert.equal(store.start.mock.calls.length, 1)
  assert.equal(store.refresh.mock.calls.length, 1)
  assert.match(target.textContent, /activityLog\.description/)

  await unmount(mounted.pop())
  assert.equal(store.stop.mock.calls.length, 1)
})
