<script lang="ts">
  import { onDestroy } from 'svelte'
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import { _ } from '$lib/i18n'
  import BackupTab from '$lib/components/settings/BackupTab.svelte'

  interface Props {
    open?: boolean
    onClose?: () => void
  }

  let { open = $bindable(false), onClose }: Props = $props()
  let guardActive = false

  function setGuardActive(active: boolean) {
    if (active === guardActive) return
    guardActive = active
    if (active) {
      dialogGuardOpen()
    } else {
      dialogGuardClose()
    }
  }

  $effect(() => {
    setGuardActive(open)
  })

  $effect(() => {
    if (!open) return

    function handleKeydown(event: KeyboardEvent) {
      if (event.key !== 'Escape') return
      event.preventDefault()
      event.stopPropagation()
      closeDialog()
    }

    window.addEventListener('keydown', handleKeydown, true)
    return () => window.removeEventListener('keydown', handleKeydown, true)
  })

  onDestroy(() => setGuardActive(false))

  function closeDialog() {
    if (!open) return
    open = false
    setGuardActive(false)
    onClose?.()
  }
</script>

{#if open}
  <div class="fixed inset-0 z-[90] flex items-center justify-center bg-black/80 p-4">
    <div
      class="flex max-h-[85vh] w-[min(92vw,760px)] max-w-2xl flex-col overflow-hidden rounded-lg border bg-background p-6 shadow-lg"
      role="dialog"
      aria-modal="true"
      aria-labelledby="backup-dialog-title"
    >
      <div class="mb-4 flex items-center gap-3">
        <Icon icon="lucide:archive" class="h-5 w-5 shrink-0" />
        <h2 id="backup-dialog-title" class="min-w-0 flex-1 text-lg font-semibold leading-none">
          {$_('settingsBackup.title')}
        </h2>
        <button
          type="button"
          class="shrink-0 rounded-sm p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={$_('common.close')}
          onclick={closeDialog}
        >
          <Icon icon="mdi:close" class="h-4 w-4" />
        </button>
      </div>

      <div class="h-[min(64vh,430px)] min-h-0 overflow-y-auto pl-1 pr-3">
        <BackupTab />
      </div>

      <div class="flex items-center justify-end gap-2 border-t border-border pt-4">
        <Button variant="ghost" onclick={closeDialog}>
          {$_('common.close')}
        </Button>
      </div>
    </div>
  </div>
{/if}
