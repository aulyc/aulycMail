<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import {
    loadBackupDirectoryHistory,
    removeBackupDirectory,
    subscribeBackupDirectoryHistory,
  } from '$lib/utils/backup-directory-history'
  // @ts-ignore - wailsjs path
  import { ChooseBackupDirectory } from '../../../../wailsjs/go/app/App.js'

  interface Props {
    directory?: string
    placeholder?: string
    disabled?: boolean
    openDisabled?: boolean
    menuOpen?: boolean
    onChoose?: (path: string) => void | Promise<void>
    onChooseError?: (error: unknown) => void
    onSelectHistory?: (path: string) => void | Promise<void>
    onRemoveHistory?: (path: string) => void | Promise<void>
    onOpenDirectory?: (path: string) => void | Promise<void>
  }

  let {
    directory = '',
    placeholder = '',
    disabled = false,
    openDisabled = false,
    menuOpen = $bindable(false),
    onChoose,
    onChooseError,
    onSelectHistory,
    onRemoveHistory,
    onOpenDirectory,
  }: Props = $props()

  let history = $state<string[]>([])
  let pickerEl = $state<HTMLDivElement | null>(null)
  let choosing = $state(false)
  let opening = $state(false)

  const label = $derived(directory || placeholder || $_('backupViewer.directoryPlaceholder'))
  const canOpen = $derived(Boolean(directory?.trim()) && !disabled && !openDisabled)

  onMount(() => {
    history = loadBackupDirectoryHistory()
    return subscribeBackupDirectoryHistory((paths) => {
      history = paths
    })
  })

  $effect(() => {
    if (!menuOpen) return

    function handlePointerDown(event: PointerEvent) {
      if (pickerEl?.contains(event.target as Node)) return
      menuOpen = false
    }

    function handleKeydown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        menuOpen = false
      }
    }

    window.addEventListener('pointerdown', handlePointerDown, true)
    window.addEventListener('keydown', handleKeydown, true)
    return () => {
      window.removeEventListener('pointerdown', handlePointerDown, true)
      window.removeEventListener('keydown', handleKeydown, true)
    }
  })

  function setMenuOpen(value: boolean) {
    if (disabled && value) return
    menuOpen = value
  }

  async function chooseNewDirectory() {
    menuOpen = false
    choosing = true
    try {
      const selected = await ChooseBackupDirectory()
      if (!selected) return
      await onChoose?.(selected)
    } catch (err) {
      console.error('Failed to choose backup directory:', err)
      onChooseError?.(err)
    } finally {
      choosing = false
    }
  }

  async function selectHistoryDirectory(path: string) {
    menuOpen = false
    await onSelectHistory?.(path)
  }

  async function removeHistory(event: MouseEvent, path: string) {
    event.preventDefault()
    event.stopPropagation()
    removeBackupDirectory(path)
    await onRemoveHistory?.(path)
  }

  async function openDirectory(event: MouseEvent) {
    event.preventDefault()
    event.stopPropagation()
    const path = directory.trim()
    if (!path || !canOpen || opening) return
    opening = true
    try {
      await onOpenDirectory?.(path)
    } finally {
      opening = false
    }
  }
</script>

<div class="relative min-w-0" bind:this={pickerEl}>
  <button
    type="button"
    class="flex h-10 w-full min-w-0 items-center gap-2 rounded-md border border-border bg-background py-2 pl-11 pr-3 text-left text-sm text-foreground outline-none transition hover:bg-muted/40 focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
    aria-haspopup="listbox"
    aria-expanded={menuOpen}
    title={directory || placeholder || $_('backupViewer.directoryPlaceholder')}
    onclick={() => setMenuOpen(!menuOpen)}
    disabled={disabled}
  >
    <span class="min-w-0 flex-1 truncate {directory ? '' : 'text-muted-foreground'}">{label}</span>
    <Icon icon="mdi:chevron-down" width="18" height="18" class={`shrink-0 text-muted-foreground transition ${menuOpen ? 'rotate-180' : ''}`} />
  </button>

  <button
    type="button"
    class="absolute left-1.5 top-1/2 z-10 inline-flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground transition hover:bg-muted hover:text-foreground disabled:cursor-not-allowed disabled:opacity-35"
    aria-label={$_('backupViewer.openDirectory')}
    title={$_('backupViewer.openDirectory')}
    disabled={!canOpen}
    onclick={openDirectory}
  >
    <Icon icon="mdi:folder-open-outline" width="18" height="18" />
  </button>

  {#if menuOpen}
    <div
      class="absolute left-0 top-[calc(100%+6px)] z-[140] w-full overflow-hidden rounded-md border border-border bg-popover shadow-lg"
      role="listbox"
      aria-label={$_('backupViewer.directoryHistory')}
    >
      <button
        type="button"
        class="flex h-10 w-full items-center gap-2 border-b border-border px-3 text-left text-sm font-semibold text-foreground transition hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
        onclick={chooseNewDirectory}
        disabled={choosing}
      >
        <Icon icon={choosing ? 'mdi:loading' : 'mdi:folder-plus-outline'} width="18" height="18" class={`shrink-0 text-muted-foreground ${choosing ? 'animate-spin' : ''}`} />
        <span class="min-w-0 flex-1 truncate">{$_('backupViewer.loadNewDirectory')}</span>
      </button>

      <div class="h-64 overflow-y-auto py-1 scrollbar-thin">
        {#if history.length === 0}
          <div class="px-3 py-2 text-sm text-muted-foreground">{$_('backupViewer.noDirectoryHistory')}</div>
        {:else}
          {#each history as path (path)}
            <div class="flex items-center gap-1 px-1">
              <button
                type="button"
                class="min-w-0 flex-1 rounded-sm px-2 py-2 text-left text-sm transition hover:bg-muted {directory === path ? 'text-primary' : 'text-foreground'}"
                title={path}
                onclick={() => selectHistoryDirectory(path)}
              >
                <span class="block truncate">{path}</span>
              </button>
              <button
                type="button"
                class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-sm text-muted-foreground transition hover:bg-muted hover:text-foreground"
                aria-label={$_('backupViewer.removeDirectoryHistory')}
                title={$_('backupViewer.removeDirectoryHistory')}
                onclick={(event) => removeHistory(event, path)}
              >
                <Icon icon="mdi:close" width="16" height="16" />
              </button>
            </div>
          {/each}
        {/if}
      </div>
    </div>
  {/if}
</div>
