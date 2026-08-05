<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    open = $bindable(false),
    onOpenChange,
    children,
  }: {
    open?: boolean
    onOpenChange?: (open: boolean) => void
    children?: Snippet
  } = $props()

  function dismiss() {
    open = false
    onOpenChange?.(false)
  }
</script>

{#if open && children}
  {@render children()}
  <button type="button" data-dialog-root-dismiss onclick={dismiss}>dismiss root</button>
{/if}
