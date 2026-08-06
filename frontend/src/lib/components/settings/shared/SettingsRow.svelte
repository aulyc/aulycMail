<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Popover } from 'bits-ui'
  import type { Snippet } from 'svelte'
  interface Props {
    label: string
    description?: string
    help?: string
    helpLabel?: string
    border?: boolean
    children: Snippet
  }
  let { label, description, help, helpLabel, border = true, children }: Props = $props()
</script>

<div
  data-settings-control-row
  data-keyboard-action-context={label}
  class="flex min-h-14 items-center justify-between gap-6 py-2.5 {border ? 'border-b border-border/75 last:border-b-0' : ''}"
>
  <div class="min-w-0">
    <div class="flex items-center gap-1.5">
      <div class="text-sm font-medium text-foreground">{label}</div>
      {#if help}
        <Popover.Root>
          <Popover.Trigger
            data-settings-help-trigger
            aria-label={helpLabel || label}
            title={helpLabel || label}
            class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-0 focus-visible:ring-offset-0"
          >
            <Icon icon="lucide:circle-help" class="h-4 w-4" />
          </Popover.Trigger>
          <Popover.Portal>
            <Popover.Content
              data-settings-help-popover
              side="top"
              align="start"
              sideOffset={8}
              trapFocus={false}
              class="z-[70] w-80 rounded-lg border border-border bg-popover p-3 text-popover-foreground outline-none"
            >
              <div class="text-sm font-medium">{label}</div>
              <p class="mt-1 text-xs leading-5 text-muted-foreground">{help}</p>
            </Popover.Content>
          </Popover.Portal>
        </Popover.Root>
      {/if}
    </div>
    {#if description}
      <div class="mt-0.5 max-w-xl text-xs leading-5 text-muted-foreground">{description}</div>
    {/if}
  </div>
  <div class="shrink-0">{@render children()}</div>
</div>

<style>
  :global([data-settings-help-popover]) {
    --settings-help-tip-size: 12px;
    filter: drop-shadow(0 12px 12px rgb(0 0 0 / 0.16)) drop-shadow(0 3px 4px rgb(0 0 0 / 0.1));
  }

  :global([data-settings-help-popover])::after {
    position: absolute;
    z-index: 1;
    width: var(--settings-help-tip-size);
    height: var(--settings-help-tip-size);
    content: '';
    background: hsl(var(--popover));
    transform: rotate(45deg);
  }

  :global([data-settings-help-popover][data-side='top'])::after {
    bottom: -5px;
    left: 12px;
    border-right: 1px solid hsl(var(--border));
    border-bottom: 1px solid hsl(var(--border));
  }

  :global([data-settings-help-popover][data-side='bottom'])::after {
    top: -5px;
    left: 12px;
    border-top: 1px solid hsl(var(--border));
    border-left: 1px solid hsl(var(--border));
  }

  :global([data-settings-help-popover][data-side='left'])::after {
    top: 12px;
    right: -5px;
    border-top: 1px solid hsl(var(--border));
    border-right: 1px solid hsl(var(--border));
  }

  :global([data-settings-help-popover][data-side='right'])::after {
    top: 12px;
    left: -5px;
    border-bottom: 1px solid hsl(var(--border));
    border-left: 1px solid hsl(var(--border));
  }
</style>
