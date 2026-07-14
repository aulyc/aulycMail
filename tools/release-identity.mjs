#!/usr/bin/env node

import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

import { verifyRelease } from './version-bump.mjs'

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url))
const DEFAULT_ROOT = path.resolve(SCRIPT_DIR, '..')

export const RELEASE_METADATA_FILES = new Set([
  'CHANGELOG.md',
  'frontend/package-lock.json',
  'frontend/package.json',
  'version.json',
  'wails.json',
])

export const REQUIRED_MANIFEST_FIELDS = [
  'application',
  'version',
  'buildNumber',
  'releaseChannel',
  'tag',
  'commit',
  'dirty',
  'artifact',
  'sha256',
  'architecture',
  'bundleIdentifier',
  'teamIdentifier',
  'minimumSystemVersion',
  'signatureType',
  'hardenedRuntime',
  'notarized',
  'notarizationSubmissionId',
  'builtAt',
]

function fail(message) {
  throw new Error(message)
}

export function git(root, args, { quiet = false } = {}) {
  return execFileSync('git', args, {
    cwd: root,
    encoding: 'utf8',
    stdio: quiet ? ['ignore', 'pipe', 'ignore'] : ['ignore', 'pipe', 'pipe'],
  }).trim()
}

function gitOrNull(root, args) {
  try {
    return git(root, args, { quiet: true })
  } catch {
    return null
  }
}

export function isGitWorktreeClean(root) {
  return git(root, ['status', '--porcelain', '--untracked-files=all']) === ''
}

function changedFilesAtCommit(root, commit) {
  const output = git(root, [
    'diff-tree',
    '--root',
    '--no-commit-id',
    '--name-only',
    '-r',
    commit,
  ])
  return output ? output.split('\n').filter(Boolean) : []
}

export function verifyReleaseCommit(root, version, commit = 'HEAD') {
  const subject = git(root, ['show', '-s', '--format=%s', commit])
  const expectedSubject = `chore: release ${version}`
  if (subject !== expectedSubject) {
    fail(`Release commit subject must be "${expectedSubject}", got "${subject}".`)
  }

  const unexpected = changedFilesAtCommit(root, commit)
    .filter((file) => !RELEASE_METADATA_FILES.has(file))
  if (unexpected.length > 0) {
    fail(`Release commit contains non-release files: ${unexpected.join(', ')}.`)
  }
}

function verifyAnnotatedTag(root, tag) {
  const type = gitOrNull(root, ['cat-file', '-t', `refs/tags/${tag}`])
  if (type !== 'tag') {
    fail(`Release tag ${tag} must exist as an annotated tag.`)
  }
  return git(root, ['rev-parse', `${tag}^{commit}`])
}

function readTaggedVersionState(root, tag) {
  const source = git(root, ['show', `${tag}^{commit}:version.json`])
  try {
    return JSON.parse(source)
  } catch (error) {
    fail(`Tag ${tag} contains an invalid version.json: ${error.message}`)
  }
}

export function verifyReleasePreflight(root = DEFAULT_ROOT) {
  const state = verifyRelease(root)
  if (!isGitWorktreeClean(root)) {
    fail('Release requires a clean Git worktree.')
  }
  verifyReleaseCommit(root, state.version)
  return {
    version: state.version,
    build: state.build,
    commit: git(root, ['rev-parse', 'HEAD']),
  }
}

export function verifyReleaseTag(root, tag, { requireHead = true } = {}) {
  const state = verifyRelease(root)
  if (!isGitWorktreeClean(root)) {
    fail(`Release tag ${tag} cannot be verified from a dirty worktree.`)
  }
  if (tag !== state.version || tag.startsWith('v')) {
    fail(`Release tag must exactly match version ${state.version} without a v prefix.`)
  }

  const tagCommit = verifyAnnotatedTag(root, tag)
  const taggedState = readTaggedVersionState(root, tag)
  if (taggedState.version !== tag || taggedState.build !== state.build) {
    fail(`Tag ${tag} version.json does not match version/build ${state.version}/${state.build}.`)
  }
  verifyReleaseCommit(root, tag, tagCommit)

  if (requireHead) {
    const head = git(root, ['rev-parse', 'HEAD'])
    if (head !== tagCommit) {
      fail(`Release tag ${tag} points to ${tagCommit}, but HEAD is ${head}.`)
    }
  }

  return { version: tag, build: state.build, commit: tagCommit }
}

