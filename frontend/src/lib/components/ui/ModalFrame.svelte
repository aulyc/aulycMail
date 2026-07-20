<script lang="ts">
  import { tick, type Snippet } from 'svelte'

  interface Props {
    open?: boolean
    onClose?: () => void
    panelClass: string
    backdropClass?: string
    containerClass?: string
    labelledBy?: string
    children?: Snippet
  }

  let {
    open = false,
    onClose,
    panelClass,
    backdropClass = 'bg-black/80',
    containerClass = 'z-[90] flex items-center justify-center p-4',
    labelledBy,
    children,
  }: Props = $props()

  let panelEl = $state<HTMLDivElement | null>(null)
  let focusRequest = 0
  const focusableSelector = [
    'a[href]',
    'button:not([disabled])',
    'input:not([disabled])',
    'select:not([disabled])',
    'textarea:not([disabled])',
    '[contenteditable="true"]',
    '[tabindex]:not([tabindex="-1"])',
  ].join(',')

  $effect(() => {
    if (!open) return
    const previouslyFocused = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null
    const request = ++focusRequest

    void tick().then(() => {
      requestAnimationFrame(() => {
        if (!open || request !== focusRequest || !panelEl) return
        if (!panelEl.contains(document.activeElement)) {
          panelEl.focus({ preventScroll: true })
        }
      })
    })

    return () => {
      focusRequest++
      if (!previouslyFocused?.isConnected) return
      requestAnimationFrame(() => previouslyFocused.focus({ preventScroll: true }))
    }
  })

  function closeFromBackdrop() {
    onClose?.()
  }

  function getFocusableElements(): HTMLElement[] {
    if (!panelEl) return []
    return [...panelEl.querySelectorAll<HTMLElement>(focusableSelector)].filter((element) => {
      return element.getAttribute('aria-hidden') !== 'true' && !element.hasAttribute('disabled')
    })
  }

  function handlePanelKeydown(event: KeyboardEvent) {
    if (event.defaultPrevented || event.isComposing || event.keyCode === 229) return
    if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      onClose?.()
      return
    }
    if (event.key !== 'Tab') return

    const focusable = getFocusableElements()
    if (focusable.length === 0) {
      event.preventDefault()
      panelEl?.focus({ preventScroll: true })
      return
    }

    const first = focusable[0]
    const last = focusable[focusable.length - 1]
    const active = document.activeElement
    const focusIsOutside = !panelEl?.contains(active) || active === panelEl
    if (event.shiftKey ? (focusIsOutside || active === first) : (focusIsOutside || active === last)) {
      event.preventDefault()
      ;(event.shiftKey ? last : first).focus({ preventScroll: true })
    }
  }
</script>

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="fixed inset-0 {containerClass} {backdropClass}" onclick={closeFromBackdrop}>
    <div
      bind:this={panelEl}
      class={panelClass}
      role="dialog"
      aria-modal="true"
      aria-labelledby={labelledBy}
      tabindex="-1"
      onclick={(event) => event.stopPropagation()}
      onkeydown={handlePanelKeydown}
    >
      {#if children}
        {@render children()}
      {/if}
    </div>
  </div>
{/if}
