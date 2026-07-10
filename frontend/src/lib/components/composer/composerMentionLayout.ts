const MENTION_MENU_WIDTH = 288
const MENTION_ROW_HEIGHT = 52
export const MENTION_VISIBLE_ROWS = 4
const MENTION_MENU_BORDER_WIDTH = 2
export const MENTION_MENU_HEIGHT = MENTION_ROW_HEIGHT * MENTION_VISIBLE_ROWS + MENTION_MENU_BORDER_WIDTH

const MENTION_VIEWPORT_PADDING = 8
const MENTION_ANCHOR_GAP = 6

export type MentionPosition = {
  left: number
  top: number
}

export function clampMentionPosition(input: {
  left: number
  top: number
  container: HTMLDivElement | null
}): MentionPosition {
  if (!input.container) {
    return { left: input.left, top: input.top }
  }

  const container = input.container
  const viewportTop = container.scrollTop
  const viewportBottom = viewportTop + container.clientHeight
  const minTop = viewportTop + MENTION_VIEWPORT_PADDING
  const maxTop = Math.max(minTop, viewportBottom - MENTION_MENU_HEIGHT - MENTION_VIEWPORT_PADDING)
  const belowTop = input.top + MENTION_ANCHOR_GAP
  const aboveTop = input.top - MENTION_MENU_HEIGHT - MENTION_ANCHOR_GAP
  const hasRoomBelow = belowTop + MENTION_MENU_HEIGHT <= viewportBottom - MENTION_VIEWPORT_PADDING
  const hasRoomAbove = aboveTop >= minTop
  const nextTop = !hasRoomBelow && hasRoomAbove ? aboveTop : Math.min(Math.max(minTop, belowTop), maxTop)
  const maxLeft = Math.max(MENTION_VIEWPORT_PADDING, container.clientWidth - MENTION_MENU_WIDTH - MENTION_VIEWPORT_PADDING)

  return {
    left: Math.min(Math.max(MENTION_VIEWPORT_PADDING, input.left), maxLeft),
    top: nextTop,
  }
}

export function getPlainMentionPosition(input: {
  textarea: HTMLTextAreaElement
  container: HTMLDivElement
  markerOffset: number
}): MentionPosition {
  const { textarea, container } = input
  const style = getComputedStyle(textarea)
  const mirror = document.createElement('div')
  mirror.style.position = 'absolute'
  mirror.style.visibility = 'hidden'
  mirror.style.whiteSpace = 'pre-wrap'
  mirror.style.wordBreak = 'break-word'
  mirror.style.overflowWrap = 'break-word'
  mirror.style.boxSizing = style.boxSizing
  mirror.style.width = `${textarea.clientWidth}px`
  mirror.style.font = style.font
  mirror.style.letterSpacing = style.letterSpacing
  mirror.style.lineHeight = style.lineHeight
  mirror.style.padding = style.padding
  mirror.style.border = style.border

  const markerPosition = Math.max(0, Math.min(input.markerOffset, textarea.value.length))
  mirror.textContent = textarea.value.slice(0, markerPosition)
  const marker = document.createElement('span')
  marker.textContent = textarea.value.slice(markerPosition, markerPosition + 1) || '.'
  mirror.appendChild(marker)
  document.body.appendChild(mirror)

  const textareaRect = textarea.getBoundingClientRect()
  const containerRect = container.getBoundingClientRect()
  const lineHeight = Number.parseFloat(style.lineHeight) || 20
  const left = textareaRect.left - containerRect.left + marker.offsetLeft - textarea.scrollLeft
  const top = textareaRect.top - containerRect.top + marker.offsetTop - textarea.scrollTop + lineHeight + container.scrollTop
  mirror.remove()

  return { left, top }
}
