<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    children,
    class: className,
    disabled = false,
    onclick,
    onSelect,
    onCloseAutoFocus,
    onOpenAutoFocus,
    side,
    align,
    sideOffset,
  } = $props<{
    children?: Snippet
    class?: string
    disabled?: boolean
    onclick?: (event: MouseEvent) => void
    onSelect?: () => void
    onCloseAutoFocus?: (event: Event) => void
    onOpenAutoFocus?: (event: Event) => void
    side?: string
    align?: string
    sideOffset?: number
  }>()

  let closePrevented = $state(false)
  let openPrevented = $state(false)

  function closeFocus() {
    const event = new Event('close-focus', { cancelable: true })
    onCloseAutoFocus?.(event)
    closePrevented = event.defaultPrevented
  }

  function openFocus() {
    const event = new Event('open-focus', { cancelable: true })
    onOpenAutoFocus?.(event)
    openPrevented = event.defaultPrevented
  }
</script>

<div
  data-primitive
  data-side={side}
  data-align={align}
  data-offset={sideOffset}
  data-close-prevented={closePrevented}
  data-open-prevented={openPrevented}
  class={className}
>
  {#if children}{@render children()}{/if}
  {#if onclick}<button type="button" data-onclick {disabled} {onclick}>primitive click</button>{/if}
  {#if onSelect}<button type="button" data-on-select {disabled} onclick={() => onSelect?.()}>primitive select</button>{/if}
  {#if onCloseAutoFocus}<button type="button" data-close-focus onclick={closeFocus}>close focus</button>{/if}
  {#if onOpenAutoFocus}<button type="button" data-open-focus onclick={openFocus}>open focus</button>{/if}
</div>
