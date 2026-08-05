import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'

import { main as runBackup, pushFormalReleaseBackup, verifyBackupRemote } from './release-backup.mjs'
import {
  CURRENT_APPLICATION,
  isGitWorktreeClean,
  run as runIdentity,
  validateManifest,
  verifyManifestFile,
  verifyManifestGit,
  verifyReleaseCommit,
  verifyReleasePreflight,
  verifyReleaseTag,
  verifyTaggedSource,
} from './release-identity.mjs'
import {
  bumpBaseVersion,
  foldUnreleasedIntoVersion,
  nextReleaseVersion,
  prepareRelease,
  promoteUnreleased,
  run as runPrepare,
} from './release-prepare.mjs'
import { buildReleaseFromTag, run as runWorktree, withDetachedTagWorktree } from './release-worktree.mjs'
import {
  loadVersionState,
  localVersion,
  parseBuildNumber,
  run as runVersion,
  setVersion,
  syncDerivedFiles,
  verifyDerivedFiles,
  verifyRelease,
} from './version-bump.mjs'

function writeJSON(file, value) {
  fs.mkdirSync(path.dirname(file), { recursive: true })
  fs.writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`)
}

function versionFixture(t, {
  version = '1.2.3',
  build = 7,
  changelog = `# Changelog\n\n## [Unreleased]\n\n## [${version}] — 2026-08-03\n\n- Fixture.\n`,
} = {}) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aulycmail-tool-coverage-'))
  t.after(() => fs.rmSync(root, { recursive: true, force: true }))
  const base = version.match(/^\d+\.\d+\.\d+/)?.[0] ?? '1.2.3'
  writeJSON(path.join(root, 'version.json'), { version, build })
  writeJSON(path.join(root, 'wails.json'), { info: { productVersion: base } })
  writeJSON(path.join(root, 'frontend/package.json'), { name: 'fixture', version })
  writeJSON(path.join(root, 'frontend/package-lock.json'), {
    name: 'fixture',
    version,
    packages: { '': { name: 'fixture', version } },
  })
  fs.writeFileSync(path.join(root, 'CHANGELOG.md'), changelog)
  return root
}

function captureLogs(callback) {
  const original = console.log
  const logs = []
  console.log = (...args) => logs.push(args.join(' '))
  try {
    return { result: callback(), logs }
  } finally {
    console.log = original
  }
}

function manifest(overrides = {}) {
  return {
    application: CURRENT_APPLICATION,
    releaseProfile: 'macos-arm64-app',
    version: '1.2.3-beta.1',
    buildNumber: 7,
    releaseChannel: 'test',
    tag: '1.2.3-beta.1',
    commit: 'a'.repeat(40),
    dirty: false,
    artifact: 'aulycMail-1.2.3-beta.1-build.7.dmg',
    sha256: 'b'.repeat(64),
    architecture: 'arm64',
    bundleIdentifier: 'com.aulyc.aulycmail',
    teamIdentifier: null,
    minimumSystemVersion: '11.0',
    signatureType: 'adhoc',
    hardenedRuntime: false,
    notarized: false,
    notarizationSubmissionId: null,
    builtAt: '2026-08-03T00:00:00Z',
    ...overrides,
  }
}

function git(root, ...args) {
  return execFileSync('git', args, {
    cwd: root,
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'ignore'],
  }).trim()
}

function taggedRepo(t, { version = '1.2.3-beta.1', build = 7 } = {}) {
  const root = versionFixture(t, {
    version: '1.2.3-dev',
    build: 0,
    changelog: '# Changelog\n\n## [Unreleased]\n',
  })
  fs.writeFileSync(path.join(root, '.gitignore'), 'frontend/node_modules\noutput/\n')
  git(root, 'init', '-q')
  git(root, 'config', 'user.name', 'Coverage Test')
  git(root, 'config', 'user.email', 'coverage@example.invalid')
  git(root, 'add', '.')
  git(root, 'commit', '-qm', 'feat: fixture')

  setVersion(root, version, build)
  fs.writeFileSync(
    path.join(root, 'CHANGELOG.md'),
    `# Changelog\n\n## [Unreleased]\n\n## [${version}] — 2026-08-03\n\n- Fixture.\n`,
  )
  git(root, 'add', 'version.json', 'wails.json', 'frontend/package.json', 'frontend/package-lock.json', 'CHANGELOG.md')
  git(root, 'commit', '-qm', `chore: release ${version}`)
  git(root, 'tag', '-a', version, '-m', 'fixture release')
  return { root, version, build, commit: git(root, 'rev-parse', 'HEAD') }
}

