// Cached account-group summary for the Contacts sidebar. The sidebar is
// mounted only when the Contacts view is active, so keeping this state outside
// the component prevents the account list from flashing empty on every switch.

// @ts-ignore - wailsjs bindings
import { Contacts_GetContactAccountGroups } from '$wailsjs/go/app/App'

export type ContactAccountGroup = {
  accountId: string
  name?: string
  email: string
  count: number
  senderCount: number
  recipientCount: number
  ccCount: number
  bccCount: number
}

let groups = $state<ContactAccountGroup[]>([])
let loading = $state(false)
let loaded = $state(false)
let loadPromise: Promise<void> | null = null

export const contactAccountGroups = {
  get groups(): ContactAccountGroup[] {
    return groups
  },
  get loading(): boolean {
    return loading
  },
  get loaded(): boolean {
    return loaded
  },
}

export async function loadContactAccountGroups(options: { force?: boolean } = {}): Promise<void> {
  if (loading && loadPromise) return loadPromise
  if (loaded && !options.force) return

  loading = true
  loadPromise = (async () => {
    try {
      groups = await Contacts_GetContactAccountGroups() || []
      loaded = true
    } catch (err) {
      console.error('Failed to load contact account groups:', err)
      if (!loaded) groups = []
    } finally {
      loading = false
      loadPromise = null
    }
  })()

  return loadPromise
}

export function preloadContactAccountGroups(): void {
  void loadContactAccountGroups()
}
