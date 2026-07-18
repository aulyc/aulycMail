import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const paths = {
  backupPage: new URL('../src/lib/components/settings/backup/BackupSettingsPage.svelte', import.meta.url),
  backupConfig: new URL('../src/lib/components/settings/backup/BackupConfigSection.svelte', import.meta.url),
  backupScope: new URL('../src/lib/components/settings/backup/BackupScopePicker.svelte', import.meta.url),
  recentBackup: new URL('../src/lib/components/settings/backup/RecentBackupLog.svelte', import.meta.url),
  activityItem: new URL('../src/lib/components/settings/activity/ActivityLogItem.svelte', import.meta.url),
  activityList: new URL('../src/lib/components/settings/activity/ActivityLogList.svelte', import.meta.url),
  settingsDialog: new URL('../src/lib/components/settings/SettingsDialog.svelte', import.meta.url),
}

test('backup selectors share a compact right-aligned width', async () => {
  const [backupConfig, backupScope] = await Promise.all([
    readFile(paths.backupConfig, 'utf8'),
    readFile(paths.backupScope, 'utf8'),
  ])

  assert.match(backupConfig, /class="w-96 max-w-full"/)
  assert.match(backupScope, /class="relative w-96 max-w-full"/)
})

test('backup action lives in the page header and latest backup stays compact', async () => {
  const [backupPage, recentBackup] = await Promise.all([
    readFile(paths.backupPage, 'utf8'),
    readFile(paths.recentBackup, 'utf8'),
  ])

  assert.match(backupPage, /<SettingsPageHeader[\s\S]*\{#snippet action\(\)\}[\s\S]*<BackupRunPanel/)
  assert.doesNotMatch(recentBackup, /activitySummary|\.split\(' · '\)/)
  assert.match(recentBackup, /settingsBackup\.viewLogs/)
  assert.match(recentBackup, /settingsBackup\.recentPartial/)
  assert.match(recentBackup, /justify-end gap-3 text-xs/)
})

test('backup activity rows expand from payload and only one row stays open', async () => {
  const [activityItem, activityList, settingsDialog] = await Promise.all([
    readFile(paths.activityItem, 'utf8'),
    readFile(paths.activityList, 'utf8'),
    readFile(paths.settingsDialog, 'utf8'),
  ])

  assert.match(activityItem, /backupActivityDetails\(log\)/)
  assert.match(activityItem, /aria-expanded=/)
  assert.match(activityItem, /settingsBackup\.serverNotReturned/)
  assert.match(activityItem, /flex items-baseline gap-2[\s\S]*settingsBackup\.backedUpComposition/)
  assert.match(activityItem, /flex items-baseline gap-2[\s\S]*settingsBackup\.notBackedUpComposition/)
  assert.match(activityList, /let expandedId = \$state<string \| null>\(null\)/)
  assert.match(activityList, /expanded=\{expandedId === log\.id\}/)
  assert.match(settingsDialog, /activityInitialType = 'backup'/)
})

test('pages without global settings drafts show a close action', async () => {
  const settingsDialog = await readFile(paths.settingsDialog, 'utf8')

  assert.match(settingsDialog, /const usesSettingsDraft = \$derived\(/)
  assert.match(settingsDialog, /const showDraftActions = \$derived\(usesSettingsDraft \|\| draft\.dirty\)/)
  assert.match(settingsDialog, /\{:else\}[\s\S]*\{\$_\('common\.close'\)\}/)
})
