<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    value = $bindable(''),
    disabled = false,
    onValueChange,
    children,
  } = $props<{
    value?: string
    disabled?: boolean
    onValueChange?: (value: string | undefined) => void
    children?: Snippet
  }>()

  function choose(next: string | undefined) {
    value = next ?? ''
    onValueChange?.(next)
  }
</script>

<div data-bool-select-root data-value={value} data-disabled={disabled}>
  {#if children}{@render children()}{/if}
  <button type="button" data-bool-value="yes" onclick={() => choose('yes')}>yes</button>
  <button type="button" data-bool-value="no" onclick={() => choose('no')}>no</button>
  <button type="button" data-bool-value="undefined" onclick={() => choose(undefined)}>undefined</button>
</div>
