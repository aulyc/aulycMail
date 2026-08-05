// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

vi.mock('bits-ui', async () => {
  const primitive = (await import('./fixtures/PrimitiveTestStub.svelte')).default
  return {
    AlertDialog: {
      Action: primitive,
      Cancel: primitive,
      Content: primitive,
      Description: primitive,
      Overlay: primitive,
      Portal: primitive,
      Title: primitive,
      Trigger: primitive,
    },
    Dialog: {
      Description: primitive,
      Portal: primitive,
      Overlay: primitive,
      Content: primitive,
      Close: primitive,
      Trigger: primitive,
    },
    DropdownMenu: {
      Content: primitive,
      Group: primitive,
      Item: primitive,
      Portal: primitive,
      Root: primitive,
      Separator: primitive,
      Trigger: primitive,
    },
  }
})
vi.mock('$lib/components/ui/select', async () => ({
  Root: (await import('./fixtures/BoolSelectRootTestStub.svelte')).default,
  Trigger: (await import('./fixtures/SnippetTestStub.svelte')).default,
  Value: (await import('./fixtures/SnippetTestStub.svelte')).default,
  Content: (await import('./fixtures/SnippetTestStub.svelte')).default,
  Item: (await import('./fixtures/SelectItemTestStub.svelte')).default,
}))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key) => key)
      return () => {}
    },
  },
}))

import UiPrimitivesHarness from './fixtures/UiPrimitivesHarness.svelte'

const mounted = []

async function render(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(UiPrimitivesHarness, { target, props })
  mounted.push(instance)
  await tick()
  return target
}

beforeEach(() => {
  document.body.innerHTML = ''
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
})

test('wrapper primitives forward classes, geometry, content, and enabled callbacks', async () => {
  const onAction = vi.fn()
  const onCancel = vi.fn()
  const onSelect = vi.fn()
  const onCheckedChange = vi.fn()
  const onOpenAutoFocus = vi.fn((event) => event.preventDefault())
  const target = await render({ onAction, onCancel, onSelect, onCheckedChange, onOpenAutoFocus })

  assert.match(target.textContent, /Alert title/)
  assert.match(target.textContent, /Alert description/)
  assert.match(target.textContent, /Dialog description/)
  assert.match(target.textContent, /Dialog footer/)
  for (const className of [
    'alert-content-custom', 'alert-header-custom', 'alert-title-custom', 'alert-description-custom',
    'alert-footer-custom', 'alert-cancel-custom', 'alert-action-custom', 'dropdown-content-custom',
    'dropdown-item-custom', 'dropdown-separator-custom', 'dialog-description-custom',
    'dialog-footer-custom',
  ]) {
    assert.ok(target.querySelector(`.${className}`), `missing forwarded class ${className}`)
  }

  const dropdownContent = target.querySelector('.dropdown-content-custom')
  assert.equal(dropdownContent.dataset.side, 'top')
  assert.equal(dropdownContent.dataset.align, 'end')
  assert.equal(dropdownContent.dataset.offset, '9')

  target.querySelector('.alert-action-custom [data-onclick]').click()
  target.querySelector('.alert-cancel-custom [data-onclick]').click()
  target.querySelector('.dropdown-item-custom [data-on-select]').click()
  target.querySelector('.alert-content-custom [data-open-focus]').click()
  target.querySelector('[data-bool-value="yes"]').click()
  await tick()
  assert.equal(onAction.mock.calls.length, 1)
  assert.equal(onCancel.mock.calls.length, 1)
  assert.equal(onSelect.mock.calls.length, 1)
  assert.equal(onOpenAutoFocus.mock.calls.length, 1)
  assert.equal(target.querySelector('.alert-content-custom').dataset.openPrevented, 'true')
  assert.deepEqual(onCheckedChange.mock.calls, [[true]])
  assert.equal(target.querySelector('[data-bool-select-root]').dataset.value, 'yes')
})

test('content focus restoration is optional and boolean selection handles no and undefined', async () => {
  let target = await render({ preventCloseAutoFocus: false, checked: true })
  let content = target.querySelector('.alert-content-custom')
  content.querySelector('[data-close-focus]').click()
  await tick()
  assert.equal(content.dataset.closePrevented, 'false')
  assert.equal(target.querySelector('[data-bool-select-root]').dataset.value, 'yes')

  const changed = vi.fn()
  target = await render({
    preventCloseAutoFocus: true,
    disabled: true,
    onCheckedChange: changed,
    onAction: vi.fn(),
    onSelect: vi.fn(),
  })
  content = target.querySelector('.alert-content-custom')
  content.querySelector('[data-close-focus]').click()
  await tick()
  assert.equal(content.dataset.closePrevented, 'true')
  assert.equal(target.querySelector('.alert-action-custom [data-onclick]').disabled, true)
  assert.equal(target.querySelector('.dropdown-item-custom [data-on-select]').disabled, true)

  target.querySelector('[data-bool-value="no"]').click()
  target.querySelector('[data-bool-value="undefined"]').click()
  await tick()
  assert.deepEqual(changed.mock.calls, [[false], [false]])
  assert.equal(target.querySelector('[data-bool-select-root]').dataset.value, '')
})
