<script lang="ts" generics="T extends { id: string }">
  // SourceSidebar — sectioned left sidebar primitive for Contacts-style panes.
  // Owns keyboard navigation (Up/Down/J/K within sidebar) via tabindex+focus.
  // Selection state managed by consumer via selectedId + onSelect.
  //
  // Pane-focus store integration mirrors ListPane: registers focusSlot, takes
  // DOM focus when the slot matches so Alt+H/L cycling routes here.
  //
  // Visual chrome (container styling + responsive overlay + back button +
  // title) is delegated to kit `SidebarFrame`.

  import { type Snippet, onMount } from 'svelte'
  import SidebarFrame from './SidebarFrame.svelte'
  import { KEY } from '$lib/keyboard/shortcuts'
  import { nextRovingIndex } from '$lib/keyboard/regionNavigation'
  import { setFocusedPane, getFocusedPane, isMainKeyboardScope, registerPaneNav, type FocusablePane } from '$lib/stores/keyboard.svelte'

  type SourceSection<U extends { id: string }> = {
    heading?: string
    items: U[]
  }

  interface Props {
    title?: string
    /** Optional custom title content forwarded to SidebarFrame. */
    titleContent?: Snippet
    /** Optional action(s) pinned to the right of the title — forwarded to
     *  SidebarFrame's titleAction slot (e.g. a refresh button). */
    titleAction?: Snippet
    /** Treat marked title buttons as one horizontal item in the sidebar's
     *  vertical keyboard order. */
    headerActionsFocused?: boolean
    selectedHeaderActionId?: string
    sections: SourceSection<T>[]
    selectedId: string | null
    focusSlot?: FocusablePane
    label?: string
    item: Snippet<[T, { active: boolean }]>
    header?: Snippet
    sectionEmpty?: Snippet<[SourceSection<T>]>
    /** Optional sticky bottom strip — forwarded to SidebarFrame's footer
     *  slot. Consumers typically render kit `SidebarFooter` here for the
     *  shared sync/settings chrome. */
    footerContent?: Snippet
    onSelect: (id: string) => void
  }

  let {
    title,
    titleContent,
    titleAction,
    headerActionsFocused = $bindable(false),
    selectedHeaderActionId = $bindable(''),
    sections,
    selectedId,
    focusSlot = 'sidebar',
    label,
    item,
    header,
    sectionEmpty,
    footerContent,
    onSelect,
  }: Props = $props()

  let containerRef = $state<HTMLElement | null>(null)

  const allItems = $derived(sections.flatMap(s => s.items))
  const HEADER_ACTION_SELECTOR = '[data-source-sidebar-header-action]'

  $effect(() => {
    if (getFocusedPane() === focusSlot && containerRef && document.activeElement !== containerRef) {
      containerRef.focus()
    }
  })

  function indexOf(id: string | null): number {
    if (id == null) return -1
    return allItems.findIndex(it => it.id === id)
  }

  function getHeaderActions(): HTMLElement[] {
    if (!containerRef) return []
    return Array.from(containerRef.querySelectorAll<HTMLElement>(HEADER_ACTION_SELECTOR))
      .filter((action) => !action.hidden && action.getAttribute('aria-hidden') !== 'true' && !action.matches(':disabled'))
  }

  function focusHeaderActions(): boolean {
    const actions = getHeaderActions()
    if (actions.length === 0) return false
    const selected = actions.find(action => action.dataset.sourceSidebarHeaderAction === selectedHeaderActionId)
      ?? actions[0]
    headerActionsFocused = true
    selectedHeaderActionId = selected.dataset.sourceSidebarHeaderAction ?? ''
    return true
  }

  function moveHeaderAction(step: 1 | -1) {
    const actions = getHeaderActions()
    if (actions.length === 0) return
    const currentIndex = actions.findIndex(action => (
      action.dataset.sourceSidebarHeaderAction === selectedHeaderActionId
    ))
    const nextIndex = nextRovingIndex(
      step === 1 ? 'ArrowDown' : 'ArrowUp',
      currentIndex,
      actions.length,
      true,
    )
    const nextAction = actions[nextIndex]
    if (nextAction) selectedHeaderActionId = nextAction.dataset.sourceSidebarHeaderAction ?? ''
  }

  function activateHeaderAction() {
    const action = getHeaderActions().find(item => (
      item.dataset.sourceSidebarHeaderAction === selectedHeaderActionId
    ))
    action?.click()
  }

  function activateCurrent() {
    if (headerActionsFocused) {
      activateHeaderAction()
      return
    }
    if (selectedId !== null) onSelect(selectedId)
  }

  function move(step: 1 | -1) {
    const hasHeaderActions = getHeaderActions().length > 0
    const itemCount = allItems.length + (hasHeaderActions ? 1 : 0)
    if (itemCount === 0) return

    const itemIndex = indexOf(selectedId)
    const currentIndex = hasHeaderActions && (headerActionsFocused || itemIndex < 0)
      ? 0
      : itemIndex + (hasHeaderActions ? 1 : 0)
    const nextIndex = nextRovingIndex(
      step === 1 ? 'ArrowDown' : 'ArrowUp',
      currentIndex,
      itemCount,
      true,
    )

    if (hasHeaderActions && nextIndex === 0) {
      focusHeaderActions()
      return
    }

    headerActionsFocused = false
    const nextItem = allItems[nextIndex - (hasHeaderActions ? 1 : 0)]
    if (nextItem) onSelect(nextItem.id)
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.isComposing || e.keyCode === 229) return
    if (
      headerActionsFocused
      && (e.key === 'ArrowLeft' || e.key === 'ArrowRight')
      && !e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey
    ) {
      e.preventDefault()
      e.stopPropagation()
      moveHeaderAction(e.key === 'ArrowRight' ? 1 : -1)
      return
    }
    if (KEY.LIST_NEXT(e)) {
      e.preventDefault()
      e.stopPropagation()
      move(1)
      return
    }
    if (KEY.LIST_PREV(e)) {
      e.preventDefault()
      e.stopPropagation()
      move(-1)
      return
    }
    if (KEY.LIST_OPEN(e) || (e.key === ' ' && !e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey)) {
      if (e.target !== containerRef) return
      if (!headerActionsFocused && selectedId === null) return
      e.preventDefault()
      e.stopPropagation()
      activateCurrent()
      return
    }
  }

  function handleFocus() {
    if (getFocusedPane() !== focusSlot) {
      setFocusedPane(focusSlot)
    }
  }

  function handleMouseDown(e: MouseEvent) {
    if (e.button === 0 && e.target instanceof Element) {
      const headerAction = e.target.closest<HTMLElement>(HEADER_ACTION_SELECTOR)
      if (headerAction && containerRef?.contains(headerAction)) {
        e.preventDefault()
        headerActionsFocused = true
        selectedHeaderActionId = headerAction.dataset.sourceSidebarHeaderAction ?? ''
      } else {
        headerActionsFocused = false
      }
    }
    if (containerRef && document.activeElement !== containerRef) {
      containerRef.focus()
    }
  }

  // Register so Alt+J/K dispatched from the global handler routes here.
  onMount(() => registerPaneNav(focusSlot, {
    navigateNext: () => move(1),
    navigatePrev: () => move(-1),
    activate: activateCurrent,
  }))
</script>

<SidebarFrame
  {title}
  {titleContent}
  {titleAction}
  {label}
  bind:containerRef
  focusable
  keyboardRegion={focusSlot}
  regionActive={isMainKeyboardScope() && getFocusedPane() === focusSlot}
  onkeydown={handleKeyDown}
  onfocus={handleFocus}
  onmousedown={handleMouseDown}
>
  {#snippet body()}
    <div class="py-2">
      {#if header}
        {@render header()}
      {/if}

      {#each sections as section, sIdx (sIdx)}
        {#if section.heading}
          <div class="mx-4 mt-3 mb-1 text-[11px] uppercase tracking-wider text-muted-foreground">
            {section.heading}
          </div>
        {/if}

        {#if section.items.length === 0}
          {#if sectionEmpty}
            {@render sectionEmpty(section)}
          {/if}
        {:else}
          {#each section.items as it (it.id)}
            {@render item(it, { active: !headerActionsFocused && it.id === selectedId })}
          {/each}
        {/if}
      {/each}
    </div>
  {/snippet}

  {#snippet footer()}
    {#if footerContent}
      {@render footerContent()}
    {/if}
  {/snippet}
</SidebarFrame>
