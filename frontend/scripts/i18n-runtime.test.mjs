// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { beforeEach, test, vi } from 'vitest'

const runtime = vi.hoisted(() => ({
  register: vi.fn(),
  init: vi.fn(),
  waitLocale: vi.fn(() => Promise.resolve()),
  localeSet: vi.fn(),
  registerContacts: vi.fn(),
}))

vi.mock('svelte-i18n', () => ({
  register: runtime.register,
  init: runtime.init,
  waitLocale: runtime.waitLocale,
  locale: { set: runtime.localeSet },
  _: { subscribe: () => () => {} },
}))
vi.mock('$contacts/i18n', () => ({ registerContactsI18n: runtime.registerContacts }))

import {
  detectSystemLocale,
  initI18n,
  setLocale,
  supportedLocales,
} from '../src/lib/i18n/index.ts'
import {
  getDateFnsLocale,
  loadDateFnsLocale,
} from '../src/lib/i18n/dateFnsLocale.ts'

const zhCN = JSON.parse(readFileSync(resolve(process.cwd(), 'src/lib/i18n/locales/zh-CN.json'), 'utf8'))
const en = JSON.parse(readFileSync(resolve(process.cwd(), 'src/lib/i18n/locales/en.json'), 'utf8'))

function setNavigatorLanguage(language) {
  Object.defineProperty(navigator, 'language', { configurable: true, value: language })
}

beforeEach(() => {
  for (const mock of Object.values(runtime)) mock.mockClear?.()
})

test('detects exact, regional, language-only, missing, and unsupported locales', () => {
  assert.deepEqual(supportedLocales.map(({ code }) => code), ['en', 'zh-CN'])
  for (const [language, expected] of [
    ['en', 'en'],
    ['ZH-cn', 'zh-CN'],
    ['zh-TW', 'zh-CN'],
    ['en-US', 'en'],
    ['fr-FR', 'en'],
    ['', 'en'],
  ]) {
    setNavigatorLanguage(language)
    assert.equal(detectSystemLocale(), expected, language)
  }
})

test('initializes saved and detected locales and updates the runtime store', async () => {
  setNavigatorLanguage('zh-HK')
  await initI18n()
  assert.equal(runtime.registerContacts.mock.calls.length, 1)
  assert.deepEqual(runtime.init.mock.calls[0], [{ fallbackLocale: 'en', initialLocale: 'zh-CN' }])
  assert.equal(runtime.waitLocale.mock.calls.length, 1)
  assert.ok(getDateFnsLocale('zh-CN'))

  await initI18n('en')
  assert.deepEqual(runtime.init.mock.calls[1], [{ fallbackLocale: 'en', initialLocale: 'en' }])
  setLocale('zh-CN')
  assert.deepEqual(runtime.localeSet.mock.calls, [['zh-CN']])
})

test('date-fns locale loading caches Chinese and ignores English and unknown locales', async () => {
  assert.equal(await loadDateFnsLocale('en'), undefined)
  const first = await loadDateFnsLocale('zh-CN')
  const second = await loadDateFnsLocale('zh-CN')
  assert.ok(first)
  assert.equal(second, first)
  assert.equal(getDateFnsLocale('zh-CN'), first)
  assert.equal(await loadDateFnsLocale('fr-FR'), undefined)
  assert.equal(getDateFnsLocale('fr-FR'), undefined)
})

test('uses user-facing update installation copy without exposing internal verification steps', () => {
  assert.equal(zhCN.settingsUpdate.systemUpdate, '软件更新')
  assert.equal(zhCN.settingsUpdate.availableCompact, '发现新版本')
  assert.equal(zhCN.settingsAbout.version, '软件版本 {version}')
  assert.equal(zhCN.settingsAbout.versionTitle, '软件版本')
  assert.equal(
    zhCN.settingsUpdate.confirmDescription,
    '将下载并安装 aulycMail {version}。更新完成后，应用将自动重新启动。',
  )
  assert.equal(
    en.settingsUpdate.confirmDescription,
    'aulycMail {version} will be downloaded and installed. The app will restart automatically when the update is complete.',
  )
})
