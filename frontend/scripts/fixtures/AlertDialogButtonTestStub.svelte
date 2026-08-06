<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    children,
    class: className,
    onclick,
    disabled = false,
  }: {
    children?: Snippet
    class?: string
    onclick?: (event: MouseEvent) => void
    disabled?: boolean
  } = $props()
</script>

<button type="button" data-alert-dialog-button class={className} {disabled} {onclick}>
  {#if children}{@render children()}{/if}
</button>
