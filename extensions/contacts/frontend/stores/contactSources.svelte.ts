// Local-only contact-sources stub.
//
// aulycmail keeps a single local address book — there are no remote contact
// sources (CardDAV / Google / Microsoft were removed). This store preserves the
// narrow interface the contacts components still reference (sources,
// isSourceWritable, load) so they compile unchanged, while reporting an empty
// source list and treating every local contact as writable.

// @ts-ignore - wailsjs bindings
import type { v1 } from '$wailsjs/go/models'

function createContactSourcesStore() {
  const sources: v1.ContactSource[] = []

  async function load(): Promise<void> {
    // No remote sources to load.
  }

  // Every contact lives in the local address book, which is always writable.
  function isSourceWritable(_sourceId: string | undefined): boolean {
    return true
  }

  return {
    get sources(): v1.ContactSource[] {
      return sources
    },
    get loading(): boolean {
      return false
    },
    get syncing(): boolean {
      return false
    },
    load,
    isSourceWritable,
  }
}

export const contactSourcesStore = createContactSourcesStore()
