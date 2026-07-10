<script lang="ts">
  import type { Snippet } from 'svelte'

  interface Props {
    open?: boolean
    onClose?: () => void
    panelClass: string
    backdropClass?: string
    containerClass?: string
    labelledBy?: string
    children?: Snippet
  }

  let {
    open = false,
    onClose,
    panelClass,
    backdropClass = 'bg-black/80',
    containerClass = 'z-[90] flex items-center justify-center p-4',
    labelledBy,
    children,
  }: Props = $props()

  function closeFromBackdrop() {
    onClose?.()
  }
</script>

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="fixed inset-0 {containerClass} {backdropClass}" onclick={closeFromBackdrop}>
    <div
      class={panelClass}
      role="dialog"
      aria-modal="true"
      aria-labelledby={labelledBy}
      tabindex="-1"
      onclick={(event) => event.stopPropagation()}
    >
      {#if children}
        {@render children()}
      {/if}
    </div>
  </div>
{/if}
