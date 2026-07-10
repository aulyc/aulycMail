// @ts-ignore - Wails generated imports
import type { contact, smtp } from '../../../../wailsjs/go/models'

export type MentionSurface = 'plain' | 'rich'
export type PlainTextSegment = { type: 'text' | 'mention'; text: string }

export function getMentionLabel(c: contact.Contact): string {
  return (c.display_name || c.email || '').trim()
}

export function getContactEmail(c: contact.Contact): string {
  return (c.email || '').trim().toLowerCase()
}

function getRecipientEmail(r: smtp.Address): string {
  return ((r as any)?.address || (r as any)?.email || '').trim().toLowerCase()
}

export function hasRecipient(email: string, recipients: smtp.Address[]): boolean {
  const normalized = email.trim().toLowerCase()
  return !!normalized && recipients.some((recipient) => getRecipientEmail(recipient) === normalized)
}

export function mentionKey(surface: MentionSurface, query: string, start: number, end: number): string {
  return `${surface}:${start}:${end}:${query}`
}

export function findMentionToken(textBeforeCursor: string): { query: string; startOffset: number } | null {
  const match = textBeforeCursor.match(/(^|[\s([（【{,;，。！？!?])@([^\s@]{0,40})$/u)
  if (!match) return null
  const query = match[2] || ''
  return {
    query,
    startOffset: textBeforeCursor.length - query.length - 1,
  }
}

export function filteredMentionResults(results: contact.Contact[], query: string): contact.Contact[] {
  const needle = query.trim().toLowerCase()
  if (!needle) return []
  const seen = new Set<string>()
  return (results || []).filter((item) => {
    const email = (item.email || '').trim()
    const label = getMentionLabel(item)
    if (!email && !label) return false
    const key = (email || label).toLowerCase()
    if (seen.has(key)) return false
    seen.add(key)
    return label.toLowerCase().includes(needle) || email.toLowerCase().includes(needle)
  })
}

function isMentionBoundary(text: string, index: number): boolean {
  if (index === 0) return true
  return /[\s([（【{,;，。！？!?]/u.test(text[index - 1] || '')
}

export function getPlainTextMentionSegments(text: string, labels: string[]): PlainTextSegment[] {
  if (!text) return []

  const sortedLabels = [...labels].sort((a, b) => b.length - a.length)
  const segments: PlainTextSegment[] = []
  let buffer = ''
  let i = 0

  const flush = () => {
    if (!buffer) return
    segments.push({ type: 'text', text: buffer })
    buffer = ''
  }

  while (i < text.length) {
    if (text[i] !== '@' || !isMentionBoundary(text, i)) {
      buffer += text[i]
      i += 1
      continue
    }

    const selectedLabel = sortedLabels.find((label) => text.startsWith(`@${label}`, i))
    if (selectedLabel) {
      flush()
      segments.push({ type: 'mention', text: `@${selectedLabel}` })
      i += selectedLabel.length + 1
      continue
    }

    const fallback = text.slice(i).match(/^@([^\s@]{1,40})/u)
    if (fallback) {
      flush()
      segments.push({ type: 'mention', text: fallback[0] })
      i += fallback[0].length
      continue
    }

    buffer += text[i]
    i += 1
  }

  flush()
  return segments
}
