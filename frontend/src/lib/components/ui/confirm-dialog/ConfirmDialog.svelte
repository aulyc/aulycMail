<script lang="ts">
  import * as AlertDialog from '$lib/components/ui/alert-dialog'
  import Icon from '@iconify/svelte'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import { cn } from '$lib/utils'

  interface Props {
    open: boolean                    // bindable
    title: string
    description: string
    confirmLabel?: string            // default: "Confirm"
    cancelLabel?: string             // default: "Cancel"
    variant?: 'default' | 'destructive'  // default: 'default'
    loading?: boolean                // show spinner on confirm button
    loadingPresentation?: 'button' | 'footer-progress'
    loadingLabel?: string
    loadingProgress?: number
    onConfirm: () => void | Promise<void>
    onCancel?: () => void
  }

  let {
    open = $bindable(false),
    title,
    description,
    confirmLabel = 'Confirm',
    cancelLabel = 'Cancel',
    variant = 'default',
    loading = false,
    loadingPresentation = 'button',
    loadingLabel = '',
    loadingProgress = 0,
    onConfirm,
    onCancel,
  }: Props = $props()

  const showFooterProgress = $derived(loading && loadingPresentation === 'footer-progress')
  const normalizedLoadingProgress = $derived(Math.max(0, Math.min(100, Math.round(loadingProgress))))

  let closedByButton = false

  // Register/unregister with the dialog guard whenever the open state flips.
  // The guard is what makes App.svelte's global key handler step out of the
  // way — without it, mail's Enter/Space dispatcher calls e.preventDefault()
  // on dialog buttons (they're not inside a `[data-pane="..."]` ancestor) and
  // the user can't activate Confirm/Cancel via keyboard.
  let guarded = false
  $effect(() => {
    if (open && !guarded) {
      dialogGuardOpen()
      guarded = true
    }
    if (!open && guarded) {
      dialogGuardClose()
      guarded = false
    }
  })

  function handleOpenChange(isOpen: boolean) {
    open = isOpen
    if (!isOpen) {
      if (!closedByButton) {
        onCancel?.()
      }
      closedByButton = false
    }
  }

  async function handleConfirm() {
    closedByButton = true
    try {
      await onConfirm()
    } finally {
      open = false
    }
  }

  function handleCancel() {
    closedByButton = true
    onCancel?.()
    open = false
  }
</script>

<AlertDialog.Root bind:open onOpenChange={handleOpenChange}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{title}</AlertDialog.Title>
      {#if description}
        <AlertDialog.Description>{description}</AlertDialog.Description>
      {/if}
    </AlertDialog.Header>

    <AlertDialog.Footer class={loadingPresentation === 'footer-progress' ? 'sm:items-center' : ''}>
      {#if showFooterProgress}
        <div data-confirm-progress class="w-full min-w-0 pb-3 sm:mr-auto sm:w-52 sm:pb-0" aria-live="polite">
          <div class="mb-1.5 flex items-center justify-between gap-3 text-xs text-muted-foreground">
            <span data-confirm-progress-label class="truncate">{loadingLabel}</span>
            <span data-confirm-progress-percent class="shrink-0 tabular-nums">{normalizedLoadingProgress}%</span>
          </div>
          <div
            role="progressbar"
            aria-label={loadingLabel}
            aria-valuemin="0"
            aria-valuemax="100"
            aria-valuenow={normalizedLoadingProgress}
            class="h-1.5 overflow-hidden rounded-full bg-muted"
          >
            <div
              data-confirm-progress-fill
              class="h-full rounded-full bg-primary transition-[width] duration-200 ease-out"
              style:width={`${normalizedLoadingProgress}%`}
            ></div>
          </div>
        </div>
      {/if}
      <AlertDialog.Cancel
        onclick={handleCancel}
        disabled={loading}
        class={loadingPresentation === 'footer-progress' ? 'shrink-0' : ''}
      >
        {cancelLabel}
      </AlertDialog.Cancel>
      <AlertDialog.Action
        onclick={handleConfirm}
        disabled={loading}
        class={cn(
          variant === 'destructive' ? 'bg-destructive text-destructive-foreground hover:bg-destructive/90' : '',
          loadingPresentation === 'footer-progress' && 'w-40 shrink-0 justify-center',
        )}
      >
        {#if loading && loadingPresentation === 'button'}
          <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
        {/if}
        {confirmLabel}
      </AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
