#!/usr/bin/env node
/* eslint-disable @typescript-eslint/no-require-imports */

const fs = require('fs')
const path = require('path')

const root = path.resolve(__dirname, '..')
const srcDir = path.join(root, 'src')
const localeFiles = [
  path.join(srcDir, 'lib/i18n/locales/en.json'),
  path.join(srcDir, 'lib/i18n/locales/zh-CN.json'),
  path.join(srcDir, 'lib/contacts/i18n/locales/en.json'),
  path.join(srcDir, 'lib/contacts/i18n/locales/zh-CN.json'),
]

function readJSON(file) {
  return JSON.parse(fs.readFileSync(file, 'utf8'))
}

function flattenKeys(value, prefix = '', out = new Set()) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    if (prefix) out.add(prefix)
    return out
  }

  const entries = Object.entries(value)
  if (entries.length === 0) {
    if (prefix) out.add(prefix)
    return out
  }

  for (const [key, child] of entries) {
    flattenKeys(child, prefix ? `${prefix}.${key}` : key, out)
  }
  return out
}

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === 'node_modules' || entry.name === 'dist') continue
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      walk(full, out)
      continue
    }
    if (/\.(svelte|ts)$/.test(entry.name)) out.push(full)
  }
  return out
}

function collectSourceStrings() {
  const strings = new Set()
  for (const file of walk(srcDir)) {
    const source = fs.readFileSync(file, 'utf8')
    const stringPattern = /['"]([A-Za-z][A-Za-z0-9_-]*(?:\.[A-Za-z0-9_-]+)+)['"]/g
    let match
    while ((match = stringPattern.exec(source)) !== null) {
      strings.add(match[1])
    }
  }
  return strings
}

const localeKeysByFile = new Map()
const allLocaleKeys = new Set()
for (const file of localeFiles) {
  const keys = flattenKeys(readJSON(file))
  localeKeysByFile.set(path.relative(root, file), keys)
  for (const key of keys) allLocaleKeys.add(key)
}

const sourceStrings = collectSourceStrings()
const dynamicUsedKeys = [
  'messageList.composeStatusDraftForward',
  'messageList.composeStatusDraftReply',
  'messageList.composeStatusDraftReplyAll',
  'messageList.composeStatusSentForward',
  'messageList.composeStatusSentReply',
  'messageList.composeStatusSentReplyAll',
]
for (const key of dynamicUsedKeys) sourceStrings.add(key)

const usedKeys = new Set([...sourceStrings].filter((key) => allLocaleKeys.has(key)))
const unusedKeys = [...allLocaleKeys].filter((key) => !usedKeys.has(key)).sort()
const localeNamespaces = new Set([...allLocaleKeys].map((key) => key.split('.')[0]))
const missingKeys = [...sourceStrings]
  .filter((key) => localeNamespaces.has(key.split('.')[0]))
  .filter((key) => !allLocaleKeys.has(key))
  .sort()

const keyOwners = new Map()
for (const [file, keys] of localeKeysByFile) {
  for (const key of keys) {
    if (!keyOwners.has(key)) keyOwners.set(key, [])
    keyOwners.get(key).push(file)
  }
}

function printList(title, items, mapper = (item) => item) {
  console.log(`${title}: ${items.length}`)
  for (const item of items) {
    console.log(`  ${mapper(item)}`)
  }
}

printList('Unused i18n keys', unusedKeys, (key) => {
  return `${key} [${keyOwners.get(key).join(', ')}]`
})
printList('Missing i18n keys referenced by source', missingKeys)

if (unusedKeys.length > 0 || missingKeys.length > 0) {
  process.exitCode = 1
}
