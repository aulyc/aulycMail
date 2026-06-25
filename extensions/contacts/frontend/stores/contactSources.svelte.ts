// Local-only contact-sources stub.
//
// aulycmail keeps a single local address book — there are no remote contact
// sources (CardDAV / Google / Microsoft were removed). This store preserves the
// narrow interface the contacts components still reference (sources,
// isSourceWritable, load) so they compile unchanged, while reporting an empty
// source list and treating every local contact as writable.

// Minimal source shape the contacts components still reference (.id / .name /
// .type / .writable). There are no remote sources anymore, so this list is
// always empty — the type just keeps the consumers' field access type-checking.
type ContactSourceLite = { id: string; name: string; type: string; writable: boolean; accountId: string }

function createContactSourcesStore() {
  const sources: ContactSourceLite[] = []

  async function load(): Promise<void> {
    // No remote sources to load.
  }

  // Every contact lives in the local address book, which is always writable.
  function isSourceWritable(_sourceId: string | undefined): boolean {
    return true
  }

  return {
    get sources(): ContactSourceLite[] {
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
