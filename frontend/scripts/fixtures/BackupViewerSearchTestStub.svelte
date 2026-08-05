<script lang="ts">
  let {
    open,
    accountScopes,
    searchScopeEmail,
    searchQuery = $bindable(''),
    searchResults,
    searchLoading,
    searchActiveIndex = $bindable(0),
    searchInputEl = $bindable(),
    onClose,
    onSelectSearchScope,
    onSearchInput,
    onCompositionStart,
    onCompositionEnd,
    onSearchKeydown,
    onSelectSearchResult,
    messageAttachmentCount,
    formatShortDate,
  } = $props()
</script>

{#if open}
  <section data-backup-viewer-search data-scope={searchScopeEmail} data-loading={searchLoading} data-active={searchActiveIndex}>
    <input
      bind:this={searchInputEl}
      bind:value={searchQuery}
      data-backup-search-input
      oninput={onSearchInput}
      oncompositionstart={onCompositionStart}
      oncompositionend={onCompositionEnd}
      onkeydown={onSearchKeydown}
    />
    <button type="button" data-backup-search-action="close" onclick={onClose}>close</button>
    {#each accountScopes as scope (scope.id)}
      <button type="button" data-backup-search-scope={scope.id} onclick={() => onSelectSearchScope(scope.id)}>{scope.label}</button>
    {/each}
    {#each searchResults as result, index (result.key)}
      <button type="button" data-backup-search-result={result.key} onclick={() => onSelectSearchResult(index)}>
        {result.subject} {messageAttachmentCount(result)} {formatShortDate(result.date)}
      </button>
    {/each}
  </section>
{/if}
