import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'

const isSupportedHost = process.platform === 'darwin' && process.arch === 'arm64'

test('verifies an app whose mounted-volume path contains spaces', { skip: !isSupportedHost }, (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aulycMail-app-verifier-test-'))
  t.after(() => fs.rmSync(root, { recursive: true, force: true }))

  const app = path.join(root, 'Mounted Volume', 'aulycMail.app')
  const executable = path.join(app, 'Contents', 'MacOS', 'aulycMail')
  const info = path.join(app, 'Contents', 'Info.plist')
  const manifest = path.join(root, 'manifest.json')
  const source = path.join(root, 'main.go')
  fs.mkdirSync(path.dirname(executable), { recursive: true })
  fs.writeFileSync(source, `package main
import "fmt"
func main() { fmt.Println("1.2.3 (build 4)") }
`)
  execFileSync('go', ['build', '-o', executable, source])
  fs.writeFileSync(info, `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleName</key><string>aulycMail</string>
<key>CFBundleDisplayName</key><string>aulycMail</string>
<key>CFBundleExecutable</key><string>aulycMail</string>
<key>CFBundleIdentifier</key><string>com.aulyc.aulycmail</string>
<key>CFBundleShortVersionString</key><string>1.2.3</string>
<key>CFBundleVersion</key><string>4</string>
<key>AULYCSemanticVersion</key><string>1.2.3</string>
<key>AULYCCommitSHA</key><string>0123456789abcdef</string>
<key>LSMinimumSystemVersion</key><string>11.0</string>
<key>CFBundleDocumentTypes</key><array><dict>
<key>CFBundleTypeName</key><string>Mail Attachment</string>
<key>CFBundleTypeRole</key><string>Viewer</string>
<key>LSHandlerRank</key><string>Alternate</string>
<key>LSItemContentTypes</key><array><string>public.data</string></array>
</dict></array>
</dict></plist>
`)
  fs.writeFileSync(manifest, `${JSON.stringify({
    application: 'aulycMail',
    version: '1.2.3',
    buildNumber: 4,
    releaseChannel: 'test',
    commit: '0123456789abcdef',
    dirty: false,
    architecture: 'arm64',
    bundleIdentifier: 'com.aulyc.aulycmail',
    minimumSystemVersion: '11.0',
    signatureType: 'adhoc',
    hardenedRuntime: false,
    teamIdentifier: '',
  }, null, 2)}\n`)

  execFileSync('codesign', ['--force', '--deep', '--sign', '-', app])
  const output = execFileSync('bash', [
    path.resolve('tools/verify_macos_app.sh'),
    '--app', app,
    '--manifest', manifest,
    '--channel', 'test',
  ], { encoding: 'utf8' })

  assert.match(output, /Verified app identity 1\.2\.3 \(build 4, adhoc, arm64\)\./)
})
