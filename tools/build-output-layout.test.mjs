import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const makefile = fs.readFileSync(new URL('../Makefile', import.meta.url), 'utf8')
const wailsConfig = JSON.parse(fs.readFileSync(new URL('../wails.json', import.meta.url), 'utf8'))

test('local app bundles use hidden output paths and clean the legacy visible path', () => {
  assert.match(makefile, /^WAILS_BUILD_DIR := \.cache\/wails$/m)
  assert.match(makefile, /^BUILD_OUTPUT_DIR := \.cache\/build$/m)
  assert.match(makefile, /^APP_BUNDLE := \$\(BUILD_OUTPUT_DIR\)\/aulycMail\.app$/m)
  assert.match(makefile, /^OBSOLETE_BUILD_OUTPUT_DIR := build\/bin$/m)
  assert.match(makefile, /^build-app: remove-obsolete-build-output$/m)
  assert.match(makefile, /^prepare-wails-build-assets: remove-obsolete-build-output$/m)
  assert.match(makefile, /^clean: remove-obsolete-build-output$/m)
  assert.equal(wailsConfig['build:dir'], '.cache/wails')
})
