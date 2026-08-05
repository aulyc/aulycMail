<script lang="ts">
  import type { Snippet } from 'svelte'

  let { children, ...rest }: { children?: Snippet; [key: string]: unknown } = $props()
</script>

<button type="button" data-dropdown-trigger {...rest}>{#if children}{@render children()}{/if}</button>
