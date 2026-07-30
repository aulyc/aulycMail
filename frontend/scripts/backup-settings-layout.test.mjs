import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const paths = {
  backupPage: new URL('../src/lib/components/settings/backup/BackupSettingsPage.svelte', import.meta.url),
  backupRunPanel: new URL('../src/lib/components/settings/backup/BackupRunPanel.svelte', import.meta.url),
  backupConfig: new URL('../src/lib/components/settings/backup/BackupConfigSection.svelte', import.meta.url),
  backupDirectoryPicker: new URL('../src/lib/components/backup/BackupDirectoryPicker.svelte', import.meta.url),
  backupScope: new URL('../src/lib/components/settings/backup/BackupScopePicker.svelte', import.meta.url),
  recentBackup: new URL('../src/lib/components/settings/backup/RecentBackupLog.svelte', import.meta.url),
  activityItem: new URL('../src/lib/components/settings/activity/ActivityLogItem.svelte', import.meta.url),
  activityList: new URL('../src/lib/components/settings/activity/ActivityLogList.svelte', import.meta.url),
  activityFilters: new URL('../src/lib/components/settings/activity/ActivityLogFilters.svelte', import.meta.url),
  activityClearMenu: new URL('../src/lib/components/settings/activity/ActivityLogClearMenu.svelte', import.meta.url),
  activityLogPage: new URL('../src/lib/components/settings/activity/ActivityLogPage.svelte', import.meta.url),
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

test('backup start action has a visible keyboard selection ring over its primary fill', async () => {
  const [backupRunPanel, settingsDialog] = await Promise.all([
    readFile(paths.backupRunPanel, 'utf8'),
    readFile(paths.settingsDialog, 'utf8'),
  ])

  assert.match(backupRunPanel, /<Button[\s\S]*data-settings-contrast-selection/)
  assert.match(
    settingsDialog,
    /data-settings-contrast-selection.*data-settings-keyboard-selected='true'[\s\S]*outline-offset: 2px;[\s\S]*box-shadow: 0 0 0 2px hsl\(var\(--background\)\);/,
  )
})

test('settings arrow navigation visits the backup folder icon before the directory trigger', async () => {
  const [backupDirectoryPicker, settingsDialog] = await Promise.all([
    readFile(paths.backupDirectoryPicker, 'utf8'),
    readFile(paths.settingsDialog, 'utf8'),
  ])

  assert.match(backupDirectoryPicker, /data-settings-keyboard-order-group/)
  assert.match(
    backupDirectoryPicker,
    /data-settings-keyboard-order="2"[\s\S]*data-settings-keyboard-order="1"/,
  )
  assert.match(
    settingsDialog,
    /orderGroups = new Map[\s\S]*data-settings-keyboard-order-group[\s\S]*\[\.\.\.entries\]\.sort\(\(left, right\) => left\.order - right\.order\)[\s\S]*controls\[slot\] = ordered\[index\]\.control/,
  )
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
  assert.doesNotMatch(
    activityItem,
    /title=\{activitySummary\(log\)\}/,
    'a fully visible activity summary should not repeat itself in a native hover tooltip',
  )
  assert.match(activityList, /let expandedId = \$state<string \| null>\(null\)/)
  assert.match(activityList, /expanded=\{expandedId === log\.id\}/)
  assert.match(settingsDialog, /selectNavigationPage\('activity', 'backup'\)/)
})

test('opening backup logs restores settings focus and keyboard-selects the backup filter', async () => {
  const [activityFilters, settingsDialog] = await Promise.all([
    readFile(paths.activityFilters, 'utf8'),
    readFile(paths.settingsDialog, 'utf8'),
  ])

  assert.match(settingsDialog, /openBackupActivityLog\(\) \{ selectNavigationPage\('activity', 'backup'\) \}/)
  assert.match(
    settingsDialog,
    /if \(settingsRegion === 'content'\) scheduleSettingsControlSelection\(true, true\)/,
  )
  assert.match(
    settingsDialog,
    /if \(restoreFocus\) contentRegionEl\?\.focus\(\{ preventScroll: true \}\)/,
  )
  assert.match(settingsDialog, /settingsInitialSelection === 'true'/)
  assert.match(activityFilters, /data-settings-initial-selection=\{active === filter\.key/)
  assert.match(activityFilters, /data-settings-horizontal-group="activity-toolbar"/)
  assert.match(activityFilters, /data-settings-horizontal-arrows-only/)
  assert.match(activityFilters, /data-settings-horizontal-action=\{filter\.key\}/)
  assert.match(settingsDialog, /selectOutsideHorizontalGroup\(selectedHorizontalGroup/)
})

test('activity toolbar enters visible log rows before load more or close', async () => {
  const [activityFilters, activityItem, activityLogPage, activityList, settingsDialog] = await Promise.all([
    readFile(paths.activityFilters, 'utf8'),
    readFile(paths.activityItem, 'utf8'),
    readFile(paths.activityLogPage, 'utf8'),
    readFile(paths.activityList, 'utf8'),
    readFile(paths.settingsDialog, 'utf8'),
  ])

  assert.doesNotMatch(activityFilters, /data-settings-arrow-down-target=/)
  assert.doesNotMatch(activityFilters, /data-settings-arrow-down-fallback=/)
  assert.match(activityFilters, /data-settings-horizontal-action="date"/)
  assert.match(activityItem, /<button[\s\S]*disabled=\{!expandable\}/)
  assert.match(activityList, /data-settings-control-id="activity-load-more"/)
  assert.match(activityList, /await store\.loadMore\(\)[\s\S]*onLoadMoreFinished\?\.\(store\.hasMore\)/)
  assert.match(activityLogPage, /<ActivityLogList \{store\} \{onLoadMoreFinished\}/)
  assert.match(
    settingsDialog,
    /selectedHorizontalGroup\.hasAttribute\('data-settings-horizontal-arrows-only'\)[\s\S]*selectExplicitHorizontalGroupTarget[\s\S]*selectOutsideHorizontalGroup\(selectedHorizontalGroup, direction\)/,
  )
  assert.match(
    settingsDialog,
    /restoreActivityLogPositionAfterLoad\(hasMore: boolean\)[\s\S]*activity-load-more[\s\S]*settingsFooterAction === 'close'/,
  )
  assert.match(
    settingsDialog,
    /selectedItem\?\.kind === 'footer'[\s\S]*event\.key === 'ArrowUp' \? controlItems\.at\(-1\) : controlItems\[0\]/,
  )
})

test('activity log clear trigger and menu use the same fixed width', async () => {
  const [source, settingsDialog] = await Promise.all([
    readFile(paths.activityClearMenu, 'utf8'),
    readFile(paths.settingsDialog, 'utf8'),
  ])

  assert.match(
    source,
    /<DropdownMenu\.Trigger[^>]*class="[^"]*\bw-40\b[^"]*"/,
    'the clear-log trigger should use the shared fixed width',
  )
  assert.match(
    source,
    /<DropdownMenu\.Content[^>]*class="[^"]*\bw-40\b[^"]*"/,
    'the clear-log menu should use the same fixed width',
  )
  assert.match(source, /data-keyboard-menu-trigger="true"/)
  assert.match(settingsDialog, /function isSettingsMenuTrigger/)
  assert.match(
    settingsDialog,
    /isSettingsSelectTrigger\(control\) \|\| isSettingsMenuTrigger\(control\)[\s\S]*control\.dispatchEvent\([\s\S]*new KeyboardEvent\('keydown'/,
  )
  assert.match(
    settingsDialog,
    /document\.querySelectorAll<HTMLElement>[\s\S]*role="alertdialog"[\s\S]*find\(\(dialog\) => !dialog\.contains\(contentRegionEl\)\)[\s\S]*if \(nestedDialog\) return/,
    'closing the menu into a confirmation dialog must not steal focus back to Settings',
  )
})

test('pages without global settings drafts show a close action', async () => {
  const settingsDialog = await readFile(paths.settingsDialog, 'utf8')

  assert.match(settingsDialog, /const usesSettingsDraft = \$derived\(/)
  assert.match(settingsDialog, /const showDraftActions = \$derived\(usesSettingsDraft \|\| draft\.dirty\)/)
  assert.match(settingsDialog, /\{:else\}[\s\S]*\{\$_\('common\.close'\)\}/)
})
