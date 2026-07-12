<script lang="ts">
  import * as Dialog from '$lib/components/ui/dialog'
  import { Button } from '$lib/components/ui/button'
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
</script>

<Dialog.Root bind:open>
  <Dialog.Content class="max-h-[78vh] w-[min(680px,90vw)] max-w-none grid-rows-[auto_minmax(0,1fr)_auto] gap-0 overflow-hidden p-0">
    <header class="shrink-0 border-b border-border px-6 py-5 pr-12">
      <Dialog.Title class="text-lg font-semibold text-foreground">{title}</Dialog.Title>
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
      <Button variant="outline" onclick={() => open = false}>{$_('common.close')}</Button>
    </footer>
  </Dialog.Content>
</Dialog.Root>
