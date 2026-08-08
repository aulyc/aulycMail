import { parseFileUris } from './composerUtils'

export interface ComposerClipboardFiles {
  files: File[]
  paths: string[]
  advertisesFiles: boolean
}

function clipboardText(data: DataTransfer, type: string): string {
  try {
    return data.getData(type) || ''
  } catch {
    return ''
  }
}

/**
 * Normalize the browser-visible portion of a paste event. macOS WebKit may
 * expose Finder copies as File objects, file URLs, or only a file-flavoured
 * type marker; the last case is completed through the native bridge.
 */
export function readComposerClipboardFiles(data: DataTransfer | null): ComposerClipboardFiles {
  if (!data) return { files: [], paths: [], advertisesFiles: false }

  const items = Array.from(data.items || [])
  const itemFiles: File[] = []
  for (const item of items) {
    if (item.kind !== 'file' && !item.type.startsWith('image/')) continue
    const file = item.getAsFile()
    if (file) itemFiles.push(file)
  }
  const files = itemFiles.length > 0 ? itemFiles : Array.from(data.files || [])

  const paths = Array.from(new Set([
    ...parseFileUris(clipboardText(data, 'text/uri-list')),
    ...parseFileUris(clipboardText(data, 'public.file-url')),
    ...parseFileUris(clipboardText(data, 'text/plain')),
  ]))
  const types = Array.from(data.types || []).map(type => String(type).toLowerCase())
  const advertisesFiles = files.length > 0 || paths.length > 0 || items.some(item =>
    item.kind === 'file' || item.type.startsWith('image/')
  ) || types.some(type =>
    type === 'files' ||
    type.includes('file-url') ||
    type === 'application/x-moz-file'
  )

  return { files, paths, advertisesFiles }
}
