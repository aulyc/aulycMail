#!/usr/bin/env node

import { execFileSync, spawnSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

import {
  isGitWorktreeClean,
  verifyManifestFile,
  verifyReleaseTag,
  verifyTaggedSource,
} from './release-identity.mjs'

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url))
const DEFAULT_ROOT = path.resolve(SCRIPT_DIR, '..')

function fail(message) {
  throw new Error(message)
}

function git(root, args, options = {}) {
  return execFileSync('git', args, {
    cwd: root,
    encoding: 'utf8',
    stdio: options.quiet ? ['ignore', 'pipe', 'ignore'] : ['ignore', 'pipe', 'pipe'],
  }).trim()
}

function cleanupWorktree(repoRoot, container, worktree) {
  let cleanupError = null
  try {
    if (fs.existsSync(worktree)) {
      execFileSync('git', ['worktree', 'remove', '--force', worktree], {
        cwd: repoRoot,
        stdio: 'ignore',
      })
    }
  } catch (error) {
    cleanupError = error
    fs.rmSync(worktree, { recursive: true, force: true })
    try {
      execFileSync('git', ['worktree', 'prune'], { cwd: repoRoot, stdio: 'ignore' })
    } catch {
      // Preserve the first cleanup failure for the caller.
    }
  } finally {
    fs.rmSync(container, { recursive: true, force: true })
  }
  if (cleanupError) fail(`Failed to clean temporary release worktree: ${cleanupError.message}`)
}

export function withDetachedTagWorktree({ repoRoot, tag, linkNodeModules = false }, callback) {
  const root = path.resolve(repoRoot)
  if (!isGitWorktreeClean(root)) fail('Caller worktree must be clean before an isolated release build.')
  const identity = verifyReleaseTag(root, tag, { requireHead: true })

  const container = fs.mkdtempSync(path.join(os.tmpdir(), 'aulycmail-release-worktree-'))
  const worktree = path.join(container, 'source')
  let primaryError = null
  let result

  try {
    git(root, ['worktree', 'add', '--detach', worktree, tag])

    if (linkNodeModules) {
      const sourceModules = path.join(root, 'frontend', 'node_modules')
      const targetModules = path.join(worktree, 'frontend', 'node_modules')
      if (fs.existsSync(sourceModules) && !fs.existsSync(targetModules)) {
        fs.symlinkSync(sourceModules, targetModules, 'dir')
      }
    }

    verifyTaggedSource(worktree, tag)
    result = callback(worktree, identity)
    verifyTaggedSource(worktree, tag)
  } catch (error) {
    primaryError = error
  }

  try {
    cleanupWorktree(root, container, worktree)
  } catch (error) {
    if (!primaryError) primaryError = error
  }

  if (!isGitWorktreeClean(root)) {
    primaryError ??= new Error('Caller worktree changed during the isolated release build.')
  }
  if (primaryError) throw primaryError
  return result
}

export function buildReleaseFromTag({
  repoRoot = DEFAULT_ROOT,
  tag,
  channel,
  outputDir,
  signIdentity = '',
  notaryProfile = '',
}) {
  if (!['test', 'formal'].includes(channel)) fail('Release channel must be test or formal.')
  if (channel === 'test' && (signIdentity || notaryProfile)) {
    fail('Test releases must not receive Developer ID or notarization credentials.')
  }
  if (channel === 'formal' && (!signIdentity || !notaryProfile)) {
    fail('Formal releases require both SIGN_IDENTITY and NOTARY_PROFILE.')
  }

  const root = path.resolve(repoRoot)
  const destination = path.resolve(outputDir)
  const identity = verifyReleaseTag(root, tag, { requireHead: true })
  const artifact = `aulycmail-${identity.version}-build.${identity.build}.dmg`
  const dmgPath = path.join(destination, artifact)
  const manifestPath = path.join(destination, artifact.replace(/\.dmg$/, '.manifest.json'))
  if (fs.existsSync(dmgPath) || fs.existsSync(manifestPath)) {
    fail(`Refusing to overwrite existing release artifact or manifest for ${identity.version}.`)
  }
  fs.mkdirSync(destination, { recursive: true })

  withDetachedTagWorktree({ repoRoot: root, tag }, (worktree) => {
    const makeArgs = [
      '--no-print-directory',
      'isolated-release-artifact',
      `RELEASE_CHANNEL=${channel}`,
      `RELEASE_TAG=${tag}`,
      `RELEASE_OUTPUT_DIR=${destination}`,
    ]
    if (signIdentity) makeArgs.push(`SIGN_IDENTITY=${signIdentity}`)
    if (notaryProfile) makeArgs.push(`NOTARY_PROFILE=${notaryProfile}`)

    const result = spawnSync('make', makeArgs, {
      cwd: worktree,
      env: process.env,
      stdio: 'inherit',
    })
    if (result.error) fail(`Failed to start isolated release build: ${result.error.message}`)
    if (result.status !== 0) fail(`Isolated release build failed with status ${result.status}.`)
  })

  if (!fs.existsSync(dmgPath) || !fs.existsSync(manifestPath)) {
    fail('Isolated release build did not produce the expected DMG and manifest.')
  }
  verifyManifestFile(manifestPath, { root, dmgPath })
  return { dmgPath, manifestPath, ...identity }
}

function optionValue(args, name, fallback = undefined) {
  const index = args.indexOf(name)
  if (index < 0) return fallback
  if (!args[index + 1]) fail(`${name} requires a value.`)
  return args[index + 1]
}

function requiredOption(args, name) {
  const value = optionValue(args, name)
  if (!value) fail(`${name} is required.`)
  return value
}

function printUsage() {
  console.log(`Usage: node tools/release-worktree.mjs build [options]

Options:
  --repo PATH
  --tag VERSION
  --channel test|formal
  --output-dir PATH
  --sign-identity IDENTITY
  --notary-profile PROFILE`)
}

export function run(argv) {
  const [command, ...args] = argv
  if (command === '-h' || command === '--help' || command === 'help') {
    printUsage()
    return
  }
  if (command !== 'build') {
    printUsage()
    fail(command ? `Unknown command "${command}".` : 'A command is required.')
  }
  const result = buildReleaseFromTag({
    repoRoot: optionValue(args, '--repo', DEFAULT_ROOT),
    tag: requiredOption(args, '--tag'),
    channel: requiredOption(args, '--channel'),
    outputDir: requiredOption(args, '--output-dir'),
    signIdentity: optionValue(args, '--sign-identity', ''),
    notaryProfile: optionValue(args, '--notary-profile', ''),
  })
  console.log(`Built isolated release artifact ${result.dmgPath}`)
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  try {
    run(process.argv.slice(2))
  } catch (error) {
    console.error(`release-worktree: ${error.message}`)
    process.exitCode = 1
  }
}
