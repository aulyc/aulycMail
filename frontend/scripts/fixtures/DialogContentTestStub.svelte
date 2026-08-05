<script lang="ts">
  import { onMount, type Snippet } from 'svelte'

  let {
    children,
    onOpenAutoFocus,
    onpointerdown,
  }: {
    children?: Snippet
    onOpenAutoFocus?: (event: Event) => void
    onpointerdown?: (event: PointerEvent) => void
    class?: string
    preventCloseAutoFocus?: boolean
    onInteractOutside?: (event: Event) => void
  } = $props()

  onMount(() => {
    onOpenAutoFocus?.(new Event('open-auto-focus', { cancelable: true }))
  })
</script>

<div role="dialog" tabindex="-1" data-dialog-content onpointerdown={onpointerdown}>
  {#if children}{@render children()}{/if}
</div>
