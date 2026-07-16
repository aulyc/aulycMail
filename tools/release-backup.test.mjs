import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'

import { pushFormalReleaseBackup, verifyBackupRemote } from './release-backup.mjs'

function writeJSON(file, value) {
  fs.writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`)
}

function createFormalReleaseRepo() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aulycMail-release-backup-test-'))
  const remote = fs.mkdtempSync(path.join(os.tmpdir(), 'aulycMail-release-backup-remote-'))
  const git = (...args) => execFileSync('git', args, {
    cwd: root,
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'ignore'],
  }).trim()
  const remoteGit = (...args) => execFileSync('git', [`--git-dir=${remote}`, ...args], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'ignore'],
  }).trim()

  fs.mkdirSync(path.join(root, 'frontend'))
  writeJSON(path.join(root, 'version.json'), { version: '1.2.3-dev', build: 0 })
  writeJSON(path.join(root, 'wails.json'), { info: { productVersion: '1.2.3' } })
  writeJSON(path.join(root, 'frontend/package.json'), { version: '1.2.3-dev' })
  writeJSON(path.join(root, 'frontend/package-lock.json'), {
    version: '1.2.3-dev',
    packages: { '': { version: '1.2.3-dev' } },
  })
  fs.writeFileSync(path.join(root, 'CHANGELOG.md'), '# Changelog\n\n## [Unreleased]\n')

  git('init', '-q')
  git('config', 'user.name', 'Release Backup Test')
  git('config', 'user.email', 'release-backup@example.invalid')
  git('add', '.')
  git('commit', '-qm', 'feat: fixture')
  git('branch', '-M', 'main')

  writeJSON(path.join(root, 'version.json'), { version: '1.2.3', build: 1 })
  writeJSON(path.join(root, 'frontend/package.json'), { version: '1.2.3' })
  writeJSON(path.join(root, 'frontend/package-lock.json'), {
    version: '1.2.3',
    packages: { '': { version: '1.2.3' } },
  })
  fs.writeFileSync(path.join(root, 'CHANGELOG.md'), '# Changelog\n\n## [Unreleased]\n\n## [1.2.3] — 2026-07-16\n\n- Test.\n')
  git('add', 'version.json', 'frontend/package.json', 'frontend/package-lock.json', 'CHANGELOG.md')
  git('commit', '-qm', 'chore: release 1.2.3')
  git('tag', '-a', '1.2.3', '-m', 'fixture release')

  execFileSync('git', ['init', '--bare', '-q', remote])
  git('remote', 'add', 'backup', remote)

  return {
    root,
    remote,
    git,
    remoteGit,
    commit: git('rev-parse', 'HEAD'),
  }
}

function cleanupFixture(fixture) {
  fs.rmSync(fixture.root, { recursive: true, force: true })
  fs.rmSync(fixture.remote, { recursive: true, force: true })
}

test('backup preflight accepts an empty reachable remote from main', () => {
  const fixture = createFormalReleaseRepo()
  try {
    const result = verifyBackupRemote(fixture.root)
    assert.equal(result.remote, 'backup')
    assert.equal(result.branch, 'main')
    assert.equal(result.remoteUrl, fixture.remote)
  } finally {
    cleanupFixture(fixture)
  }
})

test('backup preflight rejects a formal release from another branch', () => {
  const fixture = createFormalReleaseRepo()
  try {
    fixture.git('switch', '-qc', 'feature')
    assert.throws(
      () => verifyBackupRemote(fixture.root),
      /must run from branch main/,
    )
  } finally {
    cleanupFixture(fixture)
  }
})

test('formal backup push atomically publishes main and the annotated release tag', () => {
  const fixture = createFormalReleaseRepo()
  try {
    const result = pushFormalReleaseBackup(fixture.root, { tag: '1.2.3' })
    assert.equal(result.commit, fixture.commit)
    assert.equal(fixture.remoteGit('rev-parse', 'refs/heads/main'), fixture.commit)
    assert.equal(fixture.remoteGit('rev-parse', 'refs/tags/1.2.3^{}'), fixture.commit)
  } finally {
    cleanupFixture(fixture)
  }
})
