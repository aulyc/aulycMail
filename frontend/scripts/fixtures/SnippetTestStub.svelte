<script lang="ts">
  import type { Snippet } from 'svelte'

  let { children }: { children?: Snippet; class?: string } = $props()
</script>

{#if children}{@render children()}{/if}
