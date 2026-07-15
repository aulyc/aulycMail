#!/usr/bin/env node

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url))
const DEFAULT_ROOT = path.resolve(SCRIPT_DIR, '..')

function containsFile(directory) {
  if (!fs.existsSync(directory)) return false
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const entryPath = path.join(directory, entry.name)
    if (entry.isFile()) return true
    if (entry.isDirectory() && containsFile(entryPath)) return true
  }
  return false
}

// Go validates //go:embed patterns while Wails generates bindings. A clean
// checkout intentionally has no ignored frontend/dist build output, so create
// a harmless bootstrap file only when the embed tree is otherwise empty. The
// subsequent Vite production build replaces the entire dist directory.
export function ensureFrontendDist(root = DEFAULT_ROOT) {
  const dist = path.join(path.resolve(root), 'frontend', 'dist')
  if (containsFile(dist)) return false

  fs.mkdirSync(dist, { recursive: true })
  fs.writeFileSync(
    path.join(dist, 'index.html'),
    '<!doctype html><meta charset="utf-8"><title>aulycmail build bootstrap</title>\n',
  )
  return true
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  if (ensureFrontendDist()) {
    console.log('Prepared temporary frontend/dist content for Wails generation.')
  }
}
