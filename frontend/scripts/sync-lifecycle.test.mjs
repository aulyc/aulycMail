import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const accountStorePath = new URL('../src/lib/stores/accounts.svelte.ts', import.meta.url)
const appBackgroundPath = new URL('../../app/background.go', import.meta.url)
const schedulerPath = new URL('../../internal/sync/scheduler.go', import.meta.url)

test('an unchanged scheduled folder probe still clears account sync state', async () => {
  const [accountStore, appBackground, scheduler] = await Promise.all([
    readFile(accountStorePath, 'utf8'),
    readFile(appBackgroundPath, 'utf8'),
    readFile(schedulerPath, 'utf8'),
  ])

  assert.match(scheduler, /runAccountSyncLifecycle\(acc\.ID,[\s\S]*syncAccountInboxWork\(ctx, acc, trigger\)/)
  assert.match(appBackground, /SetAccountSyncFinishedCallback\([\s\S]*EventsEmit\(a\.ctx, "sync:accountFinished"[\s\S]*"succeeded": succeeded/)
  assert.match(accountStore, /EventsOn\('sync:accountFinished'[\s\S]*delete this\.syncProgress\[data\.accountId\][\s\S]*acc\.syncing = false/)
})

test('only a successful account-level sync advances the complete sync timestamp', async () => {
  const accountStore = await readFile(accountStorePath, 'utf8')

  const folderSyncedBlock = accountStore.match(
    /EventsOn\('folder:synced'[\s\S]*?\n {4}\}\)\n\n {4}\/\/ A scheduled remote probe/,
  )?.[0]
  assert.ok(folderSyncedBlock, 'folder:synced handler should be present')
  assert.doesNotMatch(folderSyncedBlock, /lastCompleteSync\s*=/)

  assert.match(
    accountStore,
    /EventsOn\('sync:accountFinished', \(data: \{ accountId: string; succeeded: boolean \}\)[\s\S]*if \(data\.succeeded\)[\s\S]*acc\.lastCompleteSync = new Date\(\)/,
  )
  assert.match(
    accountStore,
    /get lastCompleteSyncTime\(\): Date \| null[\s\S]*enabledAccounts\.some\(\(a\) => a\.lastCompleteSync === null\)[\s\S]*Math\.min/,
  )
})
