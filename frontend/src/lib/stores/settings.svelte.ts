// Runes-based settings store
// Provides reactive state for application settings

// @ts-ignore - wailsjs path
import { GetMessageListDensity, GetMessageListSortOrder, GetThemeMode, GetLanguage, GetComposerFormat, GetAlwaysLoadImages, GetDarkMailContent, GetAccentBarUnread, GetDeveloperMode, GetEnhancedKeyboardNavigation } from '../../../wailsjs/go/app/App'
import { setLocale as setI18nLocale, detectSystemLocale } from '$lib/i18n'
import { loadDateFnsLocale, getDateFnsLocale } from '$lib/i18n/dateFnsLocale'
import type { Locale } from 'date-fns'

export type ComposerFormat = 'rich' | 'plain'
export type MessageListDensity = 'micro' | 'compact' | 'standard' | 'large'
export type MessageListSortOrder = 'newest' | 'oldest'
export type ThemeMode =
  | 'system'
  | 'light' | 'light-blue' | 'light-orange' | 'light-balanced' | 'adwaita-light' | 'breeze-light'
  | 'dark' | 'dark-gray' | 'dark-balanced' | 'adwaita-dark' | 'breeze-dark'
  | 'catppuccin-latte' | 'catppuccin-frappe' | 'catppuccin-macchiato' | 'catppuccin-mocha'
  | 'dracula' | 'source-light' | 'source-dark' | 'source-soft-dark' | 'tokyo-night'
  | 'nord-light' | 'nord-dark'
  | 'pop-light' | 'pop-dark'
  | 'yaru-light' | 'yaru-dark'
  | 'vs-code-light' | 'vs-code-dark'

// Module-level reactive state
let messageListDensity = $state<MessageListDensity>('standard')
let messageListSortOrder = $state<MessageListSortOrder>('newest')
let themeMode = $state<ThemeMode>('system')
let language = $state<string>('')
let composerFormat = $state<ComposerFormat>('plain')
let alwaysLoadImages = $state<boolean>(false)
let darkMailContent = $state<boolean>(false)
let accentBarUnread = $state<boolean>(false)
let developerMode = $state<boolean>(false)
let enhancedKeyboardNavigation = $state<boolean>(true)

// Getter functions to access the state
export function getMessageListDensity(): MessageListDensity {
  return messageListDensity
}

export function getMessageListSortOrder(): MessageListSortOrder {
  return messageListSortOrder
}

export function getThemeMode(): ThemeMode {
  return themeMode
}

export function getComposerFormat(): ComposerFormat {
  return composerFormat
}

export function getAlwaysLoadImages(): boolean {
  return alwaysLoadImages
}

export function getDarkMailContent(): boolean {
  return darkMailContent
}

export function getAccentBarUnread(): boolean {
  return accentBarUnread
}

export function getDeveloperMode(): boolean {
  return developerMode
}

export function getEnhancedKeyboardNavigation(): boolean {
  return enhancedKeyboardNavigation
}

export function getCurrentDateFnsLocale(): Locale | undefined {
  // Fall back to the auto-detected system locale when no language was explicitly
  // saved, so relative times match the (auto-detected) UI language.
  return getDateFnsLocale(language || detectSystemLocale())
}

// Setter functions to update the state
export function setMessageListDensity(density: MessageListDensity) {
  messageListDensity = density === 'micro' ? 'compact' : density
}

export function setMessageListSortOrder(sortOrder: MessageListSortOrder) {
  messageListSortOrder = sortOrder
}

export function setThemeMode(mode: ThemeMode) {
  themeMode = mode
}

export function setLanguage(lang: string) {
  language = lang
  if (lang) {
    setI18nLocale(lang)
    loadDateFnsLocale(lang)
  }
}

export function setComposerFormat(format: ComposerFormat) {
  composerFormat = format
}

export function setAlwaysLoadImages(v: boolean) {
  alwaysLoadImages = v
}

export function setDarkMailContent(v: boolean) {
  darkMailContent = v
}

export function setAccentBarUnread(v: boolean) {
  accentBarUnread = v
}

export function setDeveloperMode(v: boolean) {
  developerMode = v
}

export function setEnhancedKeyboardNavigation(v: boolean) {
  enhancedKeyboardNavigation = v
}

// Load settings from backend (call on app startup)
export async function loadSettings(): Promise<ThemeMode> {
  try {
    const [density, sortOrder, theme, lang, compFormat, alwaysImages, darkMail, accentBar, devMode, keyboardNavigation] = await Promise.all([
      GetMessageListDensity(),
      GetMessageListSortOrder(),
      GetThemeMode(),
      GetLanguage(),
      GetComposerFormat(),
      GetAlwaysLoadImages(),
      GetDarkMailContent(),
      GetAccentBarUnread(),
      GetDeveloperMode(),
      GetEnhancedKeyboardNavigation(),
    ])
    // 'micro' was removed from the UI; fold any stored value into 'compact' (小).
    messageListDensity = (density === 'micro' ? 'compact' : (density as MessageListDensity)) || 'standard'
    messageListSortOrder = (sortOrder as MessageListSortOrder) || 'newest'
    themeMode = (theme as ThemeMode) || 'system'
    composerFormat = (compFormat as ComposerFormat) || 'plain'
    alwaysLoadImages = alwaysImages ?? false
    darkMailContent = darkMail ?? false
    accentBarUnread = accentBar ?? false
    developerMode = devMode ?? false
    enhancedKeyboardNavigation = keyboardNavigation ?? true
    // Apply saved language (if set, overrides system detection from initI18n)
    if (lang) {
      language = lang
      setI18nLocale(lang)
      await loadDateFnsLocale(lang)
    }
    return themeMode
  } catch (err) {
    console.error('Failed to load settings:', err)
    return 'system'
  }
}
