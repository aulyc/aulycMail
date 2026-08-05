<script lang="ts">
  import type { Snippet } from 'svelte'

  let { value = '', onValueChange, children }: {
    value?: string
    onValueChange?: (value: string) => void
    children?: Snippet
  } = $props()
</script>

<div data-select-root data-value={value}>
  {#if children}{@render children()}{/if}
  <button type="button" data-select-secondary onclick={() => onValueChange?.('secondary')}>
    select secondary
  </button>
</div>
