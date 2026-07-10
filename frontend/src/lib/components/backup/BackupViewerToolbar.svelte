<script lang="ts">
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import * as Select from '$lib/components/ui/select'
  import BackupDirectoryPicker from '$lib/components/backup/BackupDirectoryPicker.svelte'
  // @ts-ignore - wailsjs path
  import type { app } from '../../../../wailsjs/go/models'

  interface Scope {
    id: string
    label: string
    count?: number
  }

  interface Props {
    directory: string
    directoryMenuOpen: boolean
    catalog: app.BackupViewerCatalog | null
    selectedAccountEmail: string
    selectedScope?: Scope
    accountScopes: Scope[]
    loadingCatalog: boolean
    buildingIndex: boolean
    errorMessage: string
    darkFilterEnabled: boolean
    messageSortOrder: 'newest' | 'oldest'
    onLoadCatalog: (dir: string, options?: { fromHistory?: boolean; remember?: boolean }) => void | Promise<void>
    onChooseDirectoryError: () => void
    onRemoveDirectoryHistory: (path: string) => void
    onOpenDirectory: () => void | Promise<void>
    onSelectScope: (scopeID: string) => void | Promise<void>
    onScrollToTop: () => void
    onClearDirectory: () => void
    onRefreshCatalog: () => void | Promise<void>
    onOpenSearch: () => void
    onBuildIndex: () => void | Promise<void>
    onToggleSortOrder: () => void
    onClose: () => void
    scopeLabel: (scope: Scope | undefined) => string
  }

  let {
    directory,
    directoryMenuOpen = $bindable(false),
    catalog,
    selectedAccountEmail,
    selectedScope,
    accountScopes,
    loadingCatalog,
    buildingIndex,
    errorMessage,
    darkFilterEnabled = $bindable(false),
    messageSortOrder,
    onLoadCatalog,
    onChooseDirectoryError,
    onRemoveDirectoryHistory,
    onOpenDirectory,
    onSelectScope,
    onScrollToTop,
    onClearDirectory,
    onRefreshCatalog,
    onOpenSearch,
    onBuildIndex,
    onToggleSortOrder,
    onClose,
    scopeLabel,
  }: Props = $props()
</script>

