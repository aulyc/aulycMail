#!/usr/bin/env node

import { execFileSync } from 'node:child_process'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

import { verifyReleaseTag } from './release-identity.mjs'

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url))
const DEFAULT_ROOT = path.resolve(SCRIPT_DIR, '..')
export const DEFAULT_BACKUP_REMOTE = 'backup'
export const DEFAULT_BACKUP_BRANCH = 'main'

function fail(message) {
  throw new Error(message)
}

function git(root, args, { quiet = false } = {}) {
  try {
    return execFileSync('git', args, {
      cwd: root,
      encoding: 'utf8',
      stdio: quiet ? ['ignore', 'pipe', 'ignore'] : ['ignore', 'pipe', 'pipe'],
    }).trim()
  } catch (error) {
    const stderr = typeof error.stderr === 'string' ? error.stderr.trim() : ''
    fail(stderr || `git ${args[0]} failed.`)
  }
}

function validateRemoteName(remote) {
  if (!/^[A-Za-z0-9._-]+$/.test(remote)) {
    fail(`Invalid backup remote name: ${remote}.`)
  }
}

function validateBranchName(root, branch) {
  git(root, ['check-ref-format', '--branch', branch], { quiet: true })
}

export function verifyBackupRemote(root = DEFAULT_ROOT, {
  remote = DEFAULT_BACKUP_REMOTE,
  branch = DEFAULT_BACKUP_BRANCH,
} = {}) {
  validateRemoteName(remote)
  validateBranchName(root, branch)

  const remoteUrl = git(root, ['remote', 'get-url', remote], { quiet: true })
  const currentBranch = git(root, ['symbolic-ref', '--quiet', '--short', 'HEAD'], { quiet: true })
  if (currentBranch !== branch) {
    fail(`Formal releases must run from branch ${branch}; current branch is ${currentBranch}.`)
  }

  git(root, ['ls-remote', remote])
  return { remote, remoteUrl, branch }
}

function parseRemoteRefs(output) {
  const refs = new Map()
  for (const line of output.split('\n').filter(Boolean)) {
    const [sha, ref] = line.split(/\s+/, 2)
    refs.set(ref, sha)
  }
  return refs
}

export function pushFormalReleaseBackup(root = DEFAULT_ROOT, {
  remote = DEFAULT_BACKUP_REMOTE,
  branch = DEFAULT_BACKUP_BRANCH,
  tag,
} = {}) {
  if (!tag) fail('A formal release tag is required for backup push.')
  verifyBackupRemote(root, { remote, branch })

  const identity = verifyReleaseTag(root, tag, { requireHead: true })
  const branchRef = `refs/heads/${branch}`
  const tagRef = `refs/tags/${tag}`
  const peeledTagRef = `${tagRef}^{}`

  git(root, [
    'push',
    '--atomic',
    '--porcelain',
    remote,
    `HEAD:${branchRef}`,
    `${tagRef}:${tagRef}`,
  ])

  const remoteRefs = parseRemoteRefs(git(root, [
    'ls-remote',
    remote,
    branchRef,
    tagRef,
    peeledTagRef,
  ]))
  if (remoteRefs.get(branchRef) !== identity.commit) {
    fail(`Backup branch ${remote}/${branch} does not match release commit ${identity.commit}.`)
  }
  if (!remoteRefs.has(tagRef) || remoteRefs.get(peeledTagRef) !== identity.commit) {
    fail(`Backup tag ${remote}/${tag} does not resolve to release commit ${identity.commit}.`)
  }

  return {
    remote,
    branch,
    tag,
    commit: identity.commit,
    remoteUrl: git(root, ['remote', 'get-url', remote], { quiet: true }),
  }
}

function optionValue(args, name, fallback = undefined) {
  const index = args.indexOf(name)
  if (index === -1) return fallback
  if (index === args.length - 1) fail(`${name} requires a value.`)
  return args[index + 1]
}

export function main(argv) {
  const [command, ...args] = argv
  const root = path.resolve(optionValue(args, '--root', DEFAULT_ROOT))
  const remote = optionValue(args, '--remote', DEFAULT_BACKUP_REMOTE)
  const branch = optionValue(args, '--branch', DEFAULT_BACKUP_BRANCH)

  if (command === 'preflight') {
    const result = verifyBackupRemote(root, { remote, branch })
    console.log(`Verified formal-release backup remote ${result.remote} (${result.remoteUrl}) on branch ${result.branch}.`)
    return
  }

  if (command === 'push') {
    const tag = optionValue(args, '--tag')
    const result = pushFormalReleaseBackup(root, { remote, branch, tag })
    console.log(`Verified formal release ${result.tag} at ${result.commit} on ${result.remote}/${result.branch}.`)
    return
  }

  fail('Usage: release-backup.mjs <preflight|push> [--root PATH] [--remote NAME] [--branch NAME] [--tag TAG]')
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  try {
    main(process.argv.slice(2))
  } catch (error) {
    console.error(error.message)
    process.exitCode = 1
  }
}
