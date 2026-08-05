<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    open = $bindable(false),
    children,
  }: {
    open?: boolean
    onOpenChange?: (open: boolean) => void
    children?: Snippet
  } = $props()
</script>

{#if open && children}
  {@render children()}
{/if}
