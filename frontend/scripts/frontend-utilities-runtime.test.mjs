import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'

const locale = vi.hoisted(() => ({ current: undefined }))
const ui = vi.hoisted(() => ({ active: 'mail' }))

vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('$lib/stores/settings.svelte', () => ({ getCurrentDateFnsLocale: () => locale.current }))
vi.mock('$lib/stores/uiState.svelte', () => ({ getActivePane: () => ui.active }))

import {
  formatLocalDate,
  formatLocalDateTime,
  formatLocalDateTimeShort,
  formatMessageDate,
  formatRelativeDate,
  formatRelativeDateTime,
  parseFlexibleDate,
} from '../src/lib/utils/date.ts'
import { buildDarkMailFilterStyles, getDarkMailSurfaceBackground } from '../src/lib/utils/dark-mail.ts'
import {
  activityStatusLabel,
  activitySummary,
  activityTime,
  activityTypeLabel,
  backupActivityDetails,
} from '../src/lib/components/settings/activity/activityLogFormat.ts'
import { dispatchPaneShortcut, registerPaneShortcut } from '../src/lib/stores/paneShortcuts.svelte.ts'
import { addToast, toasts } from '../src/lib/stores/toast.ts'

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  locale.current = undefined
  ui.active = 'mail'
})

test('date utilities cover relative boundaries, message labels, local formatting, and Go timestamps', () => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-08-05T12:00:00'))
  assert.equal(formatRelativeDate(new Date('2026-08-05T11:59:30')), 'date.justNow')
  assert.equal(formatRelativeDate(new Date('2026-08-05T11:45:00')), '15m')
  assert.equal(formatRelativeDate(new Date('2026-08-05T09:00:00')), '3h')
  assert.equal(formatRelativeDate(new Date('2026-08-04T12:00:00')), 'date.yesterday')
  assert.equal(formatRelativeDate(new Date('2026-08-03T12:00:00')), 'Monday')
  assert.match(formatRelativeDate(new Date('2026-07-01T12:00:00')), /Jul 1/)
  assert.match(formatRelativeDate(new Date('2025-07-01T12:00:00')), /Jul 1, 2025/)
  assert.equal(formatRelativeDateTime(new Date('2026-08-05T09:05:00')), '09:05')
  assert.match(formatRelativeDateTime(new Date('2026-08-04T09:05:00')), /date.yesterday 09:05/)

  assert.equal(formatMessageDate(new Date('2026-08-05T09:05:00')), 'date.todayAt:{"time":"09:05"}')
  assert.equal(formatMessageDate(new Date('2026-08-04T09:05:00')), 'date.yesterdayAt:{"time":"09:05"}')
  assert.match(formatMessageDate(new Date('2026-07-01T09:05:00')), /Jul 1 at 09:05/)
  assert.match(formatMessageDate(new Date('2025-07-01T09:05:00')), /Jul 1, 2025 at 09:05/)

  const date = new Date('2026-08-01T03:04:00')
  assert.equal(typeof formatLocalDate(date), 'string')
  assert.equal(typeof formatLocalDateTime(date.toISOString()), 'string')
  assert.match(formatLocalDateTimeShort(date), /\d/)
  assert.equal(parseFlexibleDate(''), null)
  assert.equal(parseFlexibleDate('not a date'), null)
  assert.equal(parseFlexibleDate('2026-08-01 03:04:05.123 +0800 CST')?.toISOString(), '2026-07-31T19:04:05.000Z')
  assert.equal(parseFlexibleDate('2026-08-01T03:04:05Z')?.toISOString(), '2026-08-01T03:04:05.000Z')
})

test('dark mail styles use theme values, clamp lightness, and fall back safely', () => {
  const values = new Map()
  vi.stubGlobal('document', { documentElement: {} })
  vi.stubGlobal('getComputedStyle', () => ({ getPropertyValue: (name) => values.get(name) ?? '' }))

  assert.equal(getDarkMailSurfaceBackground(), '#000')
  assert.deepEqual(buildDarkMailFilterStyles(), {
    surfaceBackground: '#000',
    contentFilter: 'invert(1) hue-rotate(180deg) saturate(1) hue-rotate(0deg)',
    mediaFilter: 'invert(1) hue-rotate(180deg) saturate(1) hue-rotate(0deg)',
  })

  values.set('--background', '220 20% 15%')
  values.set('--dark-mail-bg-l', '120')
  values.set('--dark-mail-saturate', '2')
  values.set('--dark-mail-hue', '12.5')
  assert.deepEqual(buildDarkMailFilterStyles(), {
    surfaceBackground: 'hsl(220 20% 15%)',
    contentFilter: 'invert(0) hue-rotate(180deg) saturate(2) hue-rotate(12.5deg)',
    mediaFilter: 'invert(0) hue-rotate(180deg) saturate(0.5) hue-rotate(-12.5deg)',
  })

  values.set('--dark-mail-bg-l', 'invalid')
  values.set('--dark-mail-saturate', '-1')
  values.set('--dark-mail-hue', 'invalid')
  const fallback = buildDarkMailFilterStyles()
  assert.match(fallback.contentFilter, /invert\(0\.85\)/)
  assert.match(fallback.contentFilter, /saturate\(1\)/)
  assert.match(fallback.contentFilter, /hue-rotate\(0deg\)/)
})

