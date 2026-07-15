import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'

import { verifyReleaseTag } from './release-identity.mjs'
import { buildReleaseFromTag, withDetachedTagWorktree } from './release-worktree.mjs'

function writeJSON(file, value) {
  fs.writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`)
}

function createTaggedReleaseRepo() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aulycMail-release-worktree-test-'))
  const git = (...args) => execFileSync('git', args, { cwd: root, encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }).trim()
  fs.mkdirSync(path.join(root, 'frontend'))
  fs.writeFileSync(path.join(root, '.gitignore'), 'build/\nfrontend/node_modules/\n')
  writeJSON(path.join(root, 'version.json'), { version: '1.2.3-dev', build: 0 })
  writeJSON(path.join(root, 'wails.json'), { info: { productVersion: '1.2.3' } })
  writeJSON(path.join(root, 'frontend/package.json'), { version: '1.2.3-dev' })
  writeJSON(path.join(root, 'frontend/package-lock.json'), { version: '1.2.3-dev', packages: { '': { version: '1.2.3-dev' } } })
  fs.writeFileSync(path.join(root, 'CHANGELOG.md'), '# Changelog\n\n## [Unreleased]\n')
  git('init', '-q')
  git('config', 'user.name', 'Release Test')
  git('config', 'user.email', 'release@example.invalid')
  git('add', '.')
  git('commit', '-qm', 'feat: fixture')

  writeJSON(path.join(root, 'version.json'), { version: '1.2.3-beta.1', build: 1 })
  writeJSON(path.join(root, 'frontend/package.json'), { version: '1.2.3-beta.1' })
  writeJSON(path.join(root, 'frontend/package-lock.json'), { version: '1.2.3-beta.1', packages: { '': { version: '1.2.3-beta.1' } } })
  fs.writeFileSync(path.join(root, 'CHANGELOG.md'), '# Changelog\n\n## [Unreleased]\n\n## [1.2.3-beta.1] — 2026-07-14\n\n- Test.\n')
  git('add', 'version.json', 'frontend/package.json', 'frontend/package-lock.json', 'CHANGELOG.md')
  git('commit', '-qm', 'chore: release 1.2.3-beta.1')
  git('tag', '-a', '1.2.3-beta.1', '-m', 'fixture release')
  return { root, git, commit: git('rev-parse', 'HEAD') }
}

test('isolated release callback runs at the exact tag in a detached clean worktree and cleans it', () => {
  const { root, git, commit } = createTaggedReleaseRepo()
  let isolatedPath = ''
  try {
    withDetachedTagWorktree({ repoRoot: root, tag: '1.2.3-beta.1' }, (worktree) => {
      isolatedPath = worktree
      const isolatedGit = (...args) => execFileSync('git', args, { cwd: worktree, encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }).trim()
      assert.equal(isolatedGit('rev-parse', 'HEAD'), commit)
      assert.throws(() => isolatedGit('symbolic-ref', '-q', 'HEAD'))
      fs.mkdirSync(path.join(worktree, 'build'))
      fs.writeFileSync(path.join(worktree, 'build', 'candidate.txt'), 'ok\n')
    })
    assert.equal(fs.existsSync(isolatedPath), false)
    assert.equal(git('worktree', 'list', '--porcelain').split('\n').filter((line) => line.startsWith('worktree ')).length, 1)
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
})

test('isolated release build detects tracked changes and still removes its temporary worktree', () => {
  const { root, git } = createTaggedReleaseRepo()
  let isolatedPath = ''
  try {
    assert.throws(
      () => withDetachedTagWorktree({ repoRoot: root, tag: '1.2.3-beta.1' }, (worktree) => {
        isolatedPath = worktree
        fs.writeFileSync(path.join(worktree, 'version.json'), '{"version":"9.9.9","build":9}\n')
      }),
      /not clean/,
    )
    assert.equal(fs.existsSync(isolatedPath), false)
    assert.equal(git('worktree', 'list', '--porcelain').split('\n').filter((line) => line.startsWith('worktree ')).length, 1)
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
})

test('tag verification rejects a tag that no longer points to caller HEAD', () => {
  const { root, git } = createTaggedReleaseRepo()
  try {
    fs.writeFileSync(path.join(root, 'post-release.txt'), 'change\n')
    git('add', 'post-release.txt')
    git('commit', '-qm', 'fix: after release')
    assert.throws(
      () => verifyReleaseTag(root, '1.2.3-beta.1', { requireHead: true }),
      /but HEAD is/,
    )
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
})

test('isolated release refuses a dirty caller worktree', () => {
  const { root } = createTaggedReleaseRepo()
  try {
    fs.writeFileSync(path.join(root, 'untracked.txt'), 'dirty\n')
    assert.throws(
      () => withDetachedTagWorktree({ repoRoot: root, tag: '1.2.3-beta.1' }, () => {}),
      /Caller worktree must be clean/,
    )
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
})

test('release build refuses to overwrite an existing same-version artifact', () => {
  const { root } = createTaggedReleaseRepo()
  const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aulycMail-release-output-'))
  try {
    fs.writeFileSync(path.join(outputDir, 'aulycMail-1.2.3-beta.1-build.1.dmg'), 'existing\n')
    assert.throws(
      () => buildReleaseFromTag({
        repoRoot: root,
        tag: '1.2.3-beta.1',
        channel: 'test',
        outputDir,
      }),
      /Refusing to overwrite/,
    )
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
    fs.rmSync(outputDir, { recursive: true, force: true })
  }
})