test('version tool covers every command and rejects malformed state', (t) => {
  const root = versionFixture(t)
  const output = captureLogs(() => {
    runVersion(['get', 'version'], root)
    runVersion(['get', 'base'], root)
    runVersion(['get', 'build'], root)
    runVersion(['local-version', 'ABCDEF1', '--dirty'], root)
    runVersion(['verify'], root)
    runVersion(['verify-release'], root)
    runVersion(['sync'], root)
    runVersion(['next-build'], root)
    runVersion(['set', '1.2.4-dev', '--build', '9'], root)
    runVersion(['help'], root)
  })
  assert.match(output.logs.join('\n'), /1\.2\.3/)
  assert.equal(loadVersionState(root).build, 9)
  assert.equal(loadVersionState(root).version, '1.2.4-dev')

  assert.throws(() => runVersion(['get', 'unknown'], root), /get requires/)
  assert.throws(() => runVersion(['local-version', 'not-a-sha'], root), /Invalid commit/)
  assert.throws(() => runVersion(['set'], root), /requires a version/)
  assert.throws(() => runVersion(['set', '1.2.4', '--build'], root), /requires a number/)
  assert.throws(() => runVersion(['unknown'], root), /Unknown command/)
  assert.throws(() => runVersion([], root), /command is required/)
  assert.throws(() => verifyRelease(root), /-dev suffix/)
  assert.throws(() => parseBuildNumber(Number.NaN), /Invalid build number/)
  assert.throws(() => localVersion('1.2.3', ''), /Invalid commit/)
})

test('version metadata reports unreadable JSON, missing lock roots, zero builds, and drift', (t) => {
  const root = versionFixture(t, { version: '1.2.3-dev', build: 0 })
  fs.writeFileSync(path.join(root, 'version.json'), '{broken')
  assert.throws(() => loadVersionState(root), /Cannot read/)

  writeJSON(path.join(root, 'version.json'), { version: '1.2.3', build: 0 })
  writeJSON(path.join(root, 'wails.json'), {})
  writeJSON(path.join(root, 'frontend/package-lock.json'), { version: '1.2.3', packages: {} })
  assert.throws(() => syncDerivedFiles(root), /missing the root package entry/)

  writeJSON(path.join(root, 'frontend/package-lock.json'), {
    version: '1.2.3',
    packages: { '': { version: '1.2.3' } },
  })
  syncDerivedFiles(root)
  assert.equal(JSON.parse(fs.readFileSync(path.join(root, 'wails.json'))).info.productVersion, '1.2.3')
  assert.throws(() => verifyRelease(root), /positive build number/)
  fs.appendFileSync(path.join(root, 'frontend/package.json'), ' ')
  assert.throws(() => verifyDerivedFiles(root), /drift detected/)
})

