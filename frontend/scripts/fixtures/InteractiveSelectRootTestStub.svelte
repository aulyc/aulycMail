<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    value = $bindable(''),
    disabled = false,
    onValueChange,
    children,
  }: {
    value?: string
    disabled?: boolean
    onValueChange?: (value: string) => void
    children?: Snippet
  } = $props()

  function choose(next: string) {
    value = next
    onValueChange?.(next)
  }
</script>

<div data-select-root data-value={value} data-disabled={disabled}>
  {#if children}{@render children()}{/if}
  {#each ['none', 'html', 'plain', 'dash', 'asterisk', 'ignored'] as option (option)}
    <button type="button" data-select-value={option} onclick={() => choose(option)}>{option}</button>
  {/each}
</div>
