<script lang="ts">
  import ConfirmDialog from '$lib/components/ui/confirm-dialog/ConfirmDialog.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { addToast } from '$lib/stores/toast'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { account } from '../../../../wailsjs/go/models'

  interface Props {
    /** Whether the dialog is open */
    open?: boolean
    /** Account to delete */
    account: account.Account | null
    /** Callback when dialog should close */
    onClose?: () => void
    /** Callback when account is successfully deleted */
    onSuccess?: () => void
  }

  let {
    open = $bindable(false),
    account: accountToDelete = null,
    onClose,
    onSuccess,
  }: Props = $props()

  let deleting = $state(false)

  async function handleDelete() {
    if (!accountToDelete) return

    deleting = true

    try {
      await accountStore.removeAccount(accountToDelete.id)
      onSuccess?.()
      open = false
      onClose?.()
    } catch (err) {
      console.error('Failed to delete account:', err)
      addToast({ type: 'error', message: $_('toast.failedToDelete') })
    } finally {
      deleting = false
    }
  }

  function handleCancel() {
    open = false
    onClose?.()
  }

</script>

<ConfirmDialog
  bind:open
  title={$_('account.deleteTitle')}
  description={`${$_('account.deleteConfirm', { values: { name: accountToDelete?.name ?? '', email: accountToDelete?.email ?? '' } })} ${$_('account.deleteWarning')}`}
  confirmLabel={$_('account.deleteAccount')}
  cancelLabel={$_('common.cancel')}
  variant="destructive"
  loading={deleting}
  onConfirm={handleDelete}
  onCancel={handleCancel}
/>
