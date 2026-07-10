import type { Editor } from '@tiptap/core'

type ComposerFocusTarget = 'from' | 'to' | 'cc' | 'bcc' | 'subject' | 'body'

type FocusableRef = { focus: () => void }

export type ComposerFocusRefs = {
  fromFieldElement: HTMLDivElement | null
  toFieldElement: HTMLDivElement | null
  ccFieldElement: HTMLDivElement | null
  bccFieldElement: HTMLDivElement | null
  subjectInputElement: HTMLInputElement | null
  composerBodyElement: HTMLDivElement | null
  plainTextRef: HTMLTextAreaElement | null
  toInputRef: FocusableRef | null
  ccInputRef: FocusableRef | null
  bccInputRef: FocusableRef | null
  editor: Editor | null
  isPlainTextMode: boolean
}

function getComposerFocusOrder(showCc: boolean, showBcc: boolean): ComposerFocusTarget[] {
  const order: ComposerFocusTarget[] = ['from', 'to']
  if (showCc) order.push('cc')
  if (showBcc) order.push('bcc')
  order.push('subject', 'body')
  return order
}

function focusComposerTarget(target: ComposerFocusTarget, refs: ComposerFocusRefs): void {
  switch (target) {
    case 'from': {
      const trigger = refs.fromFieldElement?.querySelector<HTMLElement>(
        'button, [role="combobox"], [tabindex]:not([tabindex="-1"])',
      )
      trigger?.focus()
      break
    }
    case 'to':
      refs.toInputRef?.focus()
      break
    case 'cc':
      refs.ccInputRef?.focus()
      break
    case 'bcc':
      refs.bccInputRef?.focus()
      break
    case 'subject':
      refs.subjectInputElement?.focus()
      break
    case 'body':
      if (refs.isPlainTextMode) {
        refs.plainTextRef?.focus()
        return
      }
      refs.editor?.commands.focus()
      break
  }
}

function getActiveComposerFocusTarget(
  refs: Pick<ComposerFocusRefs, 'fromFieldElement' | 'toFieldElement' | 'ccFieldElement' | 'bccFieldElement' | 'subjectInputElement' | 'composerBodyElement'>,
  active: Element | null,
): ComposerFocusTarget | null {
  if (!active) return null
  if (refs.fromFieldElement?.contains(active)) return 'from'
  if (refs.toFieldElement?.contains(active)) return 'to'
  if (refs.ccFieldElement?.contains(active)) return 'cc'
  if (refs.bccFieldElement?.contains(active)) return 'bcc'
  if (refs.subjectInputElement === active) return 'subject'
  if (refs.composerBodyElement?.contains(active)) return 'body'
  return null
}

export function handleComposerTabNavigation(
  event: KeyboardEvent,
  refs: ComposerFocusRefs,
  options: {
    showCc: boolean
    showBcc: boolean
    disabled: boolean
    activeElement: Element | null
  },
): boolean {
  if (options.disabled) return false

  event.preventDefault()
  event.stopPropagation()

  const order = getComposerFocusOrder(options.showCc, options.showBcc)
  const activeTarget = getActiveComposerFocusTarget(refs, options.activeElement)
  const activeIndex = activeTarget ? order.indexOf(activeTarget) : -1
  const nextIndex = event.shiftKey
    ? (activeIndex <= 0 ? order.length - 1 : activeIndex - 1)
    : (activeIndex < 0 || activeIndex >= order.length - 1 ? 0 : activeIndex + 1)

  focusComposerTarget(order[nextIndex], refs)
  return true
}
