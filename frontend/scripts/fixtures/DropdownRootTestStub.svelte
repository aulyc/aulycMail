<script lang="ts">
  import type { Snippet } from 'svelte'

  let { open = $bindable(false), children }: { open?: boolean; children?: Snippet } = $props()
</script>

<div data-dropdown-open={open}>{#if children}{@render children()}{/if}</div>
