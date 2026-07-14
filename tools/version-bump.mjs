#!/usr/bin/env node

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url))
const DEFAULT_ROOT = path.resolve(SCRIPT_DIR, '..')
const VERSION_PATTERN = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-dev|-(?:alpha|beta|rc)\.([1-9]\d*))?$/

function fail(message) {
  throw new Error(message)
}

export function parseVersion(version) {
  if (typeof version !== 'string' || !VERSION_PATTERN.test(version)) {
    fail(`Invalid version "${version}". Expected MAJOR.MINOR.PATCH, -dev, or numbered -alpha.N/-beta.N/-rc.N.`)
  }
  const base = version.match(/^(\d+\.\d+\.\d+)/)?.[1]
  return {
    version,
    base,
    prerelease: version.slice(base.length + 1) || null,
    isDev: version.endsWith('-dev'),
  }
}

export function parseBuildNumber(value) {
  const build = typeof value === 'number' ? value : Number(value)
  if (!Number.isSafeInteger(build) || build < 0) {
    fail(`Invalid build number "${value}". Expected a non-negative integer.`)
  }
  return build
}

function readJSON(file) {
  try {
    return JSON.parse(fs.readFileSync(file, 'utf8'))
  } catch (error) {
    fail(`Cannot read ${file}: ${error.message}`)
  }
}

function writeJSON(file, value) {
  fs.writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`)
}

export function loadVersionState(root = DEFAULT_ROOT) {
  const file = path.join(root, 'version.json')
  const state = readJSON(file)
  const parsed = parseVersion(state.version)
  const build = parseBuildNumber(state.build)
  return { ...parsed, build }
}

export function localVersion(version, commit, dirty = false) {
  parseVersion(version)
  const normalizedCommit = String(commit || '').trim().toLowerCase()
  if (!/^[0-9a-f]{7,40}$/.test(normalizedCommit)) {
    fail(`Invalid commit "${commit}". Expected a 7-40 character hexadecimal Git SHA.`)
  }
  const value = version.endsWith('-dev')
    ? `${version}+${normalizedCommit}`
    : `${version}+local.${normalizedCommit}`
  return dirty ? `${value}.dirty` : value
}

function expectedDerivedState(root, state) {
  const wails = readJSON(path.join(root, 'wails.json'))
  const packageJSON = readJSON(path.join(root, 'frontend/package.json'))
  const packageLock = readJSON(path.join(root, 'frontend/package-lock.json'))

  wails.info ??= {}
  wails.info.productVersion = state.base
  packageJSON.version = state.version
  packageLock.version = state.version
  if (!packageLock.packages?.['']) {
    fail('frontend/package-lock.json is missing the root package entry.')
  }
  packageLock.packages[''].version = state.version
  return { wails, packageJSON, packageLock }
}

export function syncDerivedFiles(root = DEFAULT_ROOT) {
  const state = loadVersionState(root)
  const expected = expectedDerivedState(root, state)
  writeJSON(path.join(root, 'wails.json'), expected.wails)
  writeJSON(path.join(root, 'frontend/package.json'), expected.packageJSON)
  writeJSON(path.join(root, 'frontend/package-lock.json'), expected.packageLock)
  return state
}

function stableJSON(value) {
  return `${JSON.stringify(value, null, 2)}\n`
}

export function verifyDerivedFiles(root = DEFAULT_ROOT) {
  const state = loadVersionState(root)
  const expected = expectedDerivedState(root, state)
  const checks = [
    ['wails.json', expected.wails],
    ['frontend/package.json', expected.packageJSON],
    ['frontend/package-lock.json', expected.packageLock],
  ]
  const drift = checks
    .filter(([relative, value]) => fs.readFileSync(path.join(root, relative), 'utf8') !== stableJSON(value))
    .map(([relative]) => relative)
  if (drift.length > 0) {
    fail(`Version metadata drift detected in: ${drift.join(', ')}. Run "node tools/version-bump.mjs sync".`)
  }
  return state
}

export function verifyRelease(root = DEFAULT_ROOT) {
  const state = verifyDerivedFiles(root)
  if (state.isDev) {
    fail('Release versions cannot use the -dev suffix.')
  }
  if (state.build < 1) {
    fail('A public release requires a positive build number.')
  }
  const changelog = fs.readFileSync(path.join(root, 'CHANGELOG.md'), 'utf8')
  const escaped = state.version.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  if (!new RegExp(`^## \\[${escaped}\\](?:\\s|$)`, 'm').test(changelog)) {
    fail(`CHANGELOG.md is missing a heading for ${state.version}.`)
  }
  return state
}

export function setVersion(root, version, build = 0) {
  parseVersion(version)
  const state = { version, build: parseBuildNumber(build) }
  writeJSON(path.join(root, 'version.json'), state)
  return syncDerivedFiles(root)
}

function printUsage() {
  console.log(`Usage: node tools/version-bump.mjs <command> [arguments]

Commands:
  get version|base|build       Print a value from version.json
  local-version COMMIT [--dirty] Print the local runtime version
  set VERSION [--build N]      Set a version, retain the build counter by default, and sync
  next-build                   Increment the build number and sync
  sync                         Synchronize derived version files
  verify                       Verify all derived version files
  verify-release               Verify metadata required for a public release`)
}

export function run(argv, root = DEFAULT_ROOT) {
  const [command, ...args] = argv
  switch (command) {
    case 'get': {
      const state = loadVersionState(root)
      const field = args[0]
      if (!['version', 'base', 'build'].includes(field)) fail('get requires version, base, or build.')
      console.log(state[field])
      return
    }
    case 'local-version': {
      console.log(localVersion(loadVersionState(root).version, args[0], args.includes('--dirty')))
      return
    }
    case 'set': {
      if (!args[0]) fail('set requires a version.')
      const current = loadVersionState(root)
      const buildIndex = args.indexOf('--build')
      if (buildIndex >= 0 && args[buildIndex + 1] === undefined) fail('--build requires a number.')
      const build = buildIndex >= 0 ? parseBuildNumber(args[buildIndex + 1]) : current.build
      if (build < current.build) fail(`Build number cannot move backwards from ${current.build} to ${build}.`)
      const state = setVersion(root, args[0], build)
      console.log(`Set version ${state.version}, build ${state.build}.`)
      return
    }
    case 'next-build': {
      const state = loadVersionState(root)
      const next = setVersion(root, state.version, state.build + 1)
      console.log(`Build ${next.build}.`)
      return
    }
    case 'sync': {
      const state = syncDerivedFiles(root)
      console.log(`Synchronized ${state.version}, build ${state.build}.`)
      return
    }
    case 'verify': {
      const state = verifyDerivedFiles(root)
      console.log(`Verified ${state.version}, build ${state.build}.`)
      return
    }
    case 'verify-release': {
      const state = verifyRelease(root)
      console.log(`Release metadata verified for ${state.version}, build ${state.build}.`)
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
    console.error(`version-bump: ${error.message}`)
    process.exitCode = 1
  }
}
