import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const makefile = fs.readFileSync(new URL('../Makefile', import.meta.url), 'utf8')
const wailsConfig = JSON.parse(fs.readFileSync(new URL('../wails.json', import.meta.url), 'utf8'))
const mainSource = fs.readFileSync(new URL('../main.go', import.meta.url), 'utf8')
const packageScript = fs.readFileSync(new URL('./package_macos_app.sh', import.meta.url), 'utf8')
const dmgPackageScript = fs.readFileSync(new URL('./package_macos_dmg.sh', import.meta.url), 'utf8')
const legalVerifier = fs.readFileSync(new URL('./verify_legal_resources.sh', import.meta.url), 'utf8')
const proprietaryLicense = fs.readFileSync(new URL('../LICENSE', import.meta.url), 'utf8')
const thirdPartyNotices = fs.readFileSync(new URL('../THIRD_PARTY_NOTICES.md', import.meta.url), 'utf8')
const aerionLicense = fs.readFileSync(new URL('../LICENSES/Aerion-Apache-2.0.txt', import.meta.url), 'utf8')
const aerionModifications = fs.readFileSync(new URL('../AERION_MODIFICATIONS.md', import.meta.url), 'utf8')
const infoTemplate = fs.readFileSync(new URL('../build/darwin/Info.plist', import.meta.url), 'utf8')
const devInfoTemplate = fs.readFileSync(new URL('../build/darwin/Info.dev.plist', import.meta.url), 'utf8')

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

test('macOS bundles advertise regular files as an alternate attachment handler', () => {
  for (const metadataSource of [packageScript, infoTemplate, devInfoTemplate]) {
    assert.match(metadataSource, /<key>CFBundleDocumentTypes<\/key>/)
    assert.match(metadataSource, /<string>public\.data<\/string>/)
    assert.match(metadataSource, /<key>CFBundleTypeRole<\/key>\s*<string>Viewer<\/string>/)
    assert.match(metadataSource, /<key>LSHandlerRank<\/key>\s*<string>Alternate<\/string>/)
  }
  assert.match(mainSource, /OnFileOpen:\s*func\(filePath string\)/)
  assert.match(mainSource, /app\.HandleFileOpen\(application, filePath\)/)
})

test('macOS bundles fail closed on complete Aerion legal resources', () => {
  assert.match(proprietaryLicense, /Nothing in this\s+license limits rights granted directly by those third-party licenses\./)
  assert.match(thirdPartyNotices, /https:\/\/github\.com\/hkdb\/aerion/)
  assert.match(thirdPartyNotices, /Copyright 2024-2025 Aerion Contributors/)
  assert.match(aerionLicense, /APPENDIX: How to apply the Apache License to your work\./)
  assert.match(aerionLicense, /Copyright 2024-2025 Aerion Contributors/)
  assert.match(aerionModifications, /modified by aulyc beginning in 2026/)
  assert.match(aerionModifications, /not affiliated with, sponsored by,\s+or endorsed by/)

  for (const bundledName of [
    'LICENSE.txt',
    'THIRD_PARTY_NOTICES.md',
    'Aerion-Apache-2.0.txt',
    'AERION_MODIFICATIONS.md',
    'Nunito-OFL.txt',
  ]) {
    assert.match(packageScript, new RegExp(bundledName.replaceAll('.', '\\.')))
    assert.match(legalVerifier, new RegExp(bundledName.replaceAll('.', '\\.')))
  }
  assert.match(packageScript, /Missing or empty required legal file/)
  assert.match(packageScript, /verify_legal_resources\.sh/)
})

test('DMG layout waits for Finder to register the mounted installer volume', () => {
  assert.match(dmgPackageScript, /repeat 20 times/)
  assert.match(dmgPackageScript, /set finderDisk to get disk diskName/)
  assert.match(dmgPackageScript, /delay 0\.25/)
  assert.match(dmgPackageScript, /Finder did not register mounted disk/)
})
