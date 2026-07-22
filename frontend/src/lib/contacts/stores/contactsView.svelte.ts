// View-local state for the Contacts pane's browse UI. Source selection,
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
import type { contactdto } from '$wailsjs/go/models'
// Responsive (mobile) integration — match mail's pattern of firing
// showViewer/hideSidebar from the consumer's select actions. Layout-store
// calls are self-gating: showViewer is a no-op when not responsive,
// hideSidebar is a no-op when not narrow. Kit primitives (PaneLayout,
// SourceSidebar, DetailPane) handle overlay class application + back
// buttons + scrim; this store handles the "which view do we want to be
// in next" decisions on user actions.
import { isResponsive, showViewer, hideSidebar } from '$lib/stores/layout.svelte'
const CONTACTS_PAGE_SIZE = 200

// Source ID values the sidebar can dispatch:
//   ""                  → all local contacts
//   "local"             → all local contacts (manual + collected)
//   "local:manual"      → user-added local contacts (Add Contact UI)
//   "local:collected"   → auto-collected local contacts (sent-mail recipients)
let selectedSourceId = $state<string>('')
let searchQuery = $state<string>('')
let selectedContactId = $state<string | null>(null)
let contacts = $state<contactdto.ContactListItem[]>([])
let total = $state<number>(0)
let detail = $state<contactdto.Contact | null>(null)
let loading = $state<boolean>(false)
let loadingMore = $state<boolean>(false)
let loadError = $state<boolean>(false)
let detailLoading = $state<boolean>(false)
let detailLoadError = $state<boolean>(false)
let listResetSignal = $state(0)
let selectedContactScrollTopSignal = $state(0)
let contactsLoadSeq = 0
let contactDetailSeq = 0

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
  get contacts(): contactdto.ContactListItem[] {
    return contacts
  },
  get total(): number {
    return total
  },
  get detail(): contactdto.Contact | null {
    return detail
  },
  get loading(): boolean {
    return loading
  },
  get loadingMore(): boolean {
    return loadingMore
  },
  get loadError(): boolean {
    return loadError
  },
  get detailLoading(): boolean {
    return detailLoading
  },
  get detailLoadError(): boolean {
    return detailLoadError
  },
  get hasMore(): boolean {
    return contacts.length < total
  },
  get remaining(): number {
    return Math.max(0, total - contacts.length)
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
  contacts = []
  total = 0
  loadError = false
  detailLoading = false
  detailLoadError = false
  contactDetailSeq += 1
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

export async function reloadContacts(limit = CONTACTS_PAGE_SIZE, offset = 0, preferredIndex = 0): Promise<void> {
  const seq = ++contactsLoadSeq
  let nextDetailId: string | null = null
  loading = true
  loadingMore = false
  loadError = false
  try {
    const result = await BrowseContacts(searchQuery, selectedSourceId, limit, offset)
    if (seq === contactsLoadSeq) {
      contacts = result?.items || []
      total = result?.total ?? contacts.length
      if (contacts.length === 0) {
        selectedContactId = null
        detail = null
        detailLoading = false
        detailLoadError = false
        contactDetailSeq += 1
      } else if (!selectedContactId || !contacts.some((contact) => contact.id === selectedContactId)) {
        const index = Math.max(0, Math.min(preferredIndex, contacts.length - 1))
        nextDetailId = contacts[index].id
        selectedContactId = nextDetailId
        detail = null
        contactDetailSeq += 1
      }
    }
  } catch (err) {
    console.error('Failed to list contacts for browse:', err)
    if (seq === contactsLoadSeq) {
      loadError = true
    }
  } finally {
    if (seq === contactsLoadSeq) {
      loading = false
    }
  }

  // A slow or stalled detail request must never keep the list in its blocking
  // loading state. Start it only after the list request has fully settled.
  if (nextDetailId && seq === contactsLoadSeq) {
    void focusContact(nextDetailId)
  }
}

export async function loadMoreContacts(limit = CONTACTS_PAGE_SIZE): Promise<void> {
  if (loading || loadingMore || contacts.length >= total) return

  const seq = ++contactsLoadSeq
  const offset = contacts.length
  loadingMore = true
  try {
    const result = await BrowseContacts(searchQuery, selectedSourceId, limit, offset)
    if (seq === contactsLoadSeq) {
      const existing = new Set(contacts.map(c => c.id))
      const next = (result?.items || []).filter(c => !existing.has(c.id))
      contacts = [...contacts, ...next]
      total = result?.total ?? total
    }
  } catch (err) {
    console.error('Failed to load more contacts for browse:', err)
  } finally {
    if (seq === contactsLoadSeq) {
      loadingMore = false
    }
  }
}

// Focus-vs-activate keeps keyboard selection and responsive presentation
// separate while ensuring the detail follows the selected contact:
//
//   focusContact(id)    — arrows update the highlighted row and detail without
//                         opening a responsive overlay.
//   activateContact(id) — Enter or a row click also reveals the overlay.

async function selectContact(id: string | null, reveal: boolean): Promise<void> {
  selectedContactId = id
  const seq = ++contactDetailSeq
  if (!id) {
    detail = null
    detailLoading = false
    detailLoadError = false
    return
  }
  if (reveal && isResponsive()) showViewer()
  detail = null
  detailLoading = true
  detailLoadError = false
  try {
    const loaded = await GetContactDetail(id)
    if (seq === contactDetailSeq && selectedContactId === id) detail = loaded
  } catch (err) {
    console.error('Failed to load contact detail:', err)
    if (seq === contactDetailSeq && selectedContactId === id) {
      detail = null
      detailLoadError = true
    }
  } finally {
    if (seq === contactDetailSeq && selectedContactId === id) {
      detailLoading = false
    }
  }
}

export async function focusContact(id: string | null): Promise<void> {
  await selectContact(id, false)
}

export async function activateContact(id: string | null): Promise<void> {
  await selectContact(id, true)
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
export async function updateContact(id: string, patch: contactdto.ContactPatch): Promise<void> {
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
  const deletedIndex = contacts.findIndex((contact) => contact.id === email)
  await DeleteLocalContact(email)
  if (selectedContactId === email) {
    selectedContactId = null
    detail = null
  }
  await reloadContacts(CONTACTS_PAGE_SIZE, 0, Math.max(0, deletedIndex))
}

// Create a local manual contact. The backend rejects collected/remote source
// IDs; this UI always sends "local:manual".
// Throws on conflict — caller (AddContactDialog) translates "already exists"
// strings into a field-level error.
//
// Does NOT reload contacts or change the selected source — the caller
// (ContactsPane.handleCreated) controls the post-create UX so the dialog can
// close before the source switch.
export async function createContact(input: contactdto.ContactCreateInput): Promise<string> {
  return await CreateContact(input)
}
