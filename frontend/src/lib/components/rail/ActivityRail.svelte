<script lang="ts">
  import Icon from '@iconify/svelte'
  import { untrack } from 'svelte'
  import RailButton from './RailButton.svelte'
  import { getActivePane, setActivePane } from '$lib/stores/uiState.svelte'
  import { BUILT_IN_RAIL_PANES } from '$lib/rail/panes'
  import { getFocusedPane, isMainKeyboardScope, setFocusedPane } from '$lib/stores/keyboard.svelte'
  import { _ } from '$lib/i18n'

  interface Props {
    // Opens the app Settings dialog. Wired by App.svelte so the gear works
    // from every view (Mail + Contacts).
    onOpenSettings?: () => void
  }

  const { onOpenSettings }: Props = $props()

  // Mail is always present and always first; Contacts is a fixed built-in pane.
  // The rail always renders because it also hosts global Settings.
  let active = $derived(getActivePane())
  let selectedFeature = $state('mail')
  let railEl = $state<HTMLElement | null>(null)

  $effect(() => {
    const activePane = active
    const mainKeyboardScope = isMainKeyboardScope()
    const focusedPane = getFocusedPane()
    if (!mainKeyboardScope) return
    if (focusedPane !== 'featureNav' || untrack(() => selectedFeature) !== 'settings') {
      selectedFeature = activePane
    }
  })

  function select(name: string) {
    selectedFeature = name
    setFocusedPane('featureNav')
    setActivePane(name)
    railEl?.focus({ preventScroll: true })
  }

  function selectSettings() {
    selectedFeature = 'settings'
    setFocusedPane('featureNav')
    onOpenSettings?.()
  }

  function moveSelection(delta: number) {
    const ids = ['mail', ...BUILT_IN_RAIL_PANES.map((pane) => pane.id), 'settings']
    const currentIndex = Math.max(0, ids.indexOf(selectedFeature))
    const nextFeature = ids[(currentIndex + delta + ids.length) % ids.length]
    activateSelection(nextFeature)
  }

  function activateSelection(feature: string) {
    if (feature === 'settings') selectSettings()
    else select(feature)
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.isComposing || event.keyCode === 229) return
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      event.stopPropagation()
      moveSelection(event.key === 'ArrowDown' ? 1 : -1)
      return
    }
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      event.stopPropagation()
      activateSelection(selectedFeature)
    }
  }

  function claimRegion() {
    setFocusedPane('featureNav')
  }

  export function selectSettingsEntry() {
    selectedFeature = 'settings'
  }

  export function focusSettings() {
    selectedFeature = 'settings'
    setFocusedPane('featureNav')
    railEl?.focus({ preventScroll: true })
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<nav
  bind:this={railEl}
  data-keyboard-region="featureNav"
  data-keyboard-region-visible="true"
  data-keyboard-region-focus-target
  data-region-active={isMainKeyboardScope() && getFocusedPane() === 'featureNav'}
  tabindex="0"
  class="keyboard-region flex flex-col items-stretch w-12 flex-shrink-0 bg-muted/30 border-r border-border pt-2 outline-none"
  aria-label="Active rail pane"
  onkeydown={handleKeydown}
  onfocusin={claimRegion}
  onmousedown={claimRegion}
>
  <RailButton
    icon="mdi:email"
    label="Mail"
    active={selectedFeature === 'mail'}
    onclick={() => select('mail')}
  />
  {#each BUILT_IN_RAIL_PANES as pane (pane.id)}
    <RailButton
      icon={pane.icon}
      label={$_(pane.labelKey)}
      active={selectedFeature === pane.id}
      onclick={() => select(pane.id)}
    />
  {/each}

  <!-- Settings: pinned to the bottom, available from Mail and Contacts. -->
  <button
    tabindex="-1"
    class="mt-auto mb-2 flex items-center justify-center w-12 h-12 border-l-[3px] transition-colors duration-150 cursor-pointer focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary focus-visible:-outline-offset-2 {selectedFeature === 'settings' ? 'border-l-primary bg-accent/40 text-primary' : 'border-l-transparent text-muted-foreground hover:text-foreground hover:bg-accent/30'}"
    type="button"
    title={$_('sidebar.settings')}
    aria-label={$_('sidebar.settings')}
    onclick={selectSettings}
  >
    <Icon icon="mdi:cog" width="22" height="22" />
  </button>
</nav>
