<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    children,
    onOpenChange,
  }: {
    children?: Snippet
    onOpenChange?: (open: boolean) => void
  } = $props()
</script>

<section data-context-menu-root>
  <button type="button" data-context-menu-open onclick={() => onOpenChange?.(true)}>open menu</button>
  <button type="button" data-context-menu-close onclick={() => onOpenChange?.(false)}>close menu</button>
  {#if children}{@render children()}{/if}
</section>
