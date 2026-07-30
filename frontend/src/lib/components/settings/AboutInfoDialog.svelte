<script lang="ts">
  import Icon from '@iconify/svelte'
  import * as Dialog from '$lib/components/ui/dialog'
  import { _ } from '$lib/i18n'

  interface InfoSection {
    title: string
    body?: string
    items?: string[]
  }

  interface Props {
    open?: boolean
    title: string
    intro: string
    sections: InfoSection[]
  }

  let { open = $bindable(false), title, intro, sections }: Props = $props()
  let primaryAction = $state<HTMLButtonElement | null>(null)

  function close() {
    open = false
  }

  function handleOpenAutoFocus(event: Event) {
    event.preventDefault()
    requestAnimationFrame(() => {
      if (open) primaryAction?.focus({ preventScroll: true })
    })
  }
</script>

<Dialog.Root bind:open>
  <Dialog.Content
    class="max-h-[78vh] w-[min(680px,90vw)] max-w-none grid-rows-[auto_minmax(0,1fr)_auto] gap-0 overflow-hidden p-0 !outline-none focus:!outline-none focus-visible:!outline-none focus:ring-0 focus-visible:ring-0 [&>button]:hidden"
    onOpenAutoFocus={handleOpenAutoFocus}
  >
    <header class="shrink-0 border-b border-border px-6 py-5 pr-12">
      <Dialog.Title class="text-lg font-semibold text-foreground">{title}</Dialog.Title>
      <button
        type="button"
        class="absolute right-4 top-4 rounded-sm p-0.5 text-muted-foreground opacity-70 transition-opacity hover:opacity-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
        aria-label={$_('common.close')}
        onclick={close}
      >
        <Icon icon="mdi:close" class="h-4 w-4" />
      </button>
    </header>

    <div class="min-h-0 flex-1 space-y-6 overflow-y-auto px-6 py-5 scrollbar-thin">
      <p class="whitespace-pre-line text-sm leading-6 text-muted-foreground">{intro}</p>
      {#each sections as section (section.title)}
        <section class="space-y-2">
          <h3 class="text-sm font-semibold text-foreground">{section.title}</h3>
          {#if section.body}<p class="whitespace-pre-line text-sm leading-6 text-muted-foreground">{section.body}</p>{/if}
          {#if section.items?.length}
            <ul class="list-disc space-y-1.5 pl-5 text-sm leading-6 text-muted-foreground">
              {#each section.items as item (item)}<li>{item}</li>{/each}
            </ul>
          {/if}
        </section>
      {/each}
    </div>

    <footer class="flex shrink-0 justify-end border-t border-border px-6 py-4">
      <button
        bind:this={primaryAction}
        type="button"
        class="inline-flex h-10 items-center justify-center whitespace-nowrap rounded-md border border-input bg-background px-4 py-2 text-sm font-medium ring-offset-background transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
        onclick={close}
      >
        {$_('common.close')}
      </button>
    </footer>
  </Dialog.Content>
</Dialog.Root>
