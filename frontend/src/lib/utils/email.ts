export const EMAIL_ADDRESS_PATTERN = /[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}/g
const STRICT_EMAIL_ADDRESS_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const DISPLAY_EMAIL_PATTERN = /^(?:(.+?)\s*<)?([^\s<>]+@[^\s<>]+)>?$/

export interface ParsedEmailAddress {
  name: string
  email: string
}

export function isEmailAddress(value: string): boolean {
  return STRICT_EMAIL_ADDRESS_PATTERN.test(value.trim())
}

export function parseEmailAddress(value: string): ParsedEmailAddress | null {
  const match = value.trim().match(DISPLAY_EMAIL_PATTERN)
  if (!match) return null
  return {
    name: match[1]?.trim() || '',
    email: match[2].toLowerCase(),
  }
}
