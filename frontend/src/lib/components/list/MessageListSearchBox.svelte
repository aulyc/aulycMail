<script lang="ts">
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'

  interface Props {
    showSearch: boolean
    searchQuery: string
    serverSearchMode: boolean
    isSearching: boolean
    isServerSearching: boolean
    searchInputRef: HTMLInputElement | null
    onSearchInput: () => void
    onSearchKeydown: (event: KeyboardEvent) => void
    onClearSearch: () => void
  }

  let {
    showSearch,
    searchQuery = $bindable(''),
    serverSearchMode = $bindable(false),
    isSearching,
    isServerSearching,
    searchInputRef = $bindable(null),
    onSearchInput,
    onSearchKeydown,
    onClearSearch,
  }: Props = $props()
</script>

{#if showSearch}
  <div class="flex items-center gap-1 bg-muted rounded-md px-2 flex-1 min-w-0">
    <Icon icon="mdi:magnify" class="w-4 h-4 text-muted-foreground flex-shrink-0" />
    <input
      bind:this={searchInputRef}
      type="text"
      placeholder={$_('messageList.searchMessages')}
      class="bg-transparent border-none outline-none text-sm py-1.5 w-full min-w-0"
      bind:value={searchQuery}
      oninput={onSearchInput}
      onkeydown={onSearchKeydown}
    />
    {#if serverSearchMode}
      <button
        onclick={() => { serverSearchMode = false }}
        class="px-1.5 py-0.5 text-[10px] font-medium bg-primary/20 text-primary rounded-full flex-shrink-0 hover:bg-primary/30 transition-colors"
        title={$_('search.localSearch')}
      >
        {$_('search.server')}
      </button>
    {/if}
    {#if searchQuery || isSearching || isServerSearching}
      <button
        onclick={onClearSearch}
        class="p-0.5 hover:bg-muted-foreground/20 rounded flex-shrink-0"
        title={$_('messageList.clearSearch')}
      >
        {#if isSearching || isServerSearching}
          <Icon icon="mdi:loading" class="w-4 h-4 animate-spin text-muted-foreground" />
        {:else}
          <Icon icon="mdi:close" class="w-4 h-4 text-muted-foreground" />
        {/if}
      </button>
    {/if}
  </div>
{:else}
  <div class="flex-1"></div>
{/if}
