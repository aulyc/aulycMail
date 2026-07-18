interface ExternalOpenFilesPayload {
  paths?: unknown
}

export interface ExternalComposerAttachment {
  filename: string
  contentType: string
  size: number
  data: string
}

export interface ExternalSmtpAttachment {
  filename: string
  content_type: string
  content: number[]
  content_base64: string
  content_id: string
  inline: boolean
}

export function normalizeExternalOpenFiles(payload: unknown): string[] {
  if (!payload || typeof payload !== 'object') return []
  const paths = (payload as ExternalOpenFilesPayload).paths
  if (!Array.isArray(paths)) return []

  const seen = new Set<string>()
  const normalized: string[] = []
  for (const path of paths) {
    if (typeof path !== 'string' || path.length === 0 || seen.has(path)) continue
    seen.add(path)
    normalized.push(path)
  }
  return normalized
}

export function toExternalSmtpAttachment(att: ExternalComposerAttachment): ExternalSmtpAttachment {
  return {
    filename: att.filename,
    content_type: att.contentType || 'application/octet-stream',
    content: [],
    content_base64: att.data,
    content_id: '',
    inline: false,
  }
}