test('manifest validation covers local, test, and formal policy edges', () => {
  assert.throws(() => validateManifest(null), /JSON object/)
  assert.throws(() => validateManifest([]), /JSON object/)
  for (const field of [
    'application',
    'releaseProfile',
    'version',
    'commit',
    'artifact',
    'sha256',
    'architecture',
    'bundleIdentifier',
    'minimumSystemVersion',
    'signatureType',
    'builtAt',
  ]) {
    assert.throws(() => validateManifest(manifest({ [field]: '' })), new RegExp(field))
  }
  for (const field of ['dirty', 'hardenedRuntime', 'notarized']) {
    assert.throws(() => validateManifest(manifest({ [field]: 'false' })), new RegExp(field))
  }
  for (const value of [-1, 1.5, Number.MAX_SAFE_INTEGER + 1]) {
    assert.throws(() => validateManifest(manifest({ buildNumber: value })), /buildNumber/)
  }
  assert.throws(() => validateManifest(manifest({ commit: 'A'.repeat(40) })), /Git object ID/)
  assert.throws(() => validateManifest(manifest({ sha256: 'b'.repeat(63) })), /64 lowercase/)
  assert.throws(() => validateManifest(manifest({ bundleIdentifier: 'invalid' })), /bundleIdentifier/)
  assert.throws(() => validateManifest(manifest({ minimumSystemVersion: 'macOS 11' })), /minimumSystemVersion/)
  assert.throws(() => validateManifest(manifest({ builtAt: 'not-a-date' })), /builtAt/)
  assert.throws(() => validateManifest(manifest({ releaseChannel: 'nightly' })), /releaseChannel/)

  const local = manifest({
    version: '1.2.3-dev',
    buildNumber: 0,
    releaseChannel: 'local',
    tag: null,
    dirty: true,
    artifact: 'local.dmg',
    signatureType: 'unsigned',
  })
  assert.equal(validateManifest(local), local)
  assert.equal(validateManifest({ ...local, signatureType: 'adhoc' }).signatureType, 'adhoc')
  assert.throws(() => validateManifest({ ...local, tag: '1.2.3' }), /must not claim a release tag/)
  assert.throws(() => validateManifest({ ...local, signatureType: 'developer-id' }), /unsigned or ad-hoc/)
  assert.throws(() => validateManifest({ ...local, notarized: true }), /must not claim notarization/)
  assert.throws(() => validateManifest({ ...local, notarizationSubmissionId: 'submission' }), /must not claim notarization/)

  assert.throws(() => validateManifest(manifest({ tag: 'v1.2.3-beta.1' })), /tag must exactly match/)
  assert.throws(() => validateManifest(manifest({ buildNumber: 0 })), /positive buildNumber/)
  assert.throws(() => validateManifest(manifest({ artifact: 'wrong.dmg' })), /Manifest artifact/)
  assert.throws(() => validateManifest(manifest({ version: '1.2.3', tag: '1.2.3', artifact: 'aulycMail-1.2.3-build.7.dmg' })), /Test releases require/)
  assert.throws(() => validateManifest(manifest({ teamIdentifier: 'TEAM' })), /must not claim a Team ID/)
  assert.throws(() => validateManifest(manifest({ hardenedRuntime: true })), /hardenedRuntime=false/)
  assert.throws(() => validateManifest(manifest({ notarized: true })), /must not claim Apple notarization/)

  const formal = manifest({
    version: '1.2.3',
    releaseChannel: 'formal',
    tag: '1.2.3',
    artifact: 'aulycMail-1.2.3-build.7.dmg',
    teamIdentifier: 'TEAMID',
    signatureType: 'developer-id',
    hardenedRuntime: true,
    notarized: true,
    notarizationSubmissionId: 'submission-id',
  })
  assert.equal(validateManifest(formal), formal)
  assert.throws(() => validateManifest({ ...formal, version: '1.2.3-beta.1', tag: '1.2.3-beta.1', artifact: 'aulycMail-1.2.3-beta.1-build.7.dmg' }), /stable SemVer/)
  assert.throws(() => validateManifest({ ...formal, signatureType: 'adhoc' }), /developer-id/)
  assert.throws(() => validateManifest({ ...formal, teamIdentifier: '' }), /teamIdentifier/)
  assert.throws(() => validateManifest({ ...formal, notarized: false }), /notarized=true/)
})

test('manifest files bind filename and digest and identity CLI handles local manifests', (t) => {
  const root = versionFixture(t)
  const dmgPath = path.join(root, 'local.dmg')
  fs.writeFileSync(dmgPath, 'synthetic artifact')
  const local = manifest({
    version: '1.2.3-dev',
    buildNumber: 0,
    releaseChannel: 'local',
    tag: null,
    dirty: true,
    artifact: 'local.dmg',
    sha256: createHash('sha256').update('synthetic artifact').digest('hex'),
    signatureType: 'adhoc',
  })
  const manifestPath = path.join(root, 'local.manifest.json')
  writeJSON(manifestPath, local)
  assert.deepEqual(verifyManifestFile(manifestPath, { dmgPath }), local)
  captureLogs(() => runIdentity([
    'verify-manifest',
    '--manifest', manifestPath,
    '--repo', root,
    '--dmg', dmgPath,
  ]))

  const renamedDmg = path.join(root, 'renamed.dmg')
  fs.copyFileSync(dmgPath, renamedDmg)
  assert.throws(() => verifyManifestFile(manifestPath, { dmgPath: renamedDmg }), /filename/)
  fs.writeFileSync(dmgPath, 'tampered')
  assert.throws(() => verifyManifestFile(manifestPath, { dmgPath }), /SHA-256/)
  fs.writeFileSync(manifestPath, '{broken')
  assert.throws(() => verifyManifestFile(manifestPath, {}), /Cannot read release manifest/)
  captureLogs(() => runIdentity(['help']))
  assert.throws(() => runIdentity(['unknown']), /Unknown command/)
  assert.throws(() => runIdentity([]), /command is required/)
})

