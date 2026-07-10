import { register, init, waitLocale, locale, _ } from 'svelte-i18n'
import { loadDateFnsLocale } from './dateFnsLocale'
import { registerContactsI18n } from '$contacts/i18n'

// Register core locale files with lazy loading. The built-in Contacts pane
// registers its own locales via Vite glob auto-discovery in initI18n().
register('en', () => import('./locales/en.json'))
register('zh-CN', () => import('./locales/zh-CN.json'))

// Supported locales for the language picker
export const supportedLocales = [
  { code: 'en', name: 'English' },
  { code: 'zh-CN', name: '简体中文' },
] as const

/**
 * Detect system locale from navigator.language and map to supported locales.
 * Any zh-* (or bare zh) → zh-CN, en-US → en, anything else → en (fallback).
 */
export function detectSystemLocale(): string {
  const nav = navigator.language || 'en'
  const lower = nav.toLowerCase()

  // Exact match first
  const exact = supportedLocales.find(l => l.code.toLowerCase() === lower)
  if (exact) return exact.code

  // Language-only match (e.g., "zh" → "zh-CN", "en-US" → "en")
  const lang = lower.split('-')[0]
  if (lang === 'zh') return 'zh-CN' // any Chinese variant maps to Simplified Chinese

  const langMatch = supportedLocales.find(l => l.code.toLowerCase().split('-')[0] === lang)
  if (langMatch) return langMatch.code

  return 'en'
}

/**
 * Initialize i18n and wait for the initial locale to load.
 * Must be awaited before mounting the Svelte app, otherwise $_ throws.
 * @param savedLocale - Previously saved locale code from backend settings, or undefined for auto-detect
 */
export async function initI18n(savedLocale?: string): Promise<void> {
  const initialLocale = savedLocale || detectSystemLocale()

  // Register Contacts locale loaders before init() so its messages are merged
  // into the active locale on first wait.
  registerContactsI18n()

  init({
    fallbackLocale: 'en',
    initialLocale,
  })

  // Preload the matching date-fns locale so relative times (e.g. the sidebar
  // "X 前同步" status) render in the active language even when the user never
  // explicitly saved a language setting (auto-detected case).
  await Promise.all([waitLocale(), loadDateFnsLocale(initialLocale)])
}

/**
 * Change the active locale at runtime.
 */
export function setLocale(code: string) {
  locale.set(code)
}

export { _ }
