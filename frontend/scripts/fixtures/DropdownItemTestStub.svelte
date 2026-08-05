<script lang="ts">
  import type { Snippet } from 'svelte'

  let { children, onSelect }: { children?: Snippet; onSelect?: () => void } = $props()
</script>

<button type="button" data-dropdown-item onclick={() => onSelect?.()}>{#if children}{@render children()}{/if}</button>
