import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import { shouldBlockContactList } from '../src/lib/contacts/utils/contactLoadLifecycle.ts'

const contactListPath = new URL('../src/lib/contacts/components/ContactList.svelte', import.meta.url)
const contactsViewPath = new URL('../src/lib/contacts/stores/contactsView.svelte.ts', import.meta.url)

test('background refresh only blocks an empty contact list', () => {
  assert.equal(shouldBlockContactList(true, 0), true)
  assert.equal(shouldBlockContactList(true, 58), false)
  assert.equal(shouldBlockContactList(false, 0), false)
})

test('contact list loading ends before automatic detail loading starts', async () => {
  const [contactList, contactsView] = await Promise.all([
    readFile(contactListPath, 'utf8'),
    readFile(contactsViewPath, 'utf8'),
  ])

  assert.match(contactList, /items=\{contactsView\.contacts\}/)
  assert.match(contactList, /loading=\{shouldBlockContactList\(contactsView\.loading, contactsView\.contacts\.length\)\}/)
  assert.match(contactsView, /await BrowseContacts\(searchQuery, selectedSourceId, sortOrder, limit, offset\)/)
  assert.doesNotMatch(contactsView, /withContactRequestTimeout|ContactRequestTimeoutError/)
  assert.match(contactsView, /finally \{[\s\S]*loading = false[\s\S]*\}[\s\S]*if \(nextDetailId/)
  assert.doesNotMatch(contactsView, /await focusContact\(contacts\[index\]\.id\)/)
})

test('contact sorting is applied before pagination and default selection', async () => {
  const [contactList, contactsView] = await Promise.all([
    readFile(contactListPath, 'utf8'),
    readFile(contactsViewPath, 'utf8'),
  ])

  assert.match(contactList, /setSortOrder\(contactsView\.sortOrder === 'name-asc' \? 'name-desc' : 'name-asc'\)[\s\S]*reloadContacts\(\)/)
  assert.doesNotMatch(contactList, /items\.sort\(/)
  assert.match(contactsView, /let sortOrder = \$state<ContactSortOrder>\('name-asc'\)/)
  assert.match(contactsView, /nextDetailId = contacts\[index\]\.id/)
})

test('contact detail has an independent loading and retry state', async () => {
  const contactsView = await readFile(contactsViewPath, 'utf8')

  assert.match(contactsView, /detailLoading = true/)
  assert.match(contactsView, /detailLoadError = true/)
  assert.match(contactsView, /finally \{[\s\S]*detailLoading = false/)
})
