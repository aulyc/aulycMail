<script lang="ts">
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import ModalFrame from '$lib/components/ui/ModalFrame.svelte'
  import SearchScopeCarousel from '$lib/components/search/SearchScopeCarousel.svelte'
  // @ts-ignore - wailsjs path
  import type { app } from '../../../../wailsjs/go/models'

  interface Scope {
    id: string
    label: string
    count?: number
  }

  interface Props {
    open: boolean
    accountScopes: Scope[]
    searchScopeEmail: string
    searchQuery: string
    searchResults: app.BackupViewerMessageSummary[]
    searchLoading: boolean
    searchActiveIndex: number
    searchInputEl: HTMLInputElement | null
    onClose: () => void
    onSelectSearchScope: (scopeID: string) => void
    onSearchInput: () => void
    onCompositionStart: () => void
    onCompositionEnd: () => void
    onSearchKeydown: (event: KeyboardEvent) => void
    onSelectSearchResult: (index: number) => void | Promise<void>
    messageAttachmentCount: (message: app.BackupViewerMessageSummary) => number
    formatShortDate: (value: string) => string
  }

  let {
    open,
    accountScopes,
    searchScopeEmail,
    searchQuery = $bindable(''),
    searchResults,
    searchLoading,
    searchActiveIndex = $bindable(0),
    searchInputEl = $bindable(null),
    onClose,
    onSelectSearchScope,
    onSearchInput,
    onCompositionStart,
    onCompositionEnd,
    onSearchKeydown,
    onSelectSearchResult,
    messageAttachmentCount,
    formatShortDate,
  }: Props = $props()
</script>

{#if open}
  <ModalFrame
    {open}
    onClose={onClose}
    containerClass="z-[120]"
    backdropClass="bg-black/75"
    panelClass="mx-auto mt-[calc(12vh+52px)] w-[min(90vw,680px)]"
  >
    <div class="mb-3 overflow-hidden">
      <SearchScopeCarousel
        scopes={accountScopes}
        selectedId={searchScopeEmail}
        maxLabelWidthClass="max-w-[240px]"
        onSelect={onSelectSearchScope}
      />
    </div>

    <div class="overflow-hidden rounded-xl border border-border bg-popover shadow-2xl">
      <div class="flex items-center gap-2 border-b border-border px-4 py-3">
        <Icon icon="mdi:magnify" class="h-5 w-5 shrink-0 text-muted-foreground" />
        <input
          bind:this={searchInputEl}
          bind:value={searchQuery}
          oninput={onSearchInput}
          oncompositionstart={onCompositionStart}
          oncompositionend={onCompositionEnd}
          onkeydown={onSearchKeydown}
          placeholder={$_('backupViewer.searchPlaceholder')}
          class="min-w-0 flex-1 border-none bg-transparent text-base text-foreground outline-none"
        />
        {#if searchLoading}
          <Icon icon="mdi:loading" class="h-4 w-4 shrink-0 animate-spin text-muted-foreground" />
        {/if}
        <button type="button" class="shrink-0 rounded p-1 hover:bg-muted" onclick={onClose} aria-label={$_('common.close')}>
          <Icon icon="mdi:close" class="h-5 w-5 text-muted-foreground" />
        </button>
      </div>

      {#if searchQuery.trim() && searchResults.length === 0 && !searchLoading}
        <div class="px-4 py-6 text-center text-sm text-muted-foreground">{$_('backupViewer.searchNoResults')}</div>
      {:else if searchResults.length > 0}
        <div class="max-h-[55vh] overflow-y-auto py-1 scrollbar-thin">
          {#each searchResults as result, index (result.key)}
            <button
              type="button"
              class="flex w-full items-start gap-3 px-4 py-2 text-left {index === searchActiveIndex ? 'bg-muted' : 'hover:bg-muted/50'}"
              onclick={() => onSelectSearchResult(index)}
              onmousemove={() => searchActiveIndex = index}
            >
              <Icon icon="mdi:email-outline" class="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
              <span class="min-w-0 flex-1">
                <span class="flex min-w-0 items-baseline gap-2">
                  <span class="min-w-0 flex-1 truncate text-sm text-foreground">{result.subject || $_('backupViewer.unknownSubject')}</span>
                  {#if messageAttachmentCount(result) > 0}
                    <span class="shrink-0 text-amber-600 dark:text-amber-500" title={$_('backupViewer.attachments')}>
                      <Icon icon="mdi:paperclip" class="h-4 w-4" />
                    </span>
                  {/if}
                  <span class="shrink-0 text-xs text-muted-foreground">{formatShortDate(result.date)}</span>
                </span>
                <span class="truncate text-xs text-muted-foreground">{result.accountEmail}{result.folderPath ? ` / ${result.folderPath}` : ''}</span>
              </span>
            </button>
          {/each}
        </div>
      {/if}
    </div>
  </ModalFrame>
{/if}