export function verifyTaggedSource(root, tag) {
  if (!isGitWorktreeClean(root)) {
    fail(`Isolated release source for ${tag} is not clean.`)
  }
  const identity = verifyReleaseTag(root, tag, { requireHead: true })
  if (gitOrNull(root, ['symbolic-ref', '-q', 'HEAD']) !== null) {
    fail(`Isolated release source for ${tag} must use a detached HEAD.`)
  }
  return identity
}

function requireString(value, field) {
  if (typeof value !== 'string' || value.length === 0) {
    fail(`Manifest field ${field} must be a non-empty string.`)
  }
}

function requireBoolean(value, field) {
  if (typeof value !== 'boolean') {
    fail(`Manifest field ${field} must be a boolean.`)
  }
}

export function validateManifest(manifest) {
  if (!manifest || typeof manifest !== 'object' || Array.isArray(manifest)) {
    fail('Release manifest must be a JSON object.')
  }
  const missing = REQUIRED_MANIFEST_FIELDS.filter((field) => !Object.hasOwn(manifest, field))
  if (missing.length > 0) {
    fail(`Release manifest is missing fields: ${missing.join(', ')}.`)
  }

  requireString(manifest.application, 'application')
  requireString(manifest.version, 'version')
  requireString(manifest.commit, 'commit')
  requireString(manifest.artifact, 'artifact')
  requireString(manifest.sha256, 'sha256')
  requireString(manifest.architecture, 'architecture')
  requireString(manifest.bundleIdentifier, 'bundleIdentifier')
  requireString(manifest.minimumSystemVersion, 'minimumSystemVersion')
  requireString(manifest.signatureType, 'signatureType')
  requireString(manifest.builtAt, 'builtAt')
  requireBoolean(manifest.dirty, 'dirty')
  requireBoolean(manifest.hardenedRuntime, 'hardenedRuntime')
  requireBoolean(manifest.notarized, 'notarized')

  if (manifest.application !== 'aulycmail') fail('Manifest application must be aulycmail.')
  if (!Number.isSafeInteger(manifest.buildNumber) || manifest.buildNumber < 0) {
    fail('Manifest buildNumber must be a non-negative integer.')
  }
  if (!/^[0-9a-f]{7,64}$/.test(manifest.commit)) fail('Manifest commit is not a hexadecimal Git object ID.')
  if (!/^[0-9a-f]{64}$/.test(manifest.sha256)) fail('Manifest sha256 must contain 64 lowercase hexadecimal characters.')
  if (manifest.architecture !== 'arm64') fail('Release artifacts must be Apple Silicon arm64 only.')
  if (manifest.bundleIdentifier !== 'com.aulyc.aulycmail') fail('Unexpected bundleIdentifier in release manifest.')
  if (!/^\d+\.\d+(?:\.\d+)?$/.test(manifest.minimumSystemVersion)) fail('Invalid minimumSystemVersion.')
  if (Number.isNaN(Date.parse(manifest.builtAt))) fail('Manifest builtAt must be an ISO-8601 timestamp.')

  if (!['local', 'test', 'formal'].includes(manifest.releaseChannel)) {
    fail('Manifest releaseChannel must be local, test, or formal.')
  }

  if (manifest.releaseChannel === 'local') {
    if (manifest.tag !== null) fail('Local artifacts must not claim a release tag.')
    if (!['unsigned', 'adhoc'].includes(manifest.signatureType)) fail('Local artifacts must be unsigned or ad-hoc signed.')
    if (manifest.notarized || manifest.notarizationSubmissionId !== null) fail('Local artifacts must not claim notarization.')
    return manifest
  }

  if (manifest.tag !== manifest.version || manifest.tag.startsWith('v')) {
    fail('Release manifest tag must exactly match version without a v prefix.')
  }
  if (manifest.buildNumber < 1) fail('Test and formal releases require a positive buildNumber.')
  if (manifest.dirty) fail('Test and formal release manifests must record dirty=false.')
  const expectedArtifact = `aulycmail-${manifest.version}-build.${manifest.buildNumber}.dmg`
  if (manifest.artifact !== expectedArtifact) {
    fail(`Manifest artifact must be ${expectedArtifact}.`)
  }

  if (manifest.releaseChannel === 'test') {
    if (!/-((?:alpha|beta|rc)\.[1-9]\d*)$/.test(manifest.version)) {
      fail('Test releases require an alpha.N, beta.N, or rc.N version.')
    }
    if (manifest.signatureType !== 'adhoc') fail('Test releases require signatureType=adhoc.')
    if (manifest.teamIdentifier !== null) fail('Ad-hoc test releases must not claim a Team ID.')
    if (manifest.hardenedRuntime) fail('Ad-hoc test releases must record hardenedRuntime=false.')
    if (manifest.notarized || manifest.notarizationSubmissionId !== null) {
      fail('Test releases must not claim Apple notarization.')
    }
  } else {
    if (!/^\d+\.\d+\.\d+$/.test(manifest.version)) fail('Formal releases require a stable SemVer version.')
    if (manifest.signatureType !== 'developer-id') fail('Formal releases require signatureType=developer-id.')
    requireString(manifest.teamIdentifier, 'teamIdentifier')
    if (!manifest.hardenedRuntime) fail('Formal releases require hardenedRuntime=true.')
    if (!manifest.notarized) fail('Formal releases require notarized=true.')
    requireString(manifest.notarizationSubmissionId, 'notarizationSubmissionId')
  }

  return manifest
}

