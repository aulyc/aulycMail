<script lang="ts">
  import type { Snippet } from 'svelte'
  interface Props {
    label: string
    border?: boolean
    children: Snippet
  }
  let { label, border = true, children }: Props = $props()
</script>

<div
  data-settings-control-row
  data-settings-control-selected="false"
  class="flex min-h-14 items-center justify-between gap-6 rounded-lg py-2.5 transition-colors data-[settings-control-selected=true]:bg-primary/15 {border ? 'border-b border-border/75 last:border-b-0' : ''}"
>
  <div class="min-w-0">
    <div class="text-sm font-medium text-foreground">{label}</div>
  </div>
  <div class="shrink-0">{@render children()}</div>
</div>
