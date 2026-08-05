// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

import Switch from '../src/lib/components/ui/switch/Switch.svelte'

const mounted = []

async function renderSwitch(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(Switch, { target, props })
  mounted.push(instance)
  await tick()
  return target.querySelector('[role="switch"]')
}

beforeEach(() => {
  document.body.innerHTML = ''
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
})

test('toggles from pointer input and reports the new accessible state', async () => {
  const onCheckedChange = vi.fn()
  const control = await renderSwitch({ id: 'sync-switch', class: 'custom-switch', onCheckedChange })
  assert.equal(control.id, 'sync-switch')
  assert.match(control.className, /custom-switch/)
  assert.equal(control.getAttribute('aria-checked'), 'false')
  assert.equal(control.getAttribute('aria-label'), 'Toggle off')

  control.click()
  await tick()
  assert.equal(control.getAttribute('aria-checked'), 'true')
  assert.equal(control.getAttribute('aria-label'), 'Toggle on')
  assert.deepEqual(onCheckedChange.mock.calls, [[true]])
})

test('supports Enter and Space without allowing their browser defaults', async () => {
  const onCheckedChange = vi.fn()
  const control = await renderSwitch({ checked: true, onCheckedChange })

  const enter = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })
  control.dispatchEvent(enter)
  await tick()
  assert.equal(enter.defaultPrevented, true)
  assert.equal(control.getAttribute('aria-checked'), 'false')

  const space = new KeyboardEvent('keydown', { key: ' ', bubbles: true, cancelable: true })
  control.dispatchEvent(space)
  await tick()
  assert.equal(space.defaultPrevented, true)
  assert.equal(control.getAttribute('aria-checked'), 'true')

  const arrow = new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true })
  control.dispatchEvent(arrow)
  assert.equal(arrow.defaultPrevented, false)
  assert.deepEqual(onCheckedChange.mock.calls, [[false], [true]])
})

test('disabled switches ignore pointer and keyboard input', async () => {
  const onCheckedChange = vi.fn()
  const control = await renderSwitch({ checked: true, disabled: true, onCheckedChange })
  assert.equal(control.disabled, true)
  assert.equal(control.getAttribute('aria-disabled'), 'true')

  control.click()
  const enter = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })
  control.dispatchEvent(enter)
  await tick()
  assert.equal(enter.defaultPrevented, false)
  assert.equal(control.getAttribute('aria-checked'), 'true')
  assert.equal(onCheckedChange.mock.calls.length, 0)
})
