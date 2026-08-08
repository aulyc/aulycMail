export const INLINE_IMAGE_WARNING_SIZE = 5 * 1024 * 1024
export const INLINE_IMAGE_ATTACHMENT_SIZE = 10 * 1024 * 1024

type InlineImagePolicyDecision = 'inline' | 'confirm' | 'attachment'

export interface InlineImageSizeInput {
  data: string
  size?: number
}

export interface InlineImagePolicyResult {
  decision: InlineImagePolicyDecision
  currentBytes: number
  batchBytes: number
  projectedBytes: number
}

export function base64DecodedSize(base64: string): number {
  const normalized = base64.replace(/\s/g, '')
  if (!normalized) return 0

  const padding = normalized.endsWith('==') ? 2 : normalized.endsWith('=') ? 1 : 0
  return Math.max(0, Math.floor((normalized.length * 3) / 4) - padding)
}

export function formatInlineImageSize(bytes: number): string {
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function imageBytes(image: InlineImageSizeInput): number {
  if (typeof image.size === 'number' && Number.isFinite(image.size) && image.size >= 0) {
    return image.size
  }
  return base64DecodedSize(image.data)
}

function imageKey(image: InlineImageSizeInput, fallback: string): string {
  return image.data ? `data:${image.data}` : fallback
}

export function evaluateInlineImageBatch(
  current: InlineImageSizeInput[],
  batch: InlineImageSizeInput[],
): InlineImagePolicyResult {
  const seen = new Set<string>()
  let currentBytes = 0
  let batchBytes = 0

  current.forEach((image, index) => {
    const key = imageKey(image, `current:${index}`)
    if (seen.has(key)) return
    seen.add(key)
    currentBytes += imageBytes(image)
  })

  batch.forEach((image, index) => {
    const key = imageKey(image, `batch:${index}`)
    if (seen.has(key)) return
    seen.add(key)
    batchBytes += imageBytes(image)
  })

  const projectedBytes = currentBytes + batchBytes
  const decision: InlineImagePolicyDecision = projectedBytes > INLINE_IMAGE_ATTACHMENT_SIZE
    ? 'attachment'
    : projectedBytes > INLINE_IMAGE_WARNING_SIZE
      ? 'confirm'
      : 'inline'

  return { decision, currentBytes, batchBytes, projectedBytes }
}
