import { format, isToday, isYesterday, isThisWeek, isThisYear } from 'date-fns'
import { get } from 'svelte/store'
import { _ } from '$lib/i18n'
import { getCurrentDateFnsLocale } from '$lib/stores/settings.svelte'

/**
 * Format a date relative to now for message list display
 * - < 1 minute: "just now"
 * - < 1 hour: "Xm"
 * - < 24 hours: "Xh"
 * - Yesterday: "Yesterday"
 * - This week: "Monday", "Tuesday", etc.
 * - This year: "Dec 15"
 * - Older: "Dec 15, 2023"
 */
export function formatRelativeDate(date: Date): string {
  const t = get(_)
  const locale = getCurrentDateFnsLocale()
  // Chinese uses 年/月/日 patterns ("6月25日", "2024年6月25日"); date-fns treats
  // the CJK characters as literals since only a–z are format tokens.
  const isZh = locale?.code === 'zh-CN'
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMinutes = Math.floor(diffMs / (1000 * 60))
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60))

  if (diffMinutes < 1) {
    return t('date.justNow')
  }

  if (diffMinutes < 60) {
    return `${diffMinutes}m`
  }

  if (diffHours < 24 && isToday(date)) {
    return `${diffHours}h`
  }

  if (isYesterday(date)) {
    return t('date.yesterday')
  }

  if (isThisWeek(date)) {
    return format(date, 'EEEE', { locale })
  }

  if (isThisYear(date)) {
    return format(date, isZh ? 'M月d日' : 'MMM d', { locale })
  }

  return format(date, isZh ? 'yyyy年M月d日' : 'MMM d, yyyy', { locale })
}

/**
 * Relative date plus 24-hour wall-clock time — used by the search overlay
 * results, where the exact time helps distinguish same-day messages.
 * - Today: just the time ("13:18") — the relative label would itself be a
 *   duration ("5m", "3h"), so the wall-clock time alone reads cleaner.
 * - Otherwise: relative date + time ("4月7日 13:18", "2025年3月9日 13:18").
 */
export function formatRelativeDateTime(date: Date): string {
  const locale = getCurrentDateFnsLocale()
  const time = format(date, 'HH:mm', { locale })
  if (isToday(date)) return time
  return `${formatRelativeDate(date)} ${time}`
}

/**
 * Format a date for message header display
 * Shows full date and time
 */
export function formatMessageDate(date: Date): string {
  const t = get(_)
  const locale = getCurrentDateFnsLocale()
  const time = format(date, 'HH:mm', { locale })
  const isZh = locale?.code === 'zh-CN'

  if (isToday(date)) {
    return t('date.todayAt', { values: { time } })
  }

  if (isYesterday(date)) {
    return t('date.yesterdayAt', { values: { time } })
  }

  if (isThisYear(date)) {
    return format(date, isZh ? 'M月d日 HH:mm' : 'MMM d \'at\' HH:mm', { locale })
  }

  return format(date, isZh ? 'yyyy年M月d日 HH:mm' : 'MMM d, yyyy \'at\' HH:mm', { locale })
}

export function formatLocalDate(value: string | Date): string {
  const date = value instanceof Date ? value : new Date(value)
  return date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

export function formatLocalDateTime(value: string | Date): string {
  const date = value instanceof Date ? value : new Date(value)
  return date.toLocaleString()
}

export function formatLocalDateTimeShort(value: string | Date): string {
  const date = value instanceof Date ? value : new Date(value)
  return `${date.toLocaleDateString()} ${date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`
}