<header class="relative z-20 flex shrink-0 items-center gap-3 overflow-visible border-b border-border px-4 py-2">
  <h2 class="shrink-0 text-lg font-semibold">{$_('backupViewer.title')}</h2>

  <div class="flex min-w-0 flex-1 items-center gap-2 overflow-visible">
    <div class="w-[340px] min-w-[240px] shrink">
      <BackupDirectoryPicker
        bind:menuOpen={directoryMenuOpen}
        {directory}
        placeholder={$_('backupViewer.directoryPlaceholder')}
        onChoose={(path) => onLoadCatalog(path, { remember: true })}
        onChooseError={onChooseDirectoryError}
        onSelectHistory={(path) => onLoadCatalog(path, { fromHistory: true, remember: true })}
        onRemoveHistory={onRemoveDirectoryHistory}
        onOpenDirectory={onOpenDirectory}
      />
    </div>

    <Select.Root
      value={selectedAccountEmail}
      onValueChange={(value) => void onSelectScope(value)}
      disabled={!catalog?.messageCount}
    >
      <Select.Trigger showChevron={false} class="relative h-10 w-[300px] shrink-0 justify-start border-border bg-background px-3 py-2 text-sm font-semibold shadow-none hover:bg-muted/40 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-0">
        <Icon icon="mdi:chevron-down" class="pointer-events-none absolute left-3 h-4 w-4 opacity-50" />
        <Select.Value class="min-w-0 flex-1 text-left" placeholder={$_('backupViewer.scopeAll')}>
          <span class="grid w-full min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-4 pl-6">
            <span class="truncate">{scopeLabel(selectedScope)}</span>
            {#if typeof selectedScope?.count === 'number'}
              <span class="shrink-0 tabular-nums text-muted-foreground">{selectedScope.count}</span>
            {/if}
          </span>
        </Select.Value>
      </Select.Trigger>
      <Select.Content class="z-[130] w-[300px]">
        {#each accountScopes as scope (scope.id || 'all')}
          <Select.Item value={scope.id} label={scope.label} class="pr-3">
            <span class="grid min-w-0 flex-1 grid-cols-[minmax(0,1fr)_auto] items-center gap-4">
              <span class="truncate">{scope.label}</span>
              {#if typeof scope.count === 'number'}
                <span class="shrink-0 tabular-nums text-muted-foreground">{scope.count}</span>
              {/if}
            </span>
          </Select.Item>
        {/each}
      </Select.Content>
    </Select.Root>

    <div class="flex shrink-0 items-center gap-1" role="toolbar" aria-label={$_('backupViewer.title')}>
      <button
        type="button"
        class="rounded-md p-2 transition-colors hover:bg-muted disabled:cursor-not-allowed disabled:opacity-40"
        disabled={!catalog?.messageCount}
        title={$_('backupViewer.scrollToTop')}
        aria-label={$_('backupViewer.scrollToTop')}
        onclick={onScrollToTop}
      >
        <Icon icon="mdi:arrow-collapse-up" class="h-5 w-5 text-muted-foreground" />
      </button>
      <button
        type="button"
        class="rounded-md p-2 transition-colors hover:bg-muted disabled:cursor-not-allowed disabled:opacity-40"
        disabled={!directory}
        title={$_('backupViewer.clearDirectory')}
        aria-label={$_('backupViewer.clearDirectory')}
        onclick={onClearDirectory}
      >
        <Icon icon="mdi:folder-remove-outline" class="h-5 w-5 text-muted-foreground" />
      </button>
      <button
        type="button"
        class="rounded-md p-2 transition-colors hover:bg-muted disabled:cursor-not-allowed disabled:opacity-40"
        disabled={!directory || loadingCatalog}
        title={$_('backupViewer.refresh')}
        aria-label={$_('backupViewer.refresh')}
        onclick={onRefreshCatalog}
      >
        <Icon icon="mdi:refresh" class="h-5 w-5 text-muted-foreground {loadingCatalog ? 'animate-spin' : ''}" />
      </button>
      <button
        type="button"
        class="rounded-md p-2 transition-colors hover:bg-muted disabled:cursor-not-allowed disabled:opacity-40"
        disabled={!catalog?.messageCount}
        title={$_('backupViewer.search')}
        aria-label={$_('backupViewer.search')}
        onclick={onOpenSearch}
      >
        <Icon icon="mdi:magnify" class="h-5 w-5 text-muted-foreground" />
      </button>
      {#if catalog?.needsIndex}
        <button
          type="button"
          class="rounded-md p-2 transition-colors hover:bg-muted disabled:cursor-not-allowed disabled:opacity-40"
          disabled={!directory || buildingIndex}
          title={$_('backupViewer.buildIndex')}
          aria-label={$_('backupViewer.buildIndex')}
          onclick={onBuildIndex}
        >
          <Icon icon="mdi:database-search-outline" class="h-5 w-5 text-muted-foreground {buildingIndex ? 'animate-spin' : ''}" />
        </button>
      {/if}
      <button
        type="button"
        class="rounded-md p-2 transition-colors hover:bg-muted disabled:cursor-not-allowed disabled:opacity-40"
        disabled={!catalog?.messageCount}
        title={messageSortOrder === 'newest' ? $_('backupViewer.showingNewest') : $_('backupViewer.showingOldest')}
        aria-label={messageSortOrder === 'newest' ? $_('backupViewer.showingNewest') : $_('backupViewer.showingOldest')}
        onclick={onToggleSortOrder}
      >
        <Icon icon={messageSortOrder === 'newest' ? 'mdi:sort-descending' : 'mdi:sort-ascending'} class="h-5 w-5 text-muted-foreground" />
      </button>
      <button
        type="button"
        class="rounded-md p-2 transition-colors hover:bg-muted"
        aria-pressed={darkFilterEnabled}
        aria-label={$_('backupViewer.darkFilter')}
        title={$_('backupViewer.darkFilter')}
        onclick={() => darkFilterEnabled = !darkFilterEnabled}
      >
        <Icon icon={darkFilterEnabled ? 'mdi:weather-night' : 'mdi:white-balance-sunny'} class="h-5 w-5 text-muted-foreground" />
      </button>
    </div>

    {#if errorMessage}
      <span class="max-w-[180px] shrink truncate text-sm text-destructive" title={errorMessage}>{errorMessage}</span>
    {/if}
  </div>

  <button
    type="button"
    class="shrink-0 rounded p-1 text-muted-foreground transition hover:bg-muted hover:text-foreground"
    aria-label={$_('common.close')}
    onclick={onClose}
  >
    <Icon icon="mdi:close" width="20" height="20" />
  </button>
</header>
