<script lang="ts">
  import type { Snippet } from 'svelte'

  let { label = '', children }: { label?: string; children?: Snippet; value?: string } = $props()
</script>

<span data-select-item>{#if children}{@render children()}{:else}{label}{/if}</span>
