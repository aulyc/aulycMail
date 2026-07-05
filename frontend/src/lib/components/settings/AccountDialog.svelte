<script lang="ts">
  import * as Dialog from '$lib/components/ui/dialog'
  import AccountForm from './AccountForm.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { addToast } from '$lib/stores/toast'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { account } from '../../../../wailsjs/go/models'

  interface Props {
    /** Whether the dialog is open */
    open?: boolean
    /** Account to edit (null for new account) */
    editAccount?: account.Account | null
    /** Callback when dialog should close */
    onClose?: () => void
    /** Callback when account is successfully created/updated */
    onSuccess?: (account: account.Account) => void
  }

  let {
    open = $bindable(false),
    editAccount = null,
    onClose,
    onSuccess,
  }: Props = $props()

  let createdAccount = $state<account.Account | null>(null)
  const activeAccount = $derived(createdAccount ?? editAccount)

  // Reset create-in-dialog state when the dialog closes. If a new account was
  // created, it remains in the store; this only resets the next dialog opening.
  $effect(() => {
    if (!open) {
      createdAccount = null
    }
  })

  // Activate the dialog guard while open: suppresses background refreshes
  // and routes global keyboard shortcuts (e.g. Ctrl+A) to the dialog inputs
  // instead of the message list / viewer behind it.
  $effect(() => {
    if (open) {
      dialogGuardOpen()
      return () => dialogGuardClose()
    }
  })

  async function handleSubmit(config: account.AccountConfig) {
    if (activeAccount) {
      const result = await accountStore.updateAccount(activeAccount.id, config)
      if (createdAccount?.id === result.id) {
        createdAccount = result
      }

      addToast({
        type: 'success',
        message: $_('toast.accountSaved'),
      })

      onSuccess?.(result)
      open = false
      onClose?.()
      return
    }

    const result = await accountStore.addAccount(config)
    createdAccount = result
    onSuccess?.(result)
    addToast({
      type: 'success',
      message: $_('toast.accountCreated'),
    })
  }

  function handleCancel() {
    open = false
    onClose?.()
  }

  function handleOpenChange(isOpen: boolean) {
    open = isOpen
    if (!isOpen) {
      onClose?.()
    }
  }
</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
  <Dialog.Content class="max-w-xl max-h-[90vh] overflow-hidden flex flex-col" preventCloseAutoFocus onInteractOutside={(e) => e.preventDefault()}>
    <Dialog.Header>
      <Dialog.Title>
        {activeAccount?.sharedMailboxParentId ? $_('account.editSharedMailboxTitle') : activeAccount ? $_('account.editTitle') : $_('account.addTitle')}
      </Dialog.Title>
    </Dialog.Header>

    {#if open}
      <AccountForm
        editAccount={activeAccount}
        createdInDialog={createdAccount !== null}
        onSubmit={handleSubmit}
        onCancel={handleCancel}
      />
    {/if}
  </Dialog.Content>
</Dialog.Root>
