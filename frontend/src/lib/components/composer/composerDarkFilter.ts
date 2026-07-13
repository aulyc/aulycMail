export type ComposerDarkFilterMode = 'new' | 'reply' | 'reply-all' | 'forward'

export interface ComposerDarkFilterNode {
  type: string
  textContent: string
}

const FORWARDED_MESSAGE_SEPARATOR = '---------- Forwarded message ----------'

/**
 * Return the top-level editor nodes that contain quoted/original mail content.
 * Composer-owned content (typing area, signature, citation/forward headers) is
 * intentionally excluded so it keeps the app theme's native contrast.
 */
export function getComposerDarkFilterTargetIndexes(
  mode: ComposerDarkFilterMode,
  nodes: ComposerDarkFilterNode[],
): number[] {
  if (mode === 'reply' || mode === 'reply-all') {
    const blockquoteIndex = nodes.findIndex(node => node.type === 'blockquote')
    return blockquoteIndex >= 0 ? [blockquoteIndex] : []
  }

  if (mode === 'forward') {
    const headerIndex = nodes.findIndex(node =>
      node.textContent.includes(FORWARDED_MESSAGE_SEPARATOR)
    )
    if (headerIndex < 0) return []
    return nodes.slice(headerIndex + 1).map((_, index) => headerIndex + 1 + index)
  }

  return []
}
