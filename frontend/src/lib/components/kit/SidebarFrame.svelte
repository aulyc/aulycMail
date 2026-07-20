<script lang="ts">
  // SidebarFrame — kit primitive owning sidebar chrome for Contacts-style
  // panes. Captures container styling, responsive overlay behavior (slide-in +
  // back button in narrow mode), optional title, scrollable body slot, and
  // optional sticky footer slot.
  //
  // Why this primitive exists: Contacts uses the same responsive sidebar
  // mechanics as Mail while keeping the pane implementation compact.
  //
  // Layout model: title stays non-scrolling at the top; body fills the
  // remaining vertical space with its own overflow-y-auto; optional footer
  // stays pinned at the bottom.

  import { type Snippet } from 'svelte'
  import Icon from '@iconify/svelte'
  import { _ } from 'svelte-i18n'
  import { getLayoutMode, getResponsiveView, hideSidebar } from '$lib/stores/layout.svelte'
  import { getUIState, getUIStateVersion } from '$lib/stores/uiState.svelte'

  interface Props {
    /** Optional title rendered as <h2>. Omit for sidebars with no title. */
    title?: string
    /** Optional replacement for the title text. Keeps the same pinned title row
     *  chrome while allowing a consumer to render a button-like title. */
    titleContent?: Snippet
    /** Optional action(s) pinned to the right of the title row (e.g. a
     *  refresh button). Only rendered when `title` is set. */
    titleAction?: Snippet
    /** ARIA label for the <aside>. Defaults to `title`. */
    label?: string
    /** The scrollable body content. Required. */
    body: Snippet
    /** Optional sticky bottom strip. Consumer owns the strip's chrome
     *  (border-t, padding, content); SidebarFrame just pins it with shrink-0. */
    footer?: Snippet
    /** Bindable ref to the outer <aside>. SourceSidebar binds this for its
     *  tabindex-based keyboard nav + focus-slot integration. */
    containerRef?: HTMLElement | null
    /** When true, the <aside> gets tabindex="0" so it can take DOM focus.
     *  Default false. */
    focusable?: boolean
    /** Extra class string appended to the outer <aside>. Used by SourceSidebar
     *  for its pane-focus-flash indicator. */
    class?: string
    /** Optional DOM event handlers forwarded to the outer <aside>. */
    onkeydown?: (e: KeyboardEvent) => void
    onfocus?: () => void
    onmousedown?: (e: MouseEvent) => void
    keyboardRegion?: string
    regionActive?: boolean
  }

  let {
    title,
    titleContent,
    titleAction,
    label,
    body,
    footer,
    containerRef = $bindable(null),
    focusable = false,
    class: extraClass = '',
    onkeydown,
    onfocus,
    onmousedown,
    keyboardRegion,
    regionActive = false,
  }: Props = $props()

  const narrow = $derived(getLayoutMode() === 'narrow')
  const overlayVisible = $derived(narrow && getResponsiveView() === 'sidebar')
  // Match the mail folder sidebar's fixed width so the Contacts view lines up
  // with the mail view.
  const paneWidth = $derived.by(() => { getUIStateVersion(); return getUIState().sidebarWidth })
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<aside
  bind:this={containerRef}
  data-keyboard-region={keyboardRegion}
  data-keyboard-region-visible={!narrow || overlayVisible}
  data-keyboard-region-focus-target={keyboardRegion ? '' : undefined}
  data-region-active={keyboardRegion ? regionActive : undefined}
  role="navigation"
  aria-label={label ?? title ?? 'Sidebar'}
  tabindex={focusable ? 0 : undefined}
  class="{keyboardRegion ? 'keyboard-region' : ''} flex-shrink-0 flex flex-col {title ? '' : 'pt-3'} border-r border-border outline-none {narrow ? 'bg-background' : 'bg-muted/30'} {narrow ? 'responsive-sidebar-overlay' : ''} {overlayVisible ? 'responsive-sidebar-visible' : ''} {extraClass}"
  style="width: {paneWidth}px"
  {onkeydown}
  {onfocus}
  {onmousedown}
>
  {#if narrow}
    <button
      type="button"
      class="flex items-center gap-2 px-4 py-2 mb-2 text-sm text-muted-foreground hover:text-foreground"
      onclick={hideSidebar}
      aria-label={$_('common.back')}
    >
      <Icon icon="mdi:arrow-left" class="w-4 h-4" />
      <span>{$_('common.back')}</span>
    </button>
  {/if}

  {#if title || titleContent}
    <div class="px-4 py-3 border-b border-border flex items-center justify-between gap-2 min-h-[61px]">
      {#if titleContent}
        {@render titleContent()}
      {:else}
        <h2 class="text-lg font-semibold text-foreground">{title}</h2>
      {/if}
      {#if titleAction}
        {@render titleAction()}
      {/if}
    </div>
  {/if}

  <div class="flex-1 min-h-0 overflow-y-auto">
    {@render body()}
  </div>

  {#if footer}
    <div class="shrink-0">
      {@render footer()}
    </div>
  {/if}
</aside>
