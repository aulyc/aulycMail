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
  assert.match(appBackground, /SetAccountSyncFinishedCallback\([\s\S]*EventsEmit\(a\.ctx, "sync:accountFinished"/)
  assert.match(accountStore, /EventsOn\('sync:accountFinished'[\s\S]*delete this\.syncProgress\[data\.accountId\][\s\S]*acc\.syncing = false/)
})
