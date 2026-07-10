import {
  readFileAsBase64,
  readFileAsDataUrl,
} from './composerUtils'

export interface ComposerAttachment {
  filename: string
  contentType: string
  size: number
  data: string
}

export interface InlineImage {
  cid: string
  dataUrl: string
  contentType: string
  data: string
  filename: string
}

export async function fileToComposerAttachment(file: File): Promise<ComposerAttachment> {
  const data = await readFileAsBase64(file)
  return {
    filename: file.name,
    contentType: file.type || 'application/octet-stream',
    size: file.size,
    data,
  }
}

export async function selectedFileToComposerAttachment(file: File): Promise<ComposerAttachment | null> {
  const dataUrl = await readFileAsDataUrl(file)
  const matches = dataUrl.match(/^data:([^;]+);base64,(.+)$/)
  if (!matches) return null

  return {
    filename: file.name,
    contentType: matches[1],
    size: file.size,
    data: matches[2],
  }
}

export function backendAttachmentToComposerAttachment(att: {
  filename: string
  contentType: string
  size: number
  data: string
}): ComposerAttachment {
  return {
    filename: att.filename,
    contentType: att.contentType,
    size: att.size,
    data: att.data,
  }
}

export function estimateBase64DecodedSize(base64: string): number {
  return Math.ceil((base64.length * 3) / 4)
}

export async function fileToDataUrl(file: File): Promise<string> {
  return readFileAsDataUrl(file)
}

export function isRecipientChipDrag(e: DragEvent): boolean {
  return !!e.dataTransfer?.types.includes('application/x-aulycmail-recipient')
}

export function hasFileDropPayload(e: DragEvent): boolean {
  const types = e.dataTransfer?.types
  return !!types?.includes('Files') || !!types?.includes('text/uri-list')
}
