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

test('describes user-managed credentials, no aulyc data server, and no telemetry connections', () => {
  assert.match(zhCN.settingsAbout.product.dataBody, /密码由用户自行保存和管理/)
  assert.match(zhCN.settingsAbout.product.dataBody, /aulyc 不设用于接收、存储或处理本应用数据的服务器/)
  assert.match(zhCN.settingsAbout.product.dataBody, /不建立任何用于遥测、统计分析、广告跟踪或向 aulyc 回传用户数据的连接/)
  assert.doesNotMatch(zhCN.settingsAbout.product.dataBody, /钥匙串/)
  assert.doesNotMatch(zhCN.settingsAbout.privacy.localAccount, /钥匙串/)
  assert.doesNotMatch(zhCN.settingsAbout.privacy.securityBody, /钥匙串/)

  assert.match(en.settingsAbout.product.dataBody, /saved and managed by the user/)
  assert.match(en.settingsAbout.product.dataBody, /does not operate servers that receive, store, or process application data/)
  assert.match(en.settingsAbout.product.dataBody, /does not establish connections for telemetry, analytics, advertising tracking, or sending user data back to aulyc/)
  assert.doesNotMatch(en.settingsAbout.product.dataBody, /keychain/i)
  assert.doesNotMatch(en.settingsAbout.privacy.localAccount, /keychain/i)
  assert.doesNotMatch(en.settingsAbout.privacy.securityBody, /keychain/i)
})

test('includes every visible unmasked acknowledgement from the supplied reference', () => {
  assert.deepEqual(
    [
      zhCN.settingsAbout.acknowledgements.aiEra,
      zhCN.settingsAbout.acknowledgements.aiCollaboration,
      zhCN.settingsAbout.acknowledgements.userContributions,
      zhCN.settingsAbout.acknowledgements.openSource,
      zhCN.settingsAbout.acknowledgements.aulycJourney,
    ],
    [
      '感谢伟大的 AI 时代，让更多想法得以更快成为现实',
      '致敬 Codex 与 Claude，在创作与开发中持续并肩协作',
      '感谢每一位提交需求、报告问题和提出改进建议的用户',
      '感谢开源社区与开发工具带来的启发和帮助',
      '感谢 aulyc 一路以来的坚持与灵感',
    ],
  )
})
