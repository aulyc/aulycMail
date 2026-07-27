<script lang="ts">
  import { tick } from 'svelte'
  import Icon from '@iconify/svelte'
  import ModalFrame from '$lib/components/ui/ModalFrame.svelte'
  import {
    keyboardActionMenu,
    type KeyboardActionTarget,
  } from '$lib/stores/keyboardActionMenu.svelte'
  import { dialogGuardClose, dialogGuardOpen } from '$lib/stores/dialogGuard'
  import { _ } from '$lib/i18n'

  let query = $state('')
  let selectedIndex = $state(0)
  let searchInput = $state<HTMLInputElement | null>(null)
  let actionList = $state<HTMLDivElement | null>(null)

  const filteredActions = $derived.by(() => {
    const normalized = query.trim().toLocaleLowerCase()
    if (!normalized) return keyboardActionMenu.actions
    return keyboardActionMenu.actions.filter((action) =>
      action.label.toLocaleLowerCase().includes(normalized),
    )
  })

  $effect(() => {
    if (!keyboardActionMenu.open) {
      query = ''
      selectedIndex = 0
      return
    }
    dialogGuardOpen()
    selectedIndex = 0
    void tick().then(() => requestAnimationFrame(() => searchInput?.focus({ preventScroll: true })))
    return () => dialogGuardClose()
  })

  $effect(() => {
    void filteredActions.length
    selectedIndex = Math.max(0, Math.min(selectedIndex, filteredActions.length - 1))
  })

  $effect(() => {
    const index = selectedIndex
    if (!keyboardActionMenu.open) return
    void tick().then(() => {
      actionList
        ?.querySelector<HTMLElement>(`[data-keyboard-action-index="${index}"]`)
        ?.scrollIntoView({ block: 'nearest' })
    })
  })

  function moveSelection(delta: -1 | 1) {
    if (filteredActions.length === 0) return
    selectedIndex = (selectedIndex + delta + filteredActions.length) % filteredActions.length
  }

  function activate(action: KeyboardActionTarget | undefined) {
    if (action) keyboardActionMenu.activate(action)
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.isComposing || event.keyCode === 229) return
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      event.stopPropagation()
      moveSelection(1)
      return
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      event.stopPropagation()
      moveSelection(-1)
      return
    }
    if (event.key === 'Enter') {
      event.preventDefault()
      event.stopPropagation()
      activate(filteredActions[selectedIndex])
    }
  }
</script>

<ModalFrame
  open={keyboardActionMenu.open}
  onClose={() => keyboardActionMenu.close()}
  panelClass="w-[min(92vw,520px)] overflow-hidden rounded-xl border border-border bg-popover shadow-2xl"
  labelledBy="keyboard-action-menu-title"
>
  <div data-keyboard-action-menu>
    <div class="flex items-center gap-2 border-b border-border px-4 py-3">
      <Icon icon="mdi:keyboard-outline" class="h-5 w-5 shrink-0 text-muted-foreground" />
      <h2 id="keyboard-action-menu-title" class="text-sm font-semibold text-foreground">
        {$_('keyboardActions.title')}
      </h2>
      <button
        type="button"
        class="ml-auto rounded p-1 hover:bg-muted"
        aria-label={$_('common.close')}
        onclick={() => keyboardActionMenu.close()}
      >
        <Icon icon="mdi:close" class="h-5 w-5 text-muted-foreground" />
      </button>
    </div>

    <div class="border-b border-border px-4 py-3">
      <div class="flex items-center gap-2 rounded-md border border-input bg-background px-3">
        <Icon icon="mdi:magnify" class="h-4 w-4 shrink-0 text-muted-foreground" />
        <input
          bind:this={searchInput}
          bind:value={query}
          class="min-w-0 flex-1 bg-transparent py-2 text-sm text-foreground outline-none"
          placeholder={$_('keyboardActions.searchPlaceholder')}
          onkeydown={handleKeydown}
        />
      </div>
    </div>

    <div bind:this={actionList} class="max-h-[min(56vh,440px)] overflow-y-auto py-1 scrollbar-thin">
      {#if filteredActions.length === 0}
        <p class="px-4 py-8 text-center text-sm text-muted-foreground">{$_('keyboardActions.empty')}</p>
      {:else}
        {#each filteredActions as action, index (action.id)}
          <button
            type="button"
            data-keyboard-action-index={index}
            class="flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm {index === selectedIndex ? 'bg-muted text-foreground' : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground'}"
            onmouseenter={() => { selectedIndex = index }}
            onclick={() => activate(action)}
          >
            <Icon icon="mdi:chevron-right" class="h-4 w-4 shrink-0" />
            <span class="min-w-0 flex-1 truncate">{action.label}</span>
          </button>
        {/each}
      {/if}
    </div>

    <div class="border-t border-border px-4 py-2 text-xs text-muted-foreground">
      {$_('keyboardActions.hint')}
    </div>
  </div>
</ModalFrame>
