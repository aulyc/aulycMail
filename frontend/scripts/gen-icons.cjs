#!/usr/bin/env node
// Scan the source for Iconify icon references and emit a minimal offline
// collection containing ONLY the icons actually used. Run before build to keep
// src/lib/iconify-subset.json in sync. Replaces full @iconify-json sets (~16MB)
// with a tiny subset (~tens of KB).
const fs = require('fs')
const path = require('path')

const ROOTS = ['src', path.join('..', 'extensions')]
const PREFIXES = ['mdi', 'lucide', 'heroicons', 'logos', 'simple-icons']
const RE = new RegExp(`['"\`](${PREFIXES.join('|')}):([a-z0-9-]+)['"\`]`, 'g')

function walk(dir, acc) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name)
    if (e.isDirectory()) { if (e.name !== 'node_modules') walk(p, acc) }
    else if (/\.(svelte|ts|js)$/.test(e.name)) {
      const txt = fs.readFileSync(p, 'utf8')
      let m; while ((m = RE.exec(txt))) acc.add(m[1] + ':' + m[2])
    }
  }
}

const used = new Set()
for (const r of ROOTS) { if (fs.existsSync(r)) walk(r, used) }

const byPrefix = {}
for (const u of used) { const [p, n] = u.split(':'); (byPrefix[p] = byPrefix[p] || []).push(n) }

const out = {}
let total = 0, missing = []
for (const p of PREFIXES) {
  const names = byPrefix[p]
  if (!names || !names.length) continue
  const full = require(`@iconify-json/${p}/icons.json`)
  const sub = { prefix: full.prefix, icons: {}, aliases: {} }
  if (full.width) sub.width = full.width
  if (full.height) sub.height = full.height
  const seen = new Set(); const queue = [...names]
  while (queue.length) {
    const n = queue.shift(); if (seen.has(n)) continue; seen.add(n)
    if (full.icons[n]) sub.icons[n] = full.icons[n]
    else if (full.aliases && full.aliases[n]) { sub.aliases[n] = full.aliases[n]; queue.push(full.aliases[n].parent) }
    else missing.push(p + ':' + n)
  }
  if (!Object.keys(sub.aliases).length) delete sub.aliases
  out[p] = sub
  total += Object.keys(sub.icons).length
}
fs.writeFileSync('src/lib/iconify-subset.json', JSON.stringify(out))
console.log(`Wrote ${total} icons across ${Object.keys(out).length} collections to src/lib/iconify-subset.json`)
if (missing.length) console.warn('MISSING (not found in sets):', missing.join(', '))
