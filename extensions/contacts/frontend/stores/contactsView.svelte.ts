// View-local state for the Contacts extension's browse UI. Source selection,
// search query, and selected-row id are intentionally session-local — none of
// these need to survive across app launches.

// @ts-ignore - wailsjs bindings
import {
  Contacts_BrowseContacts as BrowseContacts,
  Contacts_GetContactDetail as GetContactDetail,
  Contacts_UpdateContact as UpdateContact,
  Contacts_DeleteLocalContact as DeleteLocalContact,
  Contacts_CreateContact as CreateContact,
} from '$wailsjs/go/app/App'
// @ts-ignore - wailsjs bindings
import type { v1 } from '$wailsjs/go/models'
// Responsive (mobile) integration — match mail's pattern of firing
// showViewer/hideSidebar from the consumer's select actions. Layout-store
// calls are self-gating: showViewer is a no-op when not responsive,
// hideSidebar is a no-op when not narrow. Kit primitives (PaneLayout,
// SourceSidebar, DetailPane) handle overlay class application + back
// buttons + scrim; this store handles the "which view do we want to be
// in next" decisions on user actions.
import { isResponsive, showViewer, hideSidebar } from '$lib/stores/layout.svelte'

// Source ID values the sidebar can dispatch:
//   ""                  → all local contacts
//   "local"             → all local contacts (manual + collected)
//   "local:manual"      → user-added local contacts (Add Contact UI)
//   "local:collected"   → auto-collected local contacts (sent-mail recipients)
let selectedSourceId = $state<string>('')
let searchQuery = $state<string>('')
let selectedContactId = $state<string | null>(null)
let contacts = $state<v1.Contact[]>([])
let total = $state<number>(0)
let detail = $state<v1.Contact | null>(null)
let loading = $state<boolean>(false)
let listResetSignal = $state(0)
let selectedContactScrollTopSignal = $state(0)
let contactsLoadSeq = 0

export const contactsView = {
  get selectedSourceId(): string {
    return selectedSourceId
  },
  get searchQuery(): string {
    return searchQuery
  },
  get selectedContactId(): string | null {
    return selectedContactId
  },
  get contacts(): v1.Contact[] {
    return contacts
  },
  get total(): number {
    return total
  },
  get detail(): v1.Contact | null {
    return detail
  },
  get loading(): boolean {
    return loading
  },
  get listResetSignal(): number {
    return listResetSignal
  },
  get selectedContactScrollTopSignal(): number {
    return selectedContactScrollTopSignal
  },
}

export function selectSource(sourceId: string): void {
  selectedSourceId = sourceId
  selectedContactId = null
  detail = null
  // Switching category resets the search filter so the new category shows all
  // its contacts (not the intersection with a lingering query). The ContactList
  // mirrors this by clearing its own search input + closing the search bar.
  searchQuery = ''
  listResetSignal += 1
  // Dismiss the sidebar overlay on narrow viewports. Self-gating store
  // call — no-op on full/medium.
  hideSidebar()
  // Caller (ContactsPane) decides when to call reloadContacts().
}

export function setSearchQuery(q: string): void {
  searchQuery = q
}

export async function reloadContacts(limit = 200, offset = 0): Promise<void> {
  const seq = ++contactsLoadSeq
  loading = true
  try {
    const result = await BrowseContacts(searchQuery, selectedSourceId, limit, offset)
    if (seq === contactsLoadSeq) {
      contacts = result?.items || []
      total = result?.total ?? contacts.length
    }
  } catch (err) {
    console.error('Failed to list contacts for browse:', err)
    if (seq === contactsLoadSeq) {
      contacts = []
      total = 0
    }
  } finally {
    if (seq === contactsLoadSeq) {
      loading = false
    }
  }
}

// Focus-vs-activate split mirrors mail's MessageList behavior (and is
// enforced by the kit's ListPane semantics — see ListPane.svelte's onSelect
// and onActivate docstrings):
//
//   focusContact(id)    — j/k navigation. Updates the highlighted row only.
//                         Does NOT load detail, does NOT slide in the viewer.
//   activateContact(id) — Enter key or row click. Loads detail and (on
//                         responsive viewports) reveals the viewer overlay.
//
// Programmatic callers that want the "old" combined behavior (e.g.,
// post-create navigation in ContactsPane.handleCreated) call
// activateContact(id) explicitly.

export function focusContact(id: string | null): void {
  selectedContactId = id
  if (!id) {
    detail = null
  }
  // Intentionally NO detail load and NO showViewer here — focus changes
  // should not move data on/off the network or trigger overlay animations.
}

export async function activateContact(id: string | null): Promise<void> {
  selectedContactId = id
  if (!id) {
    detail = null
    return
  }
  // On responsive viewports, reveal the detail pane overlay. Self-gating
  // store call — no-op on full layout.
  if (isResponsive()) showViewer()
  try {
    detail = await GetContactDetail(id)
  } catch (err) {
    console.error('Failed to load contact detail:', err)
    detail = null
  }
}

export async function activateContactFromGlobalSearch(id: string): Promise<void> {
  selectedSourceId = ''
  searchQuery = ''
  listResetSignal += 1
  await reloadContacts(0)
  await activateContact(id)
  selectedContactScrollTopSignal += 1
}


// Update a local contact with a multi-field patch. On conflict the backend
// emits "contacts:conflict" via the event listener wired in ContactsPane; this
// method's caller doesn't see the conflict directly.
export async function updateContact(id: string, patch: v1.ContactPatch): Promise<void> {
  await UpdateContact(id, patch)
  // Refresh the list + detail view so changes are visible immediately.
  await reloadContacts()
  if (selectedContactId === id) {
    await activateContact(id)
  }
}

// Delete a local (sent-recipient) contact entirely. After deletion the list
// reloads and detail view clears.
export async function deleteLocalContact(email: string): Promise<void> {
  await DeleteLocalContact(email)
  if (selectedContactId === email) {
    selectedContactId = null
    detail = null
  }
  await reloadContacts()
}

// Create a local manual contact. The backend rejects collected/remote source
// IDs; this UI always sends "local:manual".
// Throws on conflict — caller (AddContactDialog) translates "already exists"
// strings into a field-level error.
//
// Does NOT reload contacts or change the selected source — the caller
// (ContactsPane.handleCreated) controls the post-create UX so the dialog can
// close before the source switch.
export async function createContact(input: v1.ContactCreateInput): Promise<string> {
  return await CreateContact(input)
}
