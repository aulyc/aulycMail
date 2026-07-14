#!/usr/bin/env node

import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

import { loadVersionState, parseVersion, setVersion } from './version-bump.mjs'

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url))
const DEFAULT_ROOT = path.resolve(SCRIPT_DIR, '..')
const BUMP_LEVELS = new Set(['auto', 'patch', 'minor', 'major'])

function fail(message) {
  throw new Error(message)
}

export function bumpBaseVersion(base, level) {
  if (!['patch', 'minor', 'major'].includes(level)) fail(`Invalid bump level "${level}".`)
  const [major, minor, patch] = base.split('.').map(Number)
  if (level === 'major') return `${major + 1}.0.0`
  if (level === 'minor') return `${major}.${minor + 1}.0`
  return `${major}.${minor}.${patch + 1}`
}

export function inferBumpLevel(messages) {
  const text = Array.isArray(messages) ? messages.join('\n\n') : String(messages || '')
  if (/BREAKING CHANGE\s*:/i.test(text) || /^[a-z][a-z0-9-]*(?:\([^\n)]+\))?!:/im.test(text)) {
    return 'major'
  }
  if (/^feat(?:\([^\n)]+\))?:/im.test(text)) return 'minor'
  return 'patch'
}

export function nextReleaseVersion(currentVersion, channel, bumpLevel = 'patch') {
  if (!['test', 'formal'].includes(channel)) fail(`Invalid release channel "${channel}".`)
  const state = parseVersion(currentVersion)
  const prerelease = state.prerelease

  if (channel === 'formal') {
    return prerelease ? state.base : bumpBaseVersion(state.base, bumpLevel)
  }

  if (prerelease === 'dev') return `${state.base}-beta.1`
  if (prerelease?.startsWith('alpha.')) return `${state.base}-beta.1`
  if (prerelease?.startsWith('beta.')) {
    return `${state.base}-beta.${Number(prerelease.slice('beta.'.length)) + 1}`
  }
  if (prerelease?.startsWith('rc.')) {
    return `${state.base}-rc.${Number(prerelease.slice('rc.'.length)) + 1}`
  }
  return `${bumpBaseVersion(state.base, bumpLevel)}-beta.1`
}

