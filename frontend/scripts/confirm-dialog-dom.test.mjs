// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, test, vi } from 'vitest'

const guard = vi.hoisted(() => ({ open: vi.fn(), close: vi.fn() }))

vi.mock('$lib/stores/dialogGuard', () => ({
  dialogGuardOpen: guard.open,
  dialogGuardClose: guard.close,
}))
vi.mock('@iconify/svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))
vi.mock('$lib/components/ui/alert-dialog', async () => ({
  Root: (await import('./fixtures/DialogRootTestStub.svelte')).default,
  Content: (await import('./fixtures/DialogContentTestStub.svelte')).default,
  Header: (await import('./fixtures/SnippetTestStub.svelte')).default,
  Footer: (await import('./fixtures/SnippetTestStub.svelte')).default,
  Title: (await import('./fixtures/SnippetTestStub.svelte')).default,
  Description: (await import('./fixtures/SnippetTestStub.svelte')).default,
  Action: (await import('./fixtures/AlertDialogButtonTestStub.svelte')).default,
  Cancel: (await import('./fixtures/AlertDialogButtonTestStub.svelte')).default,
}))

import ConfirmDialog from '../src/lib/components/ui/confirm-dialog/ConfirmDialog.svelte'

const mounted = []

async function render(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(ConfirmDialog, {
    target,
    props: {
      open: true,
      title: '安装软件更新？',
      description: '将下载并安装 aulycMail 0.7.4。',
      confirmLabel: '更新并重启',
      cancelLabel: '取消',
      onConfirm: vi.fn(),
      ...props,
    },
  })
  mounted.push(instance)
  await tick()
  return target
}

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  document.body.innerHTML = ''
  vi.clearAllMocks()
})

test('renders footer progress before Cancel without widening the confirm button', async () => {
  const target = await render({
    loading: true,
    loadingPresentation: 'footer-progress',
    loadingLabel: '正在下载更新…',
    loadingProgress: 37,
  })

  const progress = target.querySelector('[data-confirm-progress]')
  const [cancel, confirm] = target.querySelectorAll('[data-alert-dialog-button]')
  assert.ok(progress)
  const progressBar = progress.querySelector('[role="progressbar"]')
  assert.equal(progressBar.getAttribute('aria-valuenow'), '37')
  assert.match(progress.textContent, /正在下载更新…/)
  assert.match(progress.textContent, /37%/)
  assert.match(progress.querySelector('[data-confirm-progress-fill]').getAttribute('style'), /37%/)
  assert.ok(progress.compareDocumentPosition(cancel) & Node.DOCUMENT_POSITION_FOLLOWING)
  assert.equal(cancel.disabled, true)
  assert.equal(confirm.disabled, true)
  assert.match(confirm.className, /w-40/)
  assert.equal(confirm.querySelector('[data-stub="static-component"]'), null)
})

test('reserves the same confirm button width before footer progress starts', async () => {
  const target = await render({
    loading: false,
    loadingPresentation: 'footer-progress',
  })
  const confirm = target.querySelectorAll('[data-alert-dialog-button]')[1]
  assert.equal(target.querySelector('[data-confirm-progress]'), null)
  assert.match(confirm.className, /w-40/)
})

test('keeps the existing button spinner presentation for other loading dialogs', async () => {
  const target = await render({ loading: true })
  const buttons = target.querySelectorAll('[data-alert-dialog-button]')
  const confirm = buttons[1]
  assert.equal(target.querySelector('[data-confirm-progress]'), null)
  assert.ok(confirm.querySelector('[data-stub="static-component"]'))
  assert.doesNotMatch(confirm.className, /w-40/)
})