export function verifyManifestGit(manifest, root) {
  validateManifest(manifest)
  if (manifest.releaseChannel === 'local') return

  const tagCommit = verifyAnnotatedTag(root, manifest.tag)
  if (tagCommit !== manifest.commit) {
    fail(`Manifest commit ${manifest.commit} does not match tag ${manifest.tag} at ${tagCommit}.`)
  }
  const taggedState = readTaggedVersionState(root, manifest.tag)
  if (taggedState.version !== manifest.version || taggedState.build !== manifest.buildNumber) {
    fail('Manifest version/build does not match version.json at the release tag.')
  }
  verifyReleaseCommit(root, manifest.version, tagCommit)
}

function sha256File(file) {
  const hash = createHash('sha256')
  hash.update(fs.readFileSync(file))
  return hash.digest('hex')
}

export function verifyManifestFile(manifestPath, { root, dmgPath }) {
  let manifest
  try {
    manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
  } catch (error) {
    fail(`Cannot read release manifest ${manifestPath}: ${error.message}`)
  }
  validateManifest(manifest)
  if (root) verifyManifestGit(manifest, root)
  if (dmgPath) {
    if (path.basename(dmgPath) !== manifest.artifact) fail('DMG filename does not match manifest artifact.')
    if (sha256File(dmgPath) !== manifest.sha256) fail('DMG SHA-256 does not match the release manifest.')
  }
  return manifest
}

function optionValue(args, name, fallback = undefined) {
  const index = args.indexOf(name)
  if (index < 0) return fallback
  if (!args[index + 1]) fail(`${name} requires a value.`)
  return args[index + 1]
}

function printUsage() {
  console.log(`Usage: node tools/release-identity.mjs <command> [options]

Commands:
  verify-preflight [--root PATH]
  verify-tag --tag VERSION [--root PATH]
  verify-source --tag VERSION [--root PATH]
  verify-manifest --manifest PATH --repo PATH --dmg PATH`)
}

export function run(argv) {
  const [command, ...args] = argv
  const root = path.resolve(optionValue(args, '--root', DEFAULT_ROOT))
  switch (command) {
    case 'verify-preflight': {
      const identity = verifyReleasePreflight(root)
      console.log(`Release preflight verified for ${identity.version} at ${identity.commit}.`)
      return
    }
    case 'verify-tag': {
      const identity = verifyReleaseTag(root, optionValue(args, '--tag'))
      console.log(`Annotated tag ${identity.version} verified at ${identity.commit}.`)
      return
    }
    case 'verify-source': {
      const identity = verifyTaggedSource(root, optionValue(args, '--tag'))
      console.log(`Detached clean release source ${identity.version} verified at ${identity.commit}.`)
      return
    }
    case 'verify-manifest': {
      const manifestPath = path.resolve(optionValue(args, '--manifest'))
      const repo = path.resolve(optionValue(args, '--repo'))
      const dmgPath = path.resolve(optionValue(args, '--dmg'))
      const manifest = verifyManifestFile(manifestPath, { root: repo, dmgPath })
      console.log(`Release manifest verified for ${manifest.version} (${manifest.releaseChannel}).`)
      return
    }
    case '-h':
    case '--help':
    case 'help':
      printUsage()
      return
    default:
      printUsage()
      fail(command ? `Unknown command "${command}".` : 'A command is required.')
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  try {
    run(process.argv.slice(2))
  } catch (error) {
    console.error(`release-identity: ${error.message}`)
    process.exitCode = 1
  }
}
