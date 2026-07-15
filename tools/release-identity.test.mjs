import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'

import {
  REQUIRED_MANIFEST_FIELDS,
  validateManifest,
  verifyManifestGit,
} from './release-identity.mjs'

function testManifest(overrides = {}) {
  return {
    application: 'aulycmail',
    releaseProfile: 'macos-arm64-app',
    version: '1.2.3-beta.1',
    buildNumber: 7,
    releaseChannel: 'test',
    tag: '1.2.3-beta.1',
    commit: 'a'.repeat(40),
    dirty: false,
    artifact: 'aulycmail-1.2.3-beta.1-build.7.dmg',
    sha256: 'b'.repeat(64),
    architecture: 'arm64',
    bundleIdentifier: 'com.aulyc.aulycmail',
    teamIdentifier: null,
    minimumSystemVersion: '11.0',
    signatureType: 'adhoc',
    hardenedRuntime: false,
    notarized: false,
    notarizationSubmissionId: null,
    builtAt: '2026-07-14T02:00:00Z',
    ...overrides,
  }
}

function writeJSON(file, value) {
  fs.writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`)
}

function createTaggedReleaseRepo() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aulycmail-release-identity-'))
  const git = (...args) => execFileSync('git', args, { cwd: root, encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }).trim()
  fs.mkdirSync(path.join(root, 'frontend'))
  fs.writeFileSync(path.join(root, '.gitignore'), 'build/\n')
  writeJSON(path.join(root, 'version.json'), { version: '1.2.3-dev', build: 0 })
  writeJSON(path.join(root, 'wails.json'), { info: { productVersion: '1.2.3' } })
  writeJSON(path.join(root, 'frontend/package.json'), { version: '1.2.3-dev' })
  writeJSON(path.join(root, 'frontend/package-lock.json'), { version: '1.2.3-dev', packages: { '': { version: '1.2.3-dev' } } })
  fs.writeFileSync(path.join(root, 'CHANGELOG.md'), '# Changelog\n\n## [Unreleased]\n')
  git('init', '-q')
  git('config', 'user.name', 'Release Test')
  git('config', 'user.email', 'release@example.invalid')
  git('add', '.')
  git('commit', '-qm', 'feat: initial fixture')

  writeJSON(path.join(root, 'version.json'), { version: '1.2.3-beta.1', build: 7 })
  writeJSON(path.join(root, 'frontend/package.json'), { version: '1.2.3-beta.1' })
  writeJSON(path.join(root, 'frontend/package-lock.json'), { version: '1.2.3-beta.1', packages: { '': { version: '1.2.3-beta.1' } } })
  fs.writeFileSync(path.join(root, 'CHANGELOG.md'), '# Changelog\n\n## [Unreleased]\n\n## [1.2.3-beta.1] — 2026-07-14\n\n- Test.\n')
  git('add', 'version.json', 'frontend/package.json', 'frontend/package-lock.json', 'CHANGELOG.md')
  git('commit', '-qm', 'chore: release 1.2.3-beta.1')
  git('tag', '-a', '1.2.3-beta.1', '-m', 'fixture release')
  return { root, commit: git('rev-parse', 'HEAD') }
}

test('test release manifest contains and validates every required identity field', () => {
  const manifest = testManifest()
  assert.deepEqual(REQUIRED_MANIFEST_FIELDS.filter((field) => !Object.hasOwn(manifest, field)), [])
  assert.equal(validateManifest(manifest), manifest)
})

test('formal release manifest requires Developer ID, runtime, Team ID, and notarization', () => {
  const manifest = testManifest({
    version: '1.2.3',
    releaseChannel: 'formal',
    tag: '1.2.3',
    artifact: 'aulycmail-1.2.3-build.7.dmg',
    teamIdentifier: 'M9M7M2ARFD',
    signatureType: 'developer-id',
    hardenedRuntime: true,
    notarized: true,
    notarizationSubmissionId: '37b552b8-0ba7-4778-a45c-8d3f0d667746',
  })
  assert.equal(validateManifest(manifest), manifest)
  assert.throws(() => validateManifest({ ...manifest, hardenedRuntime: false }), /hardenedRuntime=true/)
  assert.throws(() => validateManifest({ ...manifest, notarizationSubmissionId: null }), /non-empty string/)
})

test('manifest validation rejects missing fields, dirty releases, and channel impersonation', () => {
  const missing = testManifest()
  delete missing.minimumSystemVersion
  assert.throws(() => validateManifest(missing), /missing fields: minimumSystemVersion/)
  assert.throws(() => validateManifest(testManifest({ dirty: true })), /dirty=false/)
  assert.throws(() => validateManifest(testManifest({ signatureType: 'developer-id' })), /signatureType=adhoc/)
  assert.throws(() => validateManifest(testManifest({ architecture: 'x86_64' })), /arm64 only/)
  assert.throws(
    () => validateManifest(testManifest({ releaseProfile: 'obsidian-plugin' })),
    /releaseProfile must be macos-arm64-app/,
  )
})

test('manifest Git verification binds tag, commit, version, build, and release commit', () => {
  const { root, commit } = createTaggedReleaseRepo()
  try {
    const manifest = testManifest({ commit })
    assert.doesNotThrow(() => verifyManifestGit(manifest, root))
    assert.throws(
      () => verifyManifestGit({ ...manifest, commit: 'f'.repeat(40) }, root),
      /does not match tag/,
    )
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
})
