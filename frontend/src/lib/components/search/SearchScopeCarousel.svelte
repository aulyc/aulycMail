<script lang="ts">
  interface SearchScope {
    id: string
    label: string
  }

  interface SearchScopeSlot {
    scope: SearchScope
    key: string
  }

  interface Props {
    scopes?: SearchScope[]
    selectedId?: string
    maxLabelWidthClass?: string
    onSelect?: (scopeID: string) => void
  }

  const SIDE_SLOT_COUNT = 10

  let {
    scopes = [],
    selectedId = '',
    maxLabelWidthClass = 'max-w-[220px]',
    onSelect,
  }: Props = $props()

  const selectedIndex = $derived(Math.max(0, scopes.findIndex((scope) => scope.id === selectedId)))
  const selectedScope = $derived(scopes[selectedIndex])
  const leftSlots = $derived.by(() => buildSideSlots(-1))
  const rightSlots = $derived.by(() => buildSideSlots(1))

  function wrapIndex(index: number, count: number): number {
    return ((index % count) + count) % count
  }

  function buildSideSlots(direction: -1 | 1): SearchScopeSlot[] {
    const count = scopes.length
    if (count <= 1) return []
    const slots: SearchScopeSlot[] = []
    let step = 1
    while (slots.length < SIDE_SLOT_COUNT && step <= SIDE_SLOT_COUNT * count) {
      const sourceIndex = wrapIndex(selectedIndex + direction * step, count)
      if (sourceIndex !== selectedIndex) {
        const scope = scopes[sourceIndex]
        slots.push({
          scope,
          key: `${direction}:${step}:${sourceIndex}:${scope.id || 'all'}`,
        })
      }
      step += 1
    }
    return direction === -1 ? slots.reverse() : slots
  }

  function selectScope(scopeID: string) {
    onSelect?.(scopeID)
  }

  function buttonClass(active: boolean): string {
    const tone = active
      ? 'border-primary/75 bg-primary/15 text-primary'
      : 'border-border/70 bg-transparent text-muted-foreground hover:border-border hover:bg-muted/20 hover:text-foreground'
    return `${maxLabelWidthClass} shrink-0 truncate rounded-md border px-3 py-1.5 text-xs font-semibold shadow-sm backdrop-blur-sm transition-colors ${tone}`
  }
</script>

<div class="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-2 overflow-hidden">
  <div class="flex min-w-0 justify-end gap-2 overflow-hidden">
    {#each leftSlots as item (item.key)}
      <button
        type="button"
        tabindex="-1"
        aria-pressed="false"
        class={buttonClass(false)}
        onmousedown={(event) => event.preventDefault()}
        onclick={() => selectScope(item.scope.id)}
      >
        {item.scope.label}
      </button>
    {/each}
  </div>
  {#if selectedScope}
    <button
      type="button"
      tabindex="-1"
      aria-pressed="true"
      class={buttonClass(true)}
      onmousedown={(event) => event.preventDefault()}
      onclick={() => selectScope(selectedScope.id)}
    >
      {selectedScope.label}
    </button>
  {/if}
  <div class="flex min-w-0 justify-start gap-2 overflow-hidden">
    {#each rightSlots as item (item.key)}
      <button
        type="button"
        tabindex="-1"
        aria-pressed="false"
        class={buttonClass(false)}
        onmousedown={(event) => event.preventDefault()}
        onclick={() => selectScope(item.scope.id)}
      >
        {item.scope.label}
      </button>
    {/each}
  </div>
</div>
