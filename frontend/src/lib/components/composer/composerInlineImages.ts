import type { InlineImage } from './composerAttachments'

export const MAX_INLINE_IMAGE_SIZE = 10 * 1024 * 1024

export function createInlineImageCID(index: number, now = Date.now()): string {
  return `image${index}-${now}@aulycmail`
}

interface InlineImageData {
  contentType: string
  data: string
}

function parseInlineImageDataUrl(dataUrl: string): InlineImageData | null {
  const matches = dataUrl.match(/^data:([^;]+);base64,(.+)$/)
  if (!matches) return null

  return {
    contentType: matches[1],
    data: matches[2],
  }
}

function inlineImageExtension(contentType: string): string {
  return contentType.split('/')[1] || 'png'
}

export function createInlineImageFromDataUrl(options: {
  cid: string
  dataUrl: string
  counter: number
  filename?: string
  fallbackPrefix?: string
}): InlineImage | null {
  const parsed = parseInlineImageDataUrl(options.dataUrl)
  if (!parsed) return null

  return {
    cid: options.cid,
    dataUrl: options.dataUrl,
    contentType: parsed.contentType,
    data: parsed.data,
    filename: options.filename || `${options.fallbackPrefix ?? 'image'}${options.counter}.${inlineImageExtension(parsed.contentType)}`,
  }
}

export function createInlineImageFromAttachment(options: {
  cid: string
  dataUrl: string
  contentType: string
  data: string
  filename: string
}): InlineImage {
  return {
    cid: options.cid,
    dataUrl: options.dataUrl,
    contentType: options.contentType,
    data: options.data,
    filename: options.filename,
  }
}
