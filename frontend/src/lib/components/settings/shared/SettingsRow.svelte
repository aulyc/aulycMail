<script lang="ts">
  import type { Snippet } from 'svelte'
  interface Props {
    label: string
    description?: string
    border?: boolean
    children: Snippet
  }
  let { label, description = '', border = true, children }: Props = $props()
</script>

<div class="flex min-h-14 items-center justify-between gap-6 px-4 py-2.5 {border ? 'border-b border-border/75 last:border-b-0' : ''}">
  <div class="min-w-0">
    <div class="text-sm font-medium text-foreground">{label}</div>
    {#if description}<p class="mt-0.5 text-xs leading-5 text-muted-foreground">{description}</p>{/if}
  </div>
  <div class="shrink-0">{@render children()}</div>
</div>
