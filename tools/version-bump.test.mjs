import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'

import {
  localVersion,
  parseBuildNumber,
  parseVersion,
  run,
  syncDerivedFiles,
  verifyDerivedFiles,
  verifyRelease,
} from './version-bump.mjs'

test('accepts stable and supported prerelease versions', () => {
  assert.equal(parseVersion('2.0.10').base, '2.0.10')
  assert.equal(parseVersion('2.2.0-dev').isDev, true)
  assert.equal(parseVersion('2.2.0-beta.12').prerelease, 'beta.12')
  assert.equal(parseVersion('2.2.0-rc.1').base, '2.2.0')
})

test('rejects nonstandard or ambiguous versions', () => {
  for (const version of ['2.2', 'v2.2.0', '2.02.0', '2.2.0-beta', '2.2.0-rc.0', '2.2.0+build']) {
    assert.throws(() => parseVersion(version), /Invalid version/)
  }
  assert.throws(() => parseBuildNumber(-1), /Invalid build number/)
})

test('adds commit metadata to local runtime versions', () => {
  assert.equal(localVersion('0.3.92-dev', 'ABCDEF1'), '0.3.92-dev+abcdef1')
  assert.equal(localVersion('0.3.92-beta.1', 'abcdef1'), '0.3.92-beta.1+local.abcdef1')
  assert.equal(localVersion('0.3.92-dev', 'abcdef1', true), '0.3.92-dev+abcdef1.dirty')
})

test('retains the global build counter and refuses to move it backwards', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aulycmail-version-counter-'))
  const oldLog = console.log
  try {
    fs.mkdirSync(path.join(root, 'frontend'))
    fs.writeFileSync(path.join(root, 'version.json'), '{"version":"2.0.10","build":128}\n')
    fs.writeFileSync(path.join(root, 'wails.json'), '{"info":{"productVersion":"2.0.10"}}\n')
    fs.writeFileSync(path.join(root, 'frontend/package.json'), '{"version":"2.0.10"}\n')
    fs.writeFileSync(path.join(root, 'frontend/package-lock.json'), '{"version":"2.0.10","packages":{"":{"version":"2.0.10"}}}\n')
    console.log = () => {}

    run(['set', '2.0.11-dev'], root)
    assert.deepEqual(JSON.parse(fs.readFileSync(path.join(root, 'version.json'))), {
      version: '2.0.11-dev',
      build: 128,
    })
    assert.throws(() => run(['set', '2.0.11-beta.1', '--build', '127'], root), /cannot move backwards/)
  } finally {
    console.log = oldLog
    fs.rmSync(root, { recursive: true, force: true })
  }
})

test('synchronizes and detects drift in derived version files', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aulycmail-version-'))
  try {
    fs.mkdirSync(path.join(root, 'frontend'))
    fs.writeFileSync(path.join(root, 'version.json'), '{"version":"2.0.10-beta.2","build":128}\n')
    fs.writeFileSync(path.join(root, 'wails.json'), '{"info":{"productVersion":"0.0.0"}}\n')
    fs.writeFileSync(path.join(root, 'frontend/package.json'), '{"name":"fixture","version":"0.0.0"}\n')
    fs.writeFileSync(path.join(root, 'frontend/package-lock.json'), '{"version":"0.0.0","packages":{"":{"version":"0.0.0"}}}\n')

    syncDerivedFiles(root)
    assert.equal(verifyDerivedFiles(root).version, '2.0.10-beta.2')
    assert.equal(JSON.parse(fs.readFileSync(path.join(root, 'wails.json'))).info.productVersion, '2.0.10')

    const packageJSON = JSON.parse(fs.readFileSync(path.join(root, 'frontend/package.json')))
    packageJSON.version = '9.9.9'
    fs.writeFileSync(path.join(root, 'frontend/package.json'), `${JSON.stringify(packageJSON, null, 2)}\n`)
    assert.throws(() => verifyDerivedFiles(root), /drift detected/)
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
})

test('release verification requires a positive build and exact changelog heading', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aulycmail-release-version-'))
  try {
    fs.mkdirSync(path.join(root, 'frontend'))
    fs.writeFileSync(path.join(root, 'version.json'), '{"version":"2.0.10-rc.1","build":129}\n')
    fs.writeFileSync(path.join(root, 'wails.json'), '{"info":{"productVersion":"0.0.0"}}\n')
    fs.writeFileSync(path.join(root, 'frontend/package.json'), '{"version":"0.0.0"}\n')
    fs.writeFileSync(path.join(root, 'frontend/package-lock.json'), '{"version":"0.0.0","packages":{"":{"version":"0.0.0"}}}\n')
    fs.writeFileSync(path.join(root, 'CHANGELOG.md'), '# Changelog\n\n## [2.0.10-rc.1] — 2026\n')

    syncDerivedFiles(root)
    assert.equal(verifyRelease(root).build, 129)
    fs.writeFileSync(path.join(root, 'CHANGELOG.md'), '# Changelog\n\n## [2.0.10-rc.2]\n')
    assert.throws(() => verifyRelease(root), /missing a heading/)
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
})
