export interface KeyboardActionTarget {
  id: string
  label: string
  element: HTMLElement
}

const ACTION_SELECTOR = [
  'button:not(:disabled)',
  'a[href]',
  '[role="button"]:not([aria-disabled="true"])',
  '[role="combobox"]:not([aria-disabled="true"])',
  '[role="checkbox"]:not([aria-disabled="true"])',
  'input[type="checkbox"]:not(:disabled)',
  'input[type="radio"]:not(:disabled)',
].join(',')

function isVisible(element: HTMLElement): boolean {
  if (!element.isConnected || element.hidden) return false
  if (element.getAttribute('aria-hidden') === 'true') return false
  if (element.closest('[hidden], [aria-hidden="true"], [data-keyboard-action-menu]')) return false
  const style = window.getComputedStyle(element)
  if (style.display === 'none' || style.visibility === 'hidden') return false
  const rect = element.getBoundingClientRect()
  return rect.width > 0 && rect.height > 0
}

function compactText(value: string | null | undefined): string {
  return (value ?? '').replace(/\s+/g, ' ').trim()
}

function actionLabel(element: HTMLElement): string {
  const explicit = compactText(
    element.getAttribute('aria-label')
      || element.getAttribute('title')
      || element.getAttribute('data-keyboard-action-label'),
  )
  const label = explicit || compactText(element.textContent)
  if (!label || label.length > 100) return ''

  const contextElement = element.closest<HTMLElement>('[data-keyboard-action-context]')
  const context = compactText(contextElement?.dataset.keyboardActionContext)
  return context && context !== label ? `${label} — ${context}` : label
}

function collectActions(root: HTMLElement): KeyboardActionTarget[] {
  const elements = [
    ...(root.matches(ACTION_SELECTOR) ? [root] : []),
    ...root.querySelectorAll<HTMLElement>(ACTION_SELECTOR),
  ]
  const seenElements = new Set<HTMLElement>()
  const duplicateCounts = new Map<string, number>()

  return elements.flatMap((element, index) => {
    if (seenElements.has(element) || !isVisible(element)) return []
    seenElements.add(element)
    const baseLabel = actionLabel(element)
    if (!baseLabel) return []
    const duplicate = (duplicateCounts.get(baseLabel) ?? 0) + 1
    duplicateCounts.set(baseLabel, duplicate)
    return [{
      id: `keyboard-action-${index}`,
      label: duplicate === 1 ? baseLabel : `${baseLabel} (${duplicate})`,
      element,
    }]
  })
}

function visibleKeyboardRegion(region: string): HTMLElement | null {
  const elements = document.querySelectorAll<HTMLElement>(`[data-keyboard-region="${region}"]`)
  return [...elements].find((element) => isVisible(element)) ?? null
}

class KeyboardActionMenuState {
  open = $state(false)
  actions = $state<KeyboardActionTarget[]>([])

  showForRoot(root: HTMLElement | null): boolean {
    if (!root) return false
    const actions = collectActions(root)
    if (actions.length === 0) return false
    this.actions = actions
    this.open = true
    return true
  }

  showForRegion(region: string): boolean {
    return this.showForRoot(visibleKeyboardRegion(region))
  }

  close(): void {
    this.open = false
    this.actions = []
  }

  activate(action: KeyboardActionTarget): void {
    if (!action.element.isConnected) {
      this.close()
      return
    }
    this.close()
    requestAnimationFrame(() => action.element.click())
  }
}

export const keyboardActionMenu = new KeyboardActionMenuState()
