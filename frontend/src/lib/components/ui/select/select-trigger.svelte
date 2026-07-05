<script lang="ts">
  import { Select as SelectPrimitive } from 'bits-ui'
  import { cn } from '$lib/utils'
  import Icon from '@iconify/svelte'
  import type { Snippet } from 'svelte'

  interface Props {
    class?: string;
    children?: Snippet;
    showChevron?: boolean;
  }

  let { class: className, children, showChevron = true }: Props = $props()
</script>

<SelectPrimitive.Trigger
  class={cn(
    'flex h-10 w-full items-center justify-between rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground ring-offset-background placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
    className
  )}
>
  {#if children}
    {@render children()}
  {/if}
  {#if showChevron}
    <Icon icon="mdi:chevron-down" class="h-4 w-4 opacity-50" />
  {/if}
</SelectPrimitive.Trigger>
