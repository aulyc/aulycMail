// @ts-ignore - Wails generated imports
import { smtp } from '../../../../wailsjs/go/models'
import type { ComposerAttachment, InlineImage } from './composerAttachments'

type AddressLike = {
  name?: string
  address?: string
  email?: string
} | null | undefined

type IdentityLike = {
  name?: string
  email?: string
} | null | undefined

export function toSmtpAddress(address: AddressLike): smtp.Address {
  return new smtp.Address({
    name: address?.name || '',
    address: address?.address || address?.email || '',
  })
}

export function restoreBlockedRemoteImages(html: string): string {
  return html.replace(
    /<img([^>]*)\sdata-original-src="([^"]+)"([^>]*)>/gi,
    (match, _before, originalSrc) => match
      .replace(/src="[^"]*"/, `src="${originalSrc}"`)
      .replace(/\s*data-original-src="[^"]*"/, ''),
  )
}

export function buildComposeMessage(input: {
  identity: IdentityLike
  to: smtp.Address[]
  cc: smtp.Address[]
  bcc: smtp.Address[]
  subject: string
  htmlBody: string
  textBody: string
  attachments: ComposerAttachment[]
  inlineImages: InlineImage[]
  inReplyTo?: string
  references: string[]
  sourceMessageId: string
  replyType: string
  requestReadReceipt: boolean
}): smtp.ComposeMessage {
  const attachments = input.attachments.map((attachment) => new smtp.Attachment({
    filename: attachment.filename,
    content_type: attachment.contentType,
    content_base64: attachment.data,
    content_id: '',
    inline: false,
  }))

  for (const image of input.inlineImages) {
    attachments.push(new smtp.Attachment({
      filename: image.filename,
      content_type: image.contentType,
      content_base64: image.data,
      content_id: image.cid,
      inline: true,
    }))
  }

  return new smtp.ComposeMessage({
    from: toSmtpAddress(input.identity),
    to: input.to,
    cc: input.cc,
    bcc: input.bcc,
    subject: input.subject,
    html_body: input.htmlBody,
    text_body: input.textBody,
    attachments,
    in_reply_to: input.inReplyTo,
    references: input.references,
    source_message_id: input.sourceMessageId,
    reply_type: input.replyType,
    request_read_receipt: input.requestReadReceipt,
  })
}
