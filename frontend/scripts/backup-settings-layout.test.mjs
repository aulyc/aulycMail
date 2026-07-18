import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const paths = {
  backupPage: new URL('../src/lib/components/settings/backup/BackupSettingsPage.svelte', import.meta.url),
  recentBackup: new URL('../src/lib/components/settings/backup/RecentBackupLog.svelte', import.meta.url),
  activityItem: new URL('../src/lib/components/settings/activity/ActivityLogItem.svelte', import.meta.url),
  activityList: new URL('../src/lib/components/settings/activity/ActivityLogList.svelte', import.meta.url),
  settingsDialog: new URL('../src/lib/components/settings/SettingsDialog.svelte', import.meta.url),
}

test('backup action lives in the page header and latest backup stays compact', async () => {
  const [backupPage, recentBackup] = await Promise.all([
    readFile(paths.backupPage, 'utf8'),
    readFile(paths.recentBackup, 'utf8'),
  ])

  assert.match(backupPage, /<SettingsPageHeader[\s\S]*\{#snippet action\(\)\}[\s\S]*<BackupRunPanel/)
  assert.doesNotMatch(recentBackup, /activitySummary|\.split\(' · '\)/)
  assert.match(recentBackup, /settingsBackup\.viewLogs/)
  assert.match(recentBackup, /settingsBackup\.recentPartial/)
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
  assert.match(activityList, /let expandedId = \$state<string \| null>\(null\)/)
  assert.match(activityList, /expanded=\{expandedId === log\.id\}/)
  assert.match(settingsDialog, /activityInitialType = 'backup'/)
})
