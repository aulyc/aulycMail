<script lang="ts">
  import type { Snippet } from 'svelte'

  let { value = '', onValueChange, children }: {
    value?: string
    onValueChange?: (value: string) => void
    children?: Snippet
  } = $props()

  const nextValues: Record<string, string> = {
    en: 'zh-CN',
    rich: 'plain',
    ask: 'always',
    'pop-dark': 'light-blue',
    standard: 'large',
  }
</script>

<div data-settings-select data-value={value}>
  {#if children}{@render children()}{/if}
  <button type="button" data-settings-select-change onclick={() => onValueChange?.(nextValues[value] ?? value)}>
    change {value}
  </button>
</div>
