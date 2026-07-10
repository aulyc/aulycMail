// Contacts pane i18n registration.
//
// svelte-i18n merges loaders per-locale: core ships its keys, this file ships
// the contacts.* namespace. No key collisions as long as namespaces stay
// distinct.

import { register } from 'svelte-i18n'

export function registerContactsI18n(): void {
  register('en', () => import('./locales/en.json'))
  register('zh-CN', () => import('./locales/zh-CN.json'))
}
