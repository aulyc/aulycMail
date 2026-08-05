<script lang="ts">
  let {
    directory,
    directoryMenuOpen = $bindable(false),
    catalog,
    selectedAccountEmail,
    loadingCatalog,
    buildingIndex,
    errorMessage,
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
  } = $props()
</script>

<header
  data-backup-toolbar
  data-directory={directory}
  data-message-count={catalog?.messageCount ?? 0}
  data-scope={selectedAccountEmail}
  data-loading={loadingCatalog}
  data-building={buildingIndex}
  data-error={errorMessage}
  data-sort={messageSortOrder}
  data-menu-open={directoryMenuOpen}
>
  <button type="button" data-backup-action="load" onclick={() => onLoadCatalog('/synthetic/backup', { remember: true })}>load</button>
  <button type="button" data-backup-action="load-history" onclick={() => onLoadCatalog('/missing/backup', { fromHistory: true })}>history</button>
  <button type="button" data-backup-action="load-blank" onclick={() => onLoadCatalog('   ')}>blank</button>
  <button type="button" data-backup-action="load-plain-failure" onclick={() => onLoadCatalog('/plain-failure')}>plain failure</button>
  <button type="button" data-backup-action="choose-error" onclick={onChooseDirectoryError}>choose error</button>
  <button type="button" data-backup-action="remove-history" onclick={() => onRemoveDirectoryHistory(directory)}>remove history</button>
  <button type="button" data-backup-action="remove-other-history" onclick={() => onRemoveDirectoryHistory('/other/backup')}>remove other history</button>
  <button type="button" data-backup-action="open-directory" onclick={onOpenDirectory}>open directory</button>
  <button type="button" data-backup-action="scope" onclick={() => onSelectScope('second@example.test')}>scope</button>
  <button type="button" data-backup-action="scope-all" onclick={() => onSelectScope('')}>scope all</button>
  <button type="button" data-backup-action="toggle-menu" onclick={() => directoryMenuOpen = !directoryMenuOpen}>toggle menu</button>
  <button type="button" data-backup-action="scroll" onclick={onScrollToTop}>scroll</button>
  <button type="button" data-backup-action="clear" onclick={onClearDirectory}>clear</button>
  <button type="button" data-backup-action="refresh" onclick={onRefreshCatalog}>refresh</button>
  <button type="button" data-backup-action="search" onclick={onOpenSearch}>search</button>
  <button type="button" data-backup-action="build" onclick={onBuildIndex}>build</button>
  <button type="button" data-backup-action="sort" onclick={onToggleSortOrder}>sort</button>
  <button type="button" data-backup-action="close" onclick={onClose}>close</button>
</header>
