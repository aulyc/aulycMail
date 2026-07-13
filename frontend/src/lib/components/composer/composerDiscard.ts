export async function discardDraftBeforeClose(
  draftId: string | null,
  deleteDraft: (draftId: string) => Promise<void>,
  close: () => void,
): Promise<void> {
  if (draftId) {
    await deleteDraft(draftId)
  }
  close()
}
