export type MentionInputMode = 'keyboard' | 'mouse' | 'program'

export interface MentionSelectionState {
  selectedIndex: number
  windowStart: number
  keyboardMode: boolean
  pointerX: number
  pointerY: number
}

export function createMentionSearchController<T>(options: {
  delayMs: number
  search: (query: string) => Promise<T[]>
  onResults: (query: string, results: T[]) => void
  onError?: (error: unknown) => void
}) {
  let timer: ReturnType<typeof setTimeout> | null = null
  let sequence = 0

  const cancel = () => {
    if (timer) clearTimeout(timer)
    timer = null
  }

  return {
    schedule(query: string) {
      cancel()
      const requestSequence = ++sequence
      timer = setTimeout(async () => {
        timer = null
        try {
          const results = await options.search(query)
          if (requestSequence === sequence) options.onResults(query, results)
        } catch (error) {
          if (requestSequence === sequence) options.onError?.(error)
        }
      }, options.delayMs)
    },
    cancel() {
      sequence++
      cancel()
    },
    destroy() {
      sequence++
      cancel()
    },
  }
}

export function selectMentionIndex(
  state: MentionSelectionState,
  suggestionCount: number,
  index: number,
  mode: MentionInputMode,
  visibleRows: number,
): MentionSelectionState {
  if (suggestionCount === 0) {
    return { ...state, selectedIndex: -1, windowStart: 0 }
  }

  const selectedIndex = Math.min(Math.max(index, 0), suggestionCount - 1)
  let windowStart = state.windowStart
  if (selectedIndex < windowStart) {
    windowStart = selectedIndex
  } else if (selectedIndex >= windowStart + visibleRows) {
    windowStart = selectedIndex - visibleRows + 1
  }

  return {
    ...state,
    selectedIndex,
    windowStart,
    keyboardMode: mode === 'keyboard' ? true : mode === 'mouse' ? false : state.keyboardMode,
  }
}

export function clampMentionSelection(
  state: MentionSelectionState,
  suggestionCount: number,
  visibleRows: number,
): MentionSelectionState {
  if (suggestionCount === 0) {
    return { ...state, selectedIndex: -1, windowStart: 0 }
  }
  const selectedIndex = Math.min(Math.max(state.selectedIndex, 0), suggestionCount - 1)
  const maxStart = Math.max(0, suggestionCount - visibleRows)
  return selectMentionIndex(
    { ...state, windowStart: Math.min(state.windowStart, maxStart) },
    suggestionCount,
    selectedIndex,
    'program',
    visibleRows,
  )
}

export function moveMentionPointerSelection(
  state: MentionSelectionState,
  suggestionCount: number,
  index: number,
  pointerX: number,
  pointerY: number,
  visibleRows: number,
): { state: MentionSelectionState, changed: boolean } {
  if (pointerX === state.pointerX && pointerY === state.pointerY) {
    return { state, changed: false }
  }
  return {
    changed: true,
    state: selectMentionIndex(
      { ...state, pointerX, pointerY },
      suggestionCount,
      index,
      'mouse',
      visibleRows,
    ),
  }
}
