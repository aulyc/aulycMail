import type { account, smtp } from '../../../../wailsjs/go/models'
import type { Editor } from '@tiptap/core'
import type { ComposerAttachment } from './composerAttachments'

export type ComposerDisplayMode = 'new' | 'reply' | 'reply-all' | 'forward'

export function widenComposerLabel(label: string): string {
  const chars = [...label]
  if (chars.length === 2 && /^[一-鿿]{2}$/.test(label)) {
    return chars[0] + '　' + chars[1]
  }
  return label
}

export function formatIdentityLabel(identity: account.Identity | null | undefined): string {
  if (!identity) return ''

  const email = (identity.email || '').trim()
  const name = (identity.name || '').trim()
  if (!name || name.toLowerCase() === email.toLowerCase()) return email

  return `${name} <${email}>`
}

export function getComposerDisplayMode(input: {
  initialMessage?: smtp.ComposeMessage | null
  replyType: 'reply' | 'reply-all' | 'forward' | ''
}): ComposerDisplayMode {
  if (!input.initialMessage) return 'new'
  if (input.replyType) return input.replyType
  if (input.initialMessage.subject?.startsWith('Fwd:')) return 'forward'
  if (input.initialMessage.in_reply_to) {
    if ((input.initialMessage.to?.length || 0) > 1 || (input.initialMessage.cc?.length || 0) > 0) {
      return 'reply-all'
    }
    return 'reply'
  }
  return 'new'
}

export function composerHasContent(input: {
  toCount: number
  ccCount: number
  bccCount: number
  subject: string
  isPlainTextMode: boolean
  plainTextContent: string
  editor: Editor | null
  attachments: ComposerAttachment[]
}): boolean {
  const bodyText = input.isPlainTextMode
    ? input.plainTextContent.trim()
    : (input.editor?.getText()?.trim() || '')

  return input.toCount > 0 ||
    input.ccCount > 0 ||
    input.bccCount > 0 ||
    input.subject.trim() !== '' ||
    bodyText !== '' ||
    input.attachments.length > 0
}