test('activity formatting handles payload objects, JSON, invalid data, every status, and summaries', () => {
  assert.equal(backupActivityDetails({ type: 'sync' }), null)
  assert.equal(backupActivityDetails({ type: 'backup', payloadJson: '{bad' }), null)
  const details = backupActivityDetails({
    type: 'backup',
    payload: { mode: 'incremental', total: 10, backedUp: 7, added: 4, skipped: 1, missing: 1, unavailable: 0, failed: 1, directory: ' /backup ' },
  })
  assert.equal(details.mode, 'settingsBackup.incrementalExport')
  assert.equal(details.directory, '/backup')
  assert.equal(details.statistics.backedUp, 7)
  assert.equal(details.statistics.notBackedUp, 2)
  assert.equal(backupActivityDetails({ type: 'backup', payloadJson: JSON.stringify({ mode: 'full', success: 2 }) }).mode, 'settingsBackup.fullExport')

  assert.equal(activityTypeLabel('sync'), 'activityLog.sync')
  assert.equal(activityTypeLabel('backup'), 'activityLog.backup')
  assert.equal(activityTypeLabel('custom'), 'custom')
  assert.deepEqual(
    ['success', 'partial', 'failed', 'cancelled', 'mystery'].map(activityStatusLabel),
    ['activityLog.status.success', 'activityLog.status.partial', 'activityLog.status.failed', 'activityLog.status.cancelled', 'activityLog.status.unknown'],
  )
  assert.equal(typeof activityTime('2026-08-01T03:04:05Z'), 'string')
  assert.match(activitySummary({ type: 'backup', status: 'success', payload: { mode: 'full', completed: 2, failed: 1 } }), /activityLog.backupCompactSummary/)
  assert.equal(activitySummary({ type: 'backup', status: 'cancelled' }), 'activityLog.backupCancelled')
  assert.equal(activitySummary({ type: 'backup', status: 'failed' }), 'activityLog.backupFailed')
  assert.match(activitySummary({ type: 'sync', title: 'Inbox', payload: { accountEmail: 'a@example.com', folderName: 'Sent', added: 3 } }), /a@example.com · Sent/)
  assert.match(activitySummary({ type: 'sync', title: 'Inbox', payload: {} }), /Inbox/)
  assert.equal(activitySummary({ type: 'custom', summary: 'Custom summary' }), 'Custom summary')
})

test('pane shortcut registry dispatches in order and unregisters without affecting other handlers', () => {
  const first = vi.fn()
  const second = vi.fn()
  const unregisterFirst = registerPaneShortcut('contacts', (event) => event.key === 'x', first)
  const unregisterSecond = registerPaneShortcut('contacts', (event) => event.key === 'y', second)
  ui.active = 'contacts'
  assert.equal(dispatchPaneShortcut({ key: 'none' }), false)
  assert.equal(dispatchPaneShortcut({ key: 'x' }), true)
  assert.equal(first.mock.calls.length, 1)
  unregisterFirst()
  assert.equal(dispatchPaneShortcut({ key: 'x' }), false)
  assert.equal(dispatchPaneShortcut({ key: 'y' }), true)
  unregisterSecond()
  assert.equal(dispatchPaneShortcut({ key: 'y' }), false)
  ui.active = 'mail'
  assert.equal(dispatchPaneShortcut({ key: 'y' }), false)
})

test('toast store replaces the current toast, supports each type, manual removal, and expiry', () => {
  vi.useFakeTimers()
  let index = 0
  vi.stubGlobal('crypto', { randomUUID: () => `toast-${++index}` })
  let current = []
  const unsubscribe = toasts.subscribe((value) => { current = value })

  assert.equal(toasts.success('Saved'), 'toast-1')
  assert.deepEqual(current.map((toast) => [toast.type, toast.message]), [['success', 'Saved']])
  assert.equal(toasts.error('Failed'), 'toast-2')
  assert.equal(current[0].type, 'error')
  assert.equal(toasts.info('Info'), 'toast-3')
  assert.equal(toasts.warning('Warning'), 'toast-4')
  const id = addToast({ type: 'success', message: 'Short', duration: 20 })
  assert.equal(id, 'toast-5')
  toasts.remove(id)
  assert.deepEqual(current, [])
  addToast({ type: 'info', message: 'Expires', duration: 20 })
  vi.advanceTimersByTime(20)
  assert.deepEqual(current, [])
  unsubscribe()
})