export function promoteUnreleased(changelog, targetVersion, date, channel, previousVersion) {
  const heading = '## [Unreleased]'
  const headingIndex = changelog.indexOf(heading)
  if (headingIndex < 0) fail('CHANGELOG.md is missing the [Unreleased] heading.')
  if (new RegExp(`^## \\[${targetVersion.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\](?:\\s|$)`, 'm').test(changelog)) {
    fail(`CHANGELOG.md already contains ${targetVersion}.`)
  }

  const contentStart = headingIndex + heading.length
  const nextHeadingMatch = changelog.slice(contentStart).match(/\n## \[/)
  const nextHeadingIndex = nextHeadingMatch
    ? contentStart + nextHeadingMatch.index + 1
    : changelog.length
  let releaseNotes = changelog.slice(contentStart, nextHeadingIndex).trim()
  if (!releaseNotes) {
    releaseNotes = channel === 'test'
      ? '- Test release.'
      : parseVersion(previousVersion).prerelease
        ? `- Promoted ${previousVersion} to a stable release.`
        : '- Stable release.'
  }

  const before = changelog.slice(0, headingIndex)
  const after = changelog.slice(nextHeadingIndex).replace(/^\s+/, '')
  return `${before}${heading}\n\n## [${targetVersion}] — ${date}\n\n${releaseNotes}\n\n${after}`
}

function hasVersionHeading(changelog, version) {
  const escaped = version.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return new RegExp(`^## \\[${escaped}\\](?:\\s|$)`, 'm').test(changelog)
}

export function foldUnreleasedIntoVersion(changelog, version) {
  const unreleasedHeading = '## [Unreleased]'
  const unreleasedIndex = changelog.indexOf(unreleasedHeading)
  if (unreleasedIndex < 0) fail('CHANGELOG.md is missing the [Unreleased] heading.')
  if (!hasVersionHeading(changelog, version)) fail(`CHANGELOG.md is missing ${version}.`)

  const notesStart = unreleasedIndex + unreleasedHeading.length
  const nextHeadingMatch = changelog.slice(notesStart).match(/\n## \[/)
  if (!nextHeadingMatch) fail(`CHANGELOG.md is missing a release heading after [Unreleased].`)
  const nextHeadingIndex = notesStart + nextHeadingMatch.index + 1
  const notes = changelog.slice(notesStart, nextHeadingIndex).trim()
  if (!notes) return changelog

  const before = changelog.slice(0, unreleasedIndex)
  const afterUnreleased = changelog.slice(nextHeadingIndex).replace(/^\s+/, '')
  const cleared = `${before}${unreleasedHeading}\n\n${afterUnreleased}`
  const targetHeading = `## [${version}]`
  const targetIndex = cleared.indexOf(targetHeading)
  const targetLineEnd = cleared.indexOf('\n', targetIndex)
  const targetLine = targetLineEnd < 0 ? cleared.slice(targetIndex) : cleared.slice(targetIndex, targetLineEnd)
  const existingNotes = targetLineEnd < 0 ? '' : cleared.slice(targetLineEnd).replace(/^\s+/, '')
  return `${cleared.slice(0, targetIndex)}${targetLine}\n\n${notes}${existingNotes ? `\n\n${existingNotes}` : '\n'}`
}

function git(root, args, options = {}) {
  return execFileSync('git', args, {
    cwd: root,
    encoding: 'utf8',
    stdio: options.quiet ? ['ignore', 'pipe', 'ignore'] : ['ignore', 'pipe', 'pipe'],
  }).trim()
}

function tagType(root, version) {
  try {
    return git(root, ['cat-file', '-t', version], { quiet: true })
  } catch {
    return null
  }
}

function requireReleaseTag(root, version) {
  if (tagType(root, version) !== 'tag') {
    fail(`Version ${version} must have an annotated Git tag before advancing from it.`)
  }
  try {
    execFileSync('git', ['merge-base', '--is-ancestor', `${version}^{commit}`, 'HEAD'], {
      cwd: root,
      stdio: 'ignore',
    })
  } catch {
    fail(`Tag ${version} is not an ancestor of HEAD.`)
  }
}

function localDate() {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: process.env.TZ || 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(new Date())
}

function optionValue(args, name, fallback) {
  const index = args.indexOf(name)
  if (index < 0) return fallback
  if (!args[index + 1]) fail(`${name} requires a value.`)
  return args[index + 1]
}

export function prepareRelease({ root = DEFAULT_ROOT, channel, bump = 'auto', date = localDate() }) {
  if (!['test', 'formal'].includes(channel)) fail('Release channel must be test or formal.')
  if (!BUMP_LEVELS.has(bump)) fail('Release bump must be auto, patch, minor, or major.')

  const status = git(root, ['status', '--porcelain', '--untracked-files=all'])
  if (status) fail('Commit all functional changes before preparing a release; the Git worktree is not clean.')

  const current = loadVersionState(root)
  const headSubject = git(root, ['log', '-1', '--format=%s'])
  const changelogPath = path.join(root, 'CHANGELOG.md')
  const currentChangelog = fs.readFileSync(changelogPath, 'utf8')
  const channelMatchesCurrent = channel === 'formal'
    ? current.prerelease === null
    : /^(?:beta|rc)\.\d+$/.test(current.prerelease || '')
  const alreadyPrepared = headSubject === `chore: release ${current.version}`
    && channelMatchesCurrent
    && hasVersionHeading(currentChangelog, current.version)
  if (alreadyPrepared) {
    return { version: current.version, build: current.build, alreadyPrepared: true }
  }

  // A failed quality gate may leave an untagged release version followed by
  // fix commits. Reuse that unpublished version/build, fold any new notes into
  // its heading, and let Make create a fresh release commit instead of burning
  // another public identity.
  if (channelMatchesCurrent && tagType(root, current.version) === null && hasVersionHeading(currentChangelog, current.version)) {
    const updatedChangelog = foldUnreleasedIntoVersion(currentChangelog, current.version)
    if (updatedChangelog !== currentChangelog) fs.writeFileSync(changelogPath, updatedChangelog)
    return { version: current.version, build: current.build, alreadyPrepared: false }
  }

  if (current.prerelease && current.prerelease !== 'dev') {
    requireReleaseTag(root, current.version)
    if (channel === 'test') {
      const commits = git(root, ['log', '--format=%B%x00', `${current.version}..HEAD`])
      if (!commits) fail(`No commits exist after ${current.version}; refusing to create a duplicate test release.`)
    }
  }

  let effectiveBump = bump
  if (effectiveBump === 'auto') {
    if (current.prerelease) {
      effectiveBump = 'patch'
    } else {
      requireReleaseTag(root, current.version)
      const commits = git(root, ['log', '--format=%B%x00', `${current.version}..HEAD`])
      if (!commits) fail(`No commits exist after ${current.version}; there is nothing to release.`)
      effectiveBump = inferBumpLevel(commits)
    }
  }

  const targetVersion = nextReleaseVersion(current.version, channel, effectiveBump)
  const changelog = promoteUnreleased(
    currentChangelog,
    targetVersion,
    date,
    channel,
    current.version,
  )
  setVersion(root, targetVersion, current.build + 1)
  fs.writeFileSync(changelogPath, changelog)
  return { version: targetVersion, build: current.build + 1, alreadyPrepared: false }
}

function printUsage() {
  console.log(`Usage: node tools/release-prepare.mjs test|formal [--bump auto|patch|minor|major] [--date YYYY-MM-DD]

The worktree must be clean. The command updates version metadata and promotes
CHANGELOG.md [Unreleased] content; the Make target creates the release commit.`)
}

export function run(argv, root = DEFAULT_ROOT) {
  const [channel, ...args] = argv
  if (channel === '-h' || channel === '--help' || channel === 'help') {
    printUsage()
    return
  }
  const result = prepareRelease({
    root,
    channel,
    bump: optionValue(args, '--bump', 'auto'),
    date: optionValue(args, '--date', localDate()),
  })
  if (result.alreadyPrepared) {
    console.log(`Release ${result.version} (build ${result.build}) is already prepared.`)
  } else {
    console.log(`Prepared release ${result.version} (build ${result.build}).`)
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  try {
    run(process.argv.slice(2))
  } catch (error) {
    console.error(`release-prepare: ${error.message}`)
    process.exitCode = 1
  }
}
