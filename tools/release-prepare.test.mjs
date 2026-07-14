import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'

import {
  bumpBaseVersion,
  foldUnreleasedIntoVersion,
  inferBumpLevel,
  nextReleaseVersion,
  prepareRelease,
  promoteUnreleased,
} from './release-prepare.mjs'

test('increments standard SemVer levels without decimal rollover shortcuts', () => {
  assert.equal(bumpBaseVersion('2.0.9', 'patch'), '2.0.10')
  assert.equal(bumpBaseVersion('2.4.3', 'minor'), '2.5.0')
  assert.equal(bumpBaseVersion('2.4.3', 'major'), '3.0.0')
})

test('infers the highest release impact from mixed commit styles', () => {
  assert.equal(inferBumpLevel('Fix settings spacing'), 'patch')
  assert.equal(inferBumpLevel('fix: repair sync\n\nfeat(mail): add filters'), 'minor')
  assert.equal(inferBumpLevel('feat!: replace account storage'), 'major')
  assert.equal(inferBumpLevel('refactor: storage\n\nBREAKING CHANGE: schema replaced'), 'major')
})

test('selects test and formal versions from the current lifecycle stage', () => {
  assert.equal(nextReleaseVersion('0.3.92-dev', 'test'), '0.3.92-beta.1')
  assert.equal(nextReleaseVersion('0.3.92-beta.1', 'test'), '0.3.92-beta.2')
  assert.equal(nextReleaseVersion('0.3.92-rc.2', 'test'), '0.3.92-rc.3')
  assert.equal(nextReleaseVersion('0.3.92-beta.2', 'formal'), '0.3.92')
  assert.equal(nextReleaseVersion('2.0.9', 'test', 'patch'), '2.0.10-beta.1')
  assert.equal(nextReleaseVersion('2.0.9', 'formal', 'minor'), '2.1.0')
})

test('promotes Unreleased notes into an exact dated release heading', () => {
  const source = '# Changelog\n\n## [Unreleased]\n\n### Fixed\n\n- Repair sync.\n\n## [0.3.91]\n\n- Previous.\n'
  const result = promoteUnreleased(source, '0.3.92-beta.1', '2026-07-14', 'test', '0.3.92-dev')
  assert.match(result, /## \[Unreleased\]\n\n## \[0\.3\.92-beta\.1\] — 2026-07-14\n\n### Fixed/)
  assert.match(result, /## \[0\.3\.91\]\n\n- Previous\./)
})

test('adds a meaningful note when promoting a prerelease without new notes', () => {
  const source = '# Changelog\n\n## [Unreleased]\n\n## [0.3.92-beta.1]\n\n- Test release.\n'
  const result = promoteUnreleased(source, '0.3.92', '2026-07-14', 'formal', '0.3.92-beta.1')
  assert.match(result, /## \[0\.3\.92\].*\n\n- Promoted 0\.3\.92-beta\.1 to a stable release\./s)
})

test('folds retry notes into an unpublished release without changing its identity', () => {
  const source = '# Changelog\n\n## [Unreleased]\n\n### Fixed\n\n- Gate failure fix.\n\n## [0.3.92-beta.1] — 2026-07-14\n\n- Initial test notes.\n'
  const result = foldUnreleasedIntoVersion(source, '0.3.92-beta.1')
  assert.match(result, /## \[Unreleased\]\n\n## \[0\.3\.92-beta\.1\]/)
  assert.match(result, /### Fixed\n\n- Gate failure fix\.\n\n- Initial test notes\./)
})

test('prepares test then formal metadata from committed Git states', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aulycmail-release-prepare-'))
  const git = (...args) => execFileSync('git', args, { cwd: root, stdio: 'ignore' })
  try {
    fs.mkdirSync(path.join(root, 'frontend'))
    fs.writeFileSync(path.join(root, 'version.json'), '{"version":"0.3.92-dev","build":0}\n')
    fs.writeFileSync(path.join(root, 'wails.json'), '{"info":{"productVersion":"0.3.92"}}\n')
    fs.writeFileSync(path.join(root, 'frontend/package.json'), '{"version":"0.3.92-dev"}\n')
    fs.writeFileSync(path.join(root, 'frontend/package-lock.json'), '{"version":"0.3.92-dev","packages":{"":{"version":"0.3.92-dev"}}}\n')
    fs.writeFileSync(path.join(root, 'CHANGELOG.md'), '# Changelog\n\n## [Unreleased]\n\n### Changed\n\n- New release flow.\n')
    git('init', '-q')
    git('config', 'user.name', 'Release Test')
    git('config', 'user.email', 'release@example.invalid')
    git('add', '.')
    git('commit', '-qm', 'fix: prepare release flow')

    const testRelease = prepareRelease({ root, channel: 'test', date: '2026-07-14' })
    assert.deepEqual(testRelease, { version: '0.3.92-beta.1', build: 1, alreadyPrepared: false })
    git('add', '.')
    git('commit', '-qm', 'chore: release 0.3.92-beta.1')
    git('tag', '-a', '0.3.92-beta.1', '-m', 'test release')

    const formalRelease = prepareRelease({ root, channel: 'formal', date: '2026-07-14' })
    assert.deepEqual(formalRelease, { version: '0.3.92', build: 2, alreadyPrepared: false })
    assert.match(fs.readFileSync(path.join(root, 'CHANGELOG.md'), 'utf8'), /## \[0\.3\.92\].*Promoted 0\.3\.92-beta\.1/s)
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
})