test('release identity rejects malformed commits, tags, tagged state, and dirty sources', (t) => {
  const fixture = taggedRepo(t)
  assert.equal(isGitWorktreeClean(fixture.root), true)
  assert.equal(verifyReleasePreflight(fixture.root).commit, fixture.commit)
  assert.equal(verifyReleaseTag(fixture.root, fixture.version).commit, fixture.commit)
  assert.doesNotThrow(() => verifyManifestGit(manifest({ commit: fixture.commit }), fixture.root))

  assert.throws(() => verifyReleaseTag(fixture.root, `v${fixture.version}`), /exactly match/)
  git(fixture.root, 'tag', 'lightweight')
  assert.throws(() => verifyReleaseTag(fixture.root, 'lightweight'), /exactly match/)

  fs.writeFileSync(path.join(fixture.root, 'dirty.txt'), 'dirty\n')
  assert.equal(isGitWorktreeClean(fixture.root), false)
  assert.throws(() => verifyReleasePreflight(fixture.root), /clean Git worktree/)
  assert.throws(() => verifyReleaseTag(fixture.root, fixture.version), /dirty worktree/)
  fs.rmSync(path.join(fixture.root, 'dirty.txt'))

  const attached = fs.mkdtempSync(path.join(os.tmpdir(), 'aulycmail-attached-source-'))
  t.after(() => fs.rmSync(attached, { recursive: true, force: true }))
  execFileSync('git', ['clone', '-q', fixture.root, attached])
  assert.throws(() => verifyTaggedSource(attached, fixture.version), /detached HEAD/)
})

test('release commit validation rejects wrong subjects and functional files', (t) => {
  const fixture = taggedRepo(t)
  assert.doesNotThrow(() => verifyReleaseCommit(fixture.root, fixture.version))
  fs.writeFileSync(path.join(fixture.root, 'functional.txt'), 'change\n')
  git(fixture.root, 'add', 'functional.txt')
  git(fixture.root, 'commit', '-qm', `chore: release ${fixture.version}`)
  assert.throws(() => verifyReleaseCommit(fixture.root, fixture.version), /non-release files/)
  assert.throws(() => verifyReleaseCommit(fixture.root, '9.9.9'), /subject must be/)
})

test('release preparation helpers reject invalid transitions and empty changelogs', () => {
  assert.throws(() => bumpBaseVersion('1.2.3', 'invalid'), /Invalid bump level/)
  assert.throws(() => nextReleaseVersion('1.2.3', 'local'), /Invalid release channel/)
  assert.equal(nextReleaseVersion('1.2.3-alpha.2', 'test'), '1.2.3-beta.1')
  assert.equal(nextReleaseVersion('1.2.3', 'test', 'major'), '2.0.0-beta.1')
  assert.throws(() => promoteUnreleased('# Changelog\n', '1.2.3', '2026-08-03', 'formal', '1.2.2'), /missing the \[Unreleased\]/)
  assert.throws(() => promoteUnreleased('# Changelog\n\n## [Unreleased]\n\n## [1.2.3]\n', '1.2.3', '2026-08-03', 'formal', '1.2.2'), /already contains/)
  assert.match(
    promoteUnreleased('# Changelog\n\n## [Unreleased]\n', '1.2.3-beta.1', '2026-08-03', 'test', '1.2.3-dev'),
    /- Test release\./,
  )
  assert.match(
    promoteUnreleased('# Changelog\n\n## [Unreleased]\n', '1.2.4', '2026-08-03', 'formal', '1.2.3'),
    /- Stable release\./,
  )
  assert.throws(() => foldUnreleasedIntoVersion('# Changelog\n', '1.2.3'), /missing the \[Unreleased\]/)
  assert.throws(() => foldUnreleasedIntoVersion('# Changelog\n\n## [Unreleased]\n', '1.2.3'), /missing 1\.2\.3/)
  assert.equal(
    foldUnreleasedIntoVersion('# Changelog\n\n## [Unreleased]\n\n## [1.2.3]\n\n- Existing.\n', '1.2.3'),
    '# Changelog\n\n## [Unreleased]\n\n## [1.2.3]\n\n- Existing.\n',
  )
})

