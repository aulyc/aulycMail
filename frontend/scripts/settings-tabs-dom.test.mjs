// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  GetImageAllowlist: vi.fn(),
  RemoveImageAllowlist: vi.fn(),
}))
const runtime = vi.hoisted(() => ({ openURL: vi.fn() }))
const accounts = vi.hoisted(() => ({
  accounts: [],
  loading: false,
  getAccountConnOK: vi.fn(),
  testAccountConnection: vi.fn(),
  reorderAccounts: vi.fn(),
}))
const imageStore = vi.hoisted(() => ({ refresh: vi.fn() }))
const toast = vi.hoisted(() => ({ add: vi.fn() }))

vi.mock('../wailsjs/go/app/App', () => backend)
vi.mock('../wailsjs/runtime/runtime', () => ({ BrowserOpenURL: runtime.openURL }))
vi.mock('$lib/stores/accounts.svelte', () => ({ accountStore: accounts }))
vi.mock('$lib/stores/settings.svelte', () => ({ getCurrentDateFnsLocale: () => undefined }))
vi.mock('$lib/stores/imageAllowlist.svelte', () => ({ refreshImageAllowlist: imageStore.refresh }))
vi.mock('$lib/stores/toast', () => ({ addToast: toast.add }))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('@iconify/svelte', async () => ({
  default: (await import('./fixtures/StaticStub.svelte')).default,
}))
vi.mock('$lib/components/ui/bool-select/BoolSelect.svelte', async () => ({
  default: (await import('./fixtures/SwitchTestStub.svelte')).default,
}))
vi.mock('$lib/components/ui/confirm-dialog/ConfirmDialog.svelte', async () => ({
  default: (await import('./fixtures/ConfirmDialogTestStub.svelte')).default,
}))
vi.mock('$lib/components/ui/dialog', async () => {
  const root = (await import('./fixtures/DialogRootTestStub.svelte')).default
  const content = (await import('./fixtures/DialogContentTestStub.svelte')).default
  const snippet = (await import('./fixtures/SnippetTestStub.svelte')).default
  return { Root: root, Content: content, Header: snippet, Title: snippet }
})
vi.mock('../src/lib/components/settings/AccountDialog.svelte', async () => ({
  default: (await import('./fixtures/AccountDialogTestStub.svelte')).default,
}))
vi.mock('../src/lib/components/settings/DeleteAccountDialog.svelte', async () => ({
  default: (await import('./fixtures/DeleteAccountDialogTestStub.svelte')).default,
}))
vi.mock('../src/lib/components/settings/ConnectionTestDialog.svelte', async () => ({
  default: (await import('./fixtures/ConnectionTestDialogTestStub.svelte')).default,
}))

import AboutTab from '../src/lib/components/settings/AboutTab.svelte'
import AccountsTab from '../src/lib/components/settings/AccountsTab.svelte'
import ImagesTab from '../src/lib/components/settings/ImagesTab.svelte'

const mounted = []

