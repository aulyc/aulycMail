<script lang="ts">
  import type { Snippet } from 'svelte'
  interface Props { title?: string; description?: string; framed?: boolean; bottomBorder?: boolean; children: Snippet }
  let { title = '', description = '', framed = true, bottomBorder = true, children }: Props = $props()
</script>

<section class="space-y-2">
  {#if title || description}
    <div>
      {#if title}<h3 class="text-sm font-semibold text-foreground">{title}</h3>{/if}
      {#if description}<p class="mt-0.5 text-xs text-muted-foreground">{description}</p>{/if}
    </div>
  {/if}
  <div class="overflow-visible {framed ? 'rounded-xl border border-border bg-card/70 shadow-sm' : `border-t border-border/75 ${bottomBorder ? 'border-b' : ''}`}">
    {@render children()}
  </div>
</section>
