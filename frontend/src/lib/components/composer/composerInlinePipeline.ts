import type { InlineImage } from './composerAttachments'

export function registerInlineImage(
  images: InlineImage[],
  candidate: InlineImage,
): { images: InlineImage[], image: InlineImage, added: boolean } {
  const existing = images.find((image) => image.dataUrl === candidate.dataUrl)
  if (existing) return { images, image: existing, added: false }
  return { images: [...images, candidate], image: candidate, added: true }
}

export function replaceInlineImageSourcesWithCids(html: string, images: InlineImage[]): string {
  let result = html
  for (const image of images) {
    result = result.replaceAll(image.dataUrl, `cid:${image.cid}`)
  }
  return result
}