async function flushAsync() {
  for (let index = 0; index < 8; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function render(component, props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(component, { target, props })
  mounted.push(instance)
  await flushAsync()
  return { target, instance }
}

function buttonWithText(target, text) {
  const button = [...target.querySelectorAll('button')].find((item) => item.textContent.includes(text))
  assert.ok(button, `missing button containing ${text}`)
  return button
}

function titledButtons(target, title) {
  return [...target.querySelectorAll(`button[title="${title}"]`)]
}

beforeEach(() => {
  document.body.innerHTML = ''
  backend.GetImageAllowlist.mockReset().mockResolvedValue([
    { id: 'sender-1', type: 'sender', value: 'alice@example.test' },
    { id: 'domain-1', type: 'domain', value: 'example.test' },
  ])
  backend.RemoveImageAllowlist.mockReset().mockResolvedValue(undefined)
  runtime.openURL.mockReset()
  accounts.accounts = [
    { account: { id: 'a', name: 'A', email: 'a@example.test' } },
    { account: { id: 'shared', name: 'Shared', email: 'shared@example.test', sharedMailboxParentId: 'a' } },
    { account: { id: 'b', name: 'B', email: 'b@example.test' } },
  ]
  accounts.loading = false
  accounts.getAccountConnOK.mockReset().mockImplementation(async (id) => id === 'a' ? '2026-08-01T03:04:05Z' : '')
  accounts.testAccountConnection.mockReset().mockResolvedValue({ success: true })
  accounts.reorderAccounts.mockReset().mockResolvedValue(undefined)
  imageStore.refresh.mockReset()
  toast.add.mockReset()
  vi.spyOn(window, 'requestAnimationFrame').mockImplementation((callback) => {
    callback(performance.now())
    return 1
  })
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('image settings load, collapse, confirm removal, and refresh visible entries', async () => {
  const onAlwaysLoadImagesChange = vi.fn()
  const { target } = await render(ImagesTab, {
    alwaysLoadImages: false,
    onAlwaysLoadImagesChange,
  })
  assert.match(target.textContent, /alice@example\.test/)
  assert.match(target.textContent, /example\.test/)

  buttonWithText(target, 'images.addresses').click()
  await flushAsync()
  assert.doesNotMatch(target.textContent, /alice@example\.test/)
  buttonWithText(target, 'images.addresses').click()
  await flushAsync()

  backend.GetImageAllowlist.mockResolvedValueOnce([
    { id: 'domain-1', type: 'domain', value: 'example.test' },
  ])
  titledButtons(target, 'images.removeButton')[0].click()
  await flushAsync()
  assert.match(target.textContent, /images\.removeConfirmDescription/)
  target.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.deepEqual(backend.RemoveImageAllowlist.mock.calls.at(-1), ['sender-1'])
  assert.doesNotMatch(target.textContent, /alice@example\.test/)
  assert.equal(imageStore.refresh.mock.calls.length, 1)
  assert.equal(toast.add.mock.calls.at(-1)[0].type, 'success')

  const switchInput = target.querySelector('[data-switch]')
  switchInput.checked = true
  switchInput.dispatchEvent(new Event('change', { bubbles: true }))
  await flushAsync()
  assert.match(target.textContent, /settingsGeneral\.alwaysLoadImagesWarningTitle/)
  target.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.deepEqual(onAlwaysLoadImagesChange.mock.calls.at(-1), [true])
})

test('image settings disable global loading immediately and survive load and removal failures', async () => {
  const onAlwaysLoadImagesChange = vi.fn()
  const enabled = await render(ImagesTab, { alwaysLoadImages: true, onAlwaysLoadImagesChange })
  const switchInput = enabled.target.querySelector('[data-switch]')
  switchInput.checked = false
  switchInput.dispatchEvent(new Event('change', { bubbles: true }))
  await flushAsync()
  assert.deepEqual(onAlwaysLoadImagesChange.mock.calls.at(-1), [false])
  assert.match(enabled.target.textContent, /images\.addresses/)

  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.GetImageAllowlist.mockRejectedValueOnce(new Error('load failed'))
  const failed = await render(ImagesTab, { alwaysLoadImages: false, onAlwaysLoadImagesChange: vi.fn() })
  assert.match(failed.target.textContent, /images\.noAddresses/)

  backend.GetImageAllowlist.mockResolvedValueOnce([
    { id: 'sender-2', type: 'sender', value: 'bob@example.test' },
  ])
  backend.RemoveImageAllowlist.mockRejectedValueOnce(new Error('remove failed'))
  const removal = await render(ImagesTab, { alwaysLoadImages: false, onAlwaysLoadImagesChange: vi.fn() })
  titledButtons(removal.target, 'images.removeButton')[0].click()
  await flushAsync()
  removal.target.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.equal(imageStore.refresh.mock.calls.length, 0)
  assert.ok(error.mock.calls.length >= 2)
})

test('about tab opens the website and renders every information section', async () => {
  const appInfo = {
    name: 'aulycMail',
    version: '0.6.0-test',
    buildNumber: '42',
    displayVersion: '0.6.0-test (build 42)',
    website: 'https://example.test/aulycmail',
  }
  const { target } = await render(AboutTab, { appInfo })
  const document = target.querySelector('[data-about-document]')
  assert.ok(document)
  const fixedHeader = document.querySelector('[data-about-fixed-header]')
  const scrollRegion = document.querySelector('[data-about-scroll-region]')
  const fixedFooter = document.querySelector('[data-about-fixed-footer]')
  assert.ok(fixedHeader)
  assert.ok(scrollRegion)
  assert.ok(fixedFooter)
  assert.equal(scrollRegion.contains(fixedHeader), false)
  assert.equal(scrollRegion.contains(fixedFooter), false)
  assert.match(scrollRegion.className, /overflow-y-auto/)
  assert.match(document.textContent, /settingsAbout\.title/)
  assert.match(document.querySelector('[data-about-section="metadata"]').textContent, /0\.6\.0-test · build 42/)
  assert.match(document.querySelector('[data-about-section="metadata"]').textContent, /settingsAbout\.compatibilityValue/)
  assert.match(document.querySelector('[data-about-section="metadata"]').textContent, /settingsAbout\.systemRequirementValue/)
  assert.match(document.querySelector('[data-about-section="introduction"]').textContent, /settingsAbout\.product\.dataBody/)
  assert.match(document.querySelector('[data-about-section="copyright"]').textContent, /settingsAbout\.copyright/)

  const websiteButton = document.querySelector('[data-about-section="website"] button')
  assert.equal(websiteButton.textContent.trim(), 'https://example.test/aulycmail')
  assert.equal(websiteButton.dataset.settingsFocusStyle, 'link')
  websiteButton.click()
  assert.deepEqual(runtime.openURL.mock.calls, [['https://example.test/aulycmail']])

  const relatedLinks = document.querySelector('[data-about-section="related-links"]')
  assert.equal(relatedLinks.querySelectorAll('[data-settings-focus-style="link"]').length, 5)
  const cases = [
    ['settingsAbout.productDescription', 'settingsAbout.product.featureBackup'],
    ['settingsAbout.privacyPolicy', 'settingsAbout.privacy.localBackups'],
    ['settingsAbout.termsOfUse', 'settingsAbout.terms.responsibilityBackup'],
    ['settingsAbout.acknowledgementsLabel', 'settingsAbout.acknowledgements.aiEra'],
  ]
  for (const [buttonLabel, expected] of cases) {
    buttonWithText(target, buttonLabel).click()
    await flushAsync()
    const dialog = target.querySelector('[role="dialog"]')
    assert.ok(dialog)
    assert.match(dialog.textContent, new RegExp(expected.replaceAll('.', '\\.')))
    if (buttonLabel === 'settingsAbout.acknowledgementsLabel') {
      buttonWithText(dialog, 'https://github.com/hkdb/aerion').click()
      assert.deepEqual(runtime.openURL.mock.calls.at(-1), ['https://github.com/hkdb/aerion'])
    }
    ;[...dialog.querySelectorAll('button')].at(-1).click()
    await flushAsync()
    assert.equal(target.querySelector('[role="dialog"]'), null)
  }
})

test('about tab renders loading and failed states without opening an absent website', async () => {
  const loading = await render(AboutTab, { appInfo: null, loading: true })
  assert.ok(loading.target.querySelector('[data-stub="static-component"]'))
  const failed = await render(AboutTab, { appInfo: null })
  assert.match(failed.target.textContent, /settingsAbout\.failedToLoad/)
  const noWebsite = await render(AboutTab, {
    appInfo: { name: 'aulycMail', version: 'test', buildNumber: '0', displayVersion: 'test', website: '' },
  })
  const websiteButton = noWebsite.target.querySelector('[data-about-section="website"] button')
  assert.equal(websiteButton.disabled, true)
  assert.equal(websiteButton.textContent.trim(), '—')
  websiteButton.click()
  assert.equal(runtime.openURL.mock.calls.length, 0)
})

test('accounts tab filters shared mailboxes, tests connections, edits, deletes, and reorders regular accounts', async () => {
  const onAccountOrderChanged = vi.fn()
  const { target, instance } = await render(AccountsTab, { onAccountOrderChanged })
  assert.match(target.textContent, /a@example\.test/)
  assert.match(target.textContent, /b@example\.test/)
  assert.doesNotMatch(target.textContent, /shared@example\.test/)
  assert.deepEqual(accounts.getAccountConnOK.mock.calls.map(([id]) => id), ['a', 'b'])
  assert.match(target.textContent, /account\.lastConnected/)
  assert.match(target.textContent, /account\.neverConnected/)

  titledButtons(target, 'account.testConnection')[0].click()
  await flushAsync()
  assert.match(target.querySelector('[data-connection-dialog]').textContent, /account\.connectionSuccessful/)

  accounts.testAccountConnection.mockResolvedValueOnce({ success: false, error: 'bad credentials' })
  titledButtons(target, 'account.testConnection')[1].click()
  await flushAsync()
  assert.match(target.querySelector('[data-connection-dialog]').textContent, /bad credentials/)

  titledButtons(target, 'settingsAccounts.moveDown')[0].click()
  await flushAsync()
  assert.deepEqual(accounts.reorderAccounts.mock.calls.at(-1), [['b', 'shared', 'a']])
  assert.deepEqual(onAccountOrderChanged.mock.calls.at(-1), ['a', 'move-down'])

  titledButtons(target, 'settingsAccounts.moveUp')[1].click()
  await flushAsync()
  assert.deepEqual(accounts.reorderAccounts.mock.calls.at(-1), [['b', 'shared', 'a']])
  assert.deepEqual(onAccountOrderChanged.mock.calls.at(-1), ['b', 'move-up'])

  titledButtons(target, 'settingsAccounts.editAccount')[0].click()
  await flushAsync()
  assert.ok(target.querySelector('[data-account-dialog]'))
  target.querySelector('[data-account-dialog-close]').click()
  await flushAsync()
  assert.equal(target.querySelector('[data-account-dialog]'), null)

  titledButtons(target, 'settingsAccounts.deleteAccount')[1].click()
  await flushAsync()
  assert.match(target.querySelector('[data-delete-account-dialog]').textContent, /b@example\.test/)
  target.querySelector('[data-delete-account-close]').click()
  await flushAsync()
  assert.equal(target.querySelector('[data-delete-account-dialog]'), null)

  instance.openAdd()
  await flushAsync()
  assert.ok(target.querySelector('[data-account-dialog]'))
  assert.equal(titledButtons(target, 'settingsAccounts.moveUp')[0].disabled, true)
  assert.equal(titledButtons(target, 'settingsAccounts.moveDown').at(-1).disabled, true)
})

test('accounts tab reports thrown connection errors and renders loading and empty states', async () => {
  accounts.testAccountConnection.mockRejectedValueOnce(new Error('network unavailable'))
  const populated = await render(AccountsTab)
  titledButtons(populated.target, 'account.testConnection')[0].click()
  await flushAsync()
  assert.match(populated.target.querySelector('[data-connection-dialog]').textContent, /network unavailable/)

  accounts.loading = true
  const loading = await render(AccountsTab)
  assert.ok(loading.target.querySelector('[data-stub="static-component"]'))

  accounts.loading = false
  accounts.accounts = []
  const empty = await render(AccountsTab)
  assert.match(empty.target.textContent, /settingsAccounts\.noAccountsConfigured/)
})

test('accounts tab tolerates connection-history failures and supplies fallback test messages', async () => {
  accounts.getAccountConnOK
    .mockReset()
    .mockRejectedValueOnce(new Error('history unavailable'))
    .mockResolvedValueOnce('not-a-date')
  accounts.testAccountConnection.mockResolvedValueOnce({ success: false })
  const fallback = await render(AccountsTab)
  assert.match(fallback.target.textContent, /account\.neverConnected/)
  assert.match(fallback.target.textContent, /account\.lastConnected/)
  titledButtons(fallback.target, 'account.testConnection')[0].click()
  await flushAsync()
  assert.match(fallback.target.querySelector('[data-connection-dialog]').textContent, /account\.connectionFailed/)

  accounts.testAccountConnection.mockRejectedValueOnce('plain failure')
  titledButtons(fallback.target, 'account.testConnection')[1].click()
  await flushAsync()
  assert.match(fallback.target.querySelector('[data-connection-dialog]').textContent, /plain failure/)
})