test('release preparation validates clean state, channels, bumps, and prepared retries', (t) => {
  const fixture = taggedRepo(t)
  assert.throws(() => prepareRelease({ root: fixture.root, channel: 'local' }), /channel must be/)
  assert.throws(() => prepareRelease({ root: fixture.root, channel: 'test', bump: 'invalid' }), /bump must be/)

  const prepared = prepareRelease({ root: fixture.root, channel: 'test', date: '2026-08-03' })
  assert.deepEqual(prepared, { version: '1.2.3-beta.1', build: 7, alreadyPrepared: true })
  captureLogs(() => runPrepare(['test', '--date', '2026-08-03'], fixture.root))
  captureLogs(() => runPrepare(['help'], fixture.root))
  assert.throws(() => runPrepare(['test', '--date'], fixture.root), /requires a value/)

  fs.writeFileSync(path.join(fixture.root, 'dirty.txt'), 'dirty\n')
  assert.throws(() => prepareRelease({ root: fixture.root, channel: 'test' }), /not clean/)
})

test('release worktree validates options and links reusable frontend dependencies', (t) => {
  const fixture = taggedRepo(t)
  const modules = path.join(fixture.root, 'frontend', 'node_modules')
  fs.mkdirSync(modules)
  fs.writeFileSync(path.join(modules, 'marker'), 'fixture')
  let isolated = ''
  const result = withDetachedTagWorktree(
    { repoRoot: fixture.root, tag: fixture.version, linkNodeModules: true },
    (worktree, identity) => {
      isolated = worktree
      assert.equal(fs.lstatSync(path.join(worktree, 'frontend', 'node_modules')).isSymbolicLink(), true)
      return identity.version
    },
  )
  assert.equal(result, fixture.version)
  assert.equal(fs.existsSync(isolated), false)

  const outputDir = path.join(fixture.root, 'output')
  assert.throws(() => buildReleaseFromTag({ repoRoot: fixture.root, tag: fixture.version, channel: 'nightly', outputDir }), /channel must be/)
  assert.throws(() => buildReleaseFromTag({ repoRoot: fixture.root, tag: fixture.version, channel: 'test', outputDir, signIdentity: 'Developer ID' }), /must not receive/)
  assert.throws(() => buildReleaseFromTag({ repoRoot: fixture.root, tag: fixture.version, channel: 'formal', outputDir }), /require both/)
  captureLogs(() => runWorktree(['help']))
  assert.throws(() => runWorktree([]), /command is required/)
  assert.throws(() => runWorktree(['unknown']), /Unknown command/)
  assert.throws(() => runWorktree(['build', '--tag']), /requires a value/)
  assert.throws(() => runWorktree(['build', '--channel', 'test', '--output-dir', outputDir]), /--tag is required/)
})

test('backup tooling rejects unsafe names, invalid branches, missing tags, and bad commands', (t) => {
  const fixture = taggedRepo(t, { version: '1.2.3', build: 7 })
  const remote = fs.mkdtempSync(path.join(os.tmpdir(), 'aulycmail-backup-edge-'))
  t.after(() => fs.rmSync(remote, { recursive: true, force: true }))
  execFileSync('git', ['init', '--bare', '-q', remote])
  git(fixture.root, 'branch', '-M', 'main')
  git(fixture.root, 'remote', 'add', 'backup', remote)

  assert.equal(verifyBackupRemote(fixture.root).branch, 'main')
  assert.throws(() => verifyBackupRemote(fixture.root, { remote: '../unsafe' }), /Invalid backup remote name/)
  assert.throws(() => verifyBackupRemote(fixture.root, { branch: 'bad branch' }), /check-ref-format failed/)
  assert.throws(() => verifyBackupRemote(fixture.root, { remote: 'missing' }), /remote failed|No such remote/)
  assert.throws(() => pushFormalReleaseBackup(fixture.root), /tag is required/)
  captureLogs(() => runBackup(['preflight', '--root', fixture.root]))
  assert.throws(() => runBackup(['preflight', '--root']), /requires a value/)
  assert.throws(() => runBackup(['unknown']), /Usage:/)
})
