<script lang="ts">
  import * as Dialog from '$lib/components/ui/dialog'
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { syncLog } from '$lib/stores/syncLog.svelte'
  import { formatMessageDate } from '$lib/utils/date'

  interface Props {
    open?: boolean
    onClose?: () => void
  }

  let { open = $bindable(false), onClose }: Props = $props()

  // Date filter ('yyyy-mm-dd' or '' for all). Reset whenever the dialog opens.
  let filterDate = $state('')

  // Clear the unseen-error badge + reset the filter whenever the dialog opens.
  $effect(() => {
    if (open) {
      syncLog.markSeen()
      filterDate = ''
    }
  })

  function dateKey(d: Date): string {
    const y = d.getFullYear()
    const m = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    return `${y}-${m}-${day}`
  }

  function handleOpenAutoFocus(e: Event) {
    e.preventDefault()
  }

  const entries = $derived(syncLog.entries)
  const filtered = $derived(filterDate ? entries.filter((e) => dateKey(e.time) === filterDate) : entries)
</script>

<Dialog.Root bind:open onOpenChange={(v) => { if (!v) onClose?.() }}>
  <Dialog.Content class="max-w-2xl w-[min(90vw,720px)] [&>button]:hidden" onOpenAutoFocus={handleOpenAutoFocus}>
    <!-- Header: title + date filter + clear + close all on one row -->
    <div class="flex items-center gap-2">
      <Icon icon="mdi:history" class="w-4 h-4 text-foreground flex-shrink-0" />
      <Dialog.Title class="text-sm font-semibold text-foreground">{$_('syncLog.title')}</Dialog.Title>
      <div class="ml-auto flex items-center gap-2">
        <input
          type="date"
          bind:value={filterDate}
          class="bg-muted text-foreground text-xs rounded px-2 py-1 border border-border focus:outline-none focus-visible:ring-2 focus-visible:ring-ring [color-scheme:dark]"
        />
        {#if filterDate}
          <button
            onclick={() => filterDate = ''}
            class="text-xs text-muted-foreground hover:text-foreground transition-colors focus:outline-none"
          >
            {$_('syncLog.allDates')}
          </button>
        {/if}
        {#if entries.length > 0}
          <button
            onclick={() => syncLog.clear()}
            class="px-2 py-1 text-xs rounded hover:bg-muted text-muted-foreground transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-ring ml-2"
            title={$_('syncLog.clear')}
          >
            {$_('syncLog.clear')}
          </button>
        {/if}
        <button
          onclick={() => { open = false; onClose?.() }}
          class="p-1.5 rounded hover:bg-muted transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={$_('common.close')}
        >
          <Icon icon="mdi:close" class="w-4 h-4 text-muted-foreground" />
        </button>
      </div>
    </div>

    <!-- Fixed-height, scrolling body -->
    <div class="h-[55vh] overflow-y-auto scrollbar-thin border-y border-border pr-5 [scrollbar-gutter:stable]">
      {#if filtered.length === 0}
        <div class="h-full flex items-center justify-center text-sm text-muted-foreground">
          {$_('syncLog.empty')}
        </div>
      {:else}
        <div class="divide-y divide-border">
          {#each filtered as e (e.id)}
            <div class="flex items-start gap-3 py-2">
              <Icon
                icon={e.level === 'error' ? 'mdi:alert-circle' : e.level === 'success' ? 'mdi:check-circle' : 'mdi:information'}
                class="w-4 h-4 flex-shrink-0 mt-0.5 {e.level === 'error' ? 'text-destructive' : e.level === 'success' ? 'text-green-500' : 'text-muted-foreground'}"
              />
              <div class="flex-1 min-w-0">
                <div class="flex items-baseline gap-2 min-w-0">
                  <span class="text-sm text-foreground truncate flex-1 min-w-0">{e.target}</span>
                  <span class="text-xs text-muted-foreground whitespace-nowrap flex-shrink-0 tabular-nums">{formatMessageDate(e.time)}</span>
                </div>
                <div class="text-xs {e.level === 'error' ? 'text-destructive' : 'text-muted-foreground'}">
                  {e.level === 'error' ? $_('syncLog.failed') : e.level === 'success' ? $_('syncLog.succeeded') : ''}
                </div>
                {#if e.detail}
                  <div class="text-xs text-muted-foreground/80 break-words mt-0.5 font-mono">{e.detail}</div>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </Dialog.Content>
</Dialog.Root>
