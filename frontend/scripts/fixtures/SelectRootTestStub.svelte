<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    value = $bindable(''),
    disabled = false,
    children,
  }: {
    value?: string
    disabled?: boolean
    onValueChange?: (value: string) => void
    children?: Snippet
  } = $props()
</script>

<div data-select-root data-value={value} data-disabled={disabled}>
  {#if children}{@render children()}{/if}
</div>
