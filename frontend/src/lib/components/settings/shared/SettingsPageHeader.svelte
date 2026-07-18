<script lang="ts">
  import type { Snippet } from 'svelte'
  interface Props { description: string; action?: Snippet }
  let { description, action }: Props = $props()
</script>

<header class="flex min-h-5 min-w-0 items-center justify-between gap-4">
  <p class="min-w-0 flex-1 truncate text-sm text-muted-foreground">{description}</p>
  {#if action}<div class="shrink-0">{@render action()}</div>{/if}
</header>
