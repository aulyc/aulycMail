<script lang="ts">
  import { Select as SelectPrimitive } from 'bits-ui'
  import { cn } from '$lib/utils'
  import Icon from '@iconify/svelte'
  import type { Snippet } from 'svelte'

  interface Props {
    value: string;
    label?: string;
    disabled?: boolean;
    class?: string;
    children?: Snippet;
  }

  let {
    value,
    label,
    disabled = false,
    class: className,
    children: itemContent,
  }: Props = $props()
</script>

<SelectPrimitive.Item
  {value}
  {label}
  {disabled}
  class={cn(
    'flex w-full cursor-default select-none items-center gap-2 rounded-sm py-1.5 pl-2 pr-2 text-sm outline-none',
    'text-popover-foreground',
    'focus:bg-accent focus:text-accent-foreground',
    'data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground',
    'data-[disabled]:pointer-events-none data-[disabled]:opacity-50',
    className
  )}
>
  {#snippet children({ selected })}
    <span class="min-w-0 flex-1">
      {#if itemContent}
        {@render itemContent()}
      {:else}
        <span class="block truncate">{label || value}</span>
      {/if}
    </span>
    <span class="flex h-3.5 w-3.5 shrink-0 items-center justify-center">
      {#if selected}
        <Icon icon="mdi:check" class="h-4 w-4" />
      {/if}
    </span>
  {/snippet}
</SelectPrimitive.Item>
