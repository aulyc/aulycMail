export type ComposerSendBlocker =
  | 'no-recipients'
  | 'missing-identity'
  | 'missing-attachment'
  | 'empty-subject'
  | null

export function getComposerSendBlocker(input: {
  recipientCount: number
  hasIdentity: boolean
  attachmentCount: number
  mentionsAttachment: boolean
  subject: string
}): ComposerSendBlocker {
  if (input.recipientCount === 0) return 'no-recipients'
  if (!input.hasIdentity) return 'missing-identity'
  if (input.attachmentCount === 0 && input.mentionsAttachment) return 'missing-attachment'
  if (!input.subject.trim()) return 'empty-subject'
  return null
}
