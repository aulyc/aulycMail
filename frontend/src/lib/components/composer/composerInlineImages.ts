export const MAX_INLINE_IMAGE_SIZE = 10 * 1024 * 1024

export function createInlineImageCID(index: number, now = Date.now()): string {
  return `image${index}-${now}@aulycmail`
}
