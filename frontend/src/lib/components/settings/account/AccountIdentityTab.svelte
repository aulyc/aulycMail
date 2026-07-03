<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import ConfirmDialog from '$lib/components/ui/confirm-dialog/ConfirmDialog.svelte'
  import IdentityEditor from './IdentityEditor.svelte'
  import AccountDialog from '../AccountDialog.svelte'
  import { addToast } from '$lib/stores/toast'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { account } from '../../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import { GetIdentities, CreateIdentity, UpdateIdentity, DeleteIdentity, SetDefaultIdentity, AddMicrosoftSharedMailbox, GetMicrosoftSharedMailboxes, RemoveAccount } from '../../../../../wailsjs/go/app/App'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import * as Dialog from '$lib/components/ui/dialog'

  interface Props {
    /** The account being edited */
    accountId: string
    /** The full account object (for detecting Microsoft OAuth) */
    editAccount?: account.Account
    /** Live display name for the account's default identity */
    defaultDisplayName?: string
    /** Whether the default display name has been loaded from the default identity */
    displayNameLoaded?: boolean
    /** Keep the General tab's display name in sync with the default identity editor */
    onDefaultDisplayNameChange?: (value: string) => void
  }

  let {
    accountId,
    editAccount,
    defaultDisplayName = '',
    displayNameLoaded = false,
    onDefaultDisplayNameChange,
  }: Props = $props()

  // State
  let identities = $state<account.Identity[]>([])
  let loading = $state(true)
  let showEditor = $state(false)
  let editingIdentity = $state<account.Identity | null>(null)
  let identityEditorSaved = $state(false)
  let defaultDisplayNameBeforeEdit = $state<string | null>(null)
  let deletingId = $state<string | null>(null)
  let identityToDelete = $state<account.Identity | null>(null)
  let showDeleteConfirm = $state(false)

  // Shared mailbox state
  let sharedMailboxes = $state<account.Account[]>([])
  let sharedMailboxLoading = $state(false)
  let showAddSharedMailbox = $state(false)
  let sharedMailboxEmail = $state('')
  let sharedMailboxDisplayName = $state('')
  let addingSharedMailbox = $state(false)
  let showSharedMailboxEditor = $state(false)
  let editingSharedMailbox = $state<account.Account | null>(null)

  const isMicrosoft = $derived(
    editAccount?.authType === 'oauth2' &&
    !editAccount?.sharedMailboxParentId &&
    (editAccount?.imapHost === 'outlook.office365.com' || editAccount?.imapHost === 'imap-mail.outlook.com')
  )

  function getSignatureBadge(identity: account.Identity): string {
    if (identity.signatureEnabled === false) return $_('identity.signatureBadgeNone')
    if ((identity.signatureHtml || '').trim()) return $_('identity.signatureBadgeHtml')
    if ((identity.signatureText || '').trim()) return $_('identity.signatureBadgePlain')
    return $_('identity.signatureBadgeNone')
  }

  onMount(async () => {
    await loadIdentities()
    if (isMicrosoft) {
      await loadSharedMailboxes()
    }
  })

  async function loadSharedMailboxes() {
    sharedMailboxLoading = true
    try {
      sharedMailboxes = (await GetMicrosoftSharedMailboxes(accountId)) || []
    } catch (err) {
      console.error('Failed to load shared mailboxes:', err)
    } finally {
      sharedMailboxLoading = false
    }
  }

  async function handleAddSharedMailbox() {
    if (!sharedMailboxEmail.trim()) return
    addingSharedMailbox = true
    try {
      const newAccount = await AddMicrosoftSharedMailbox(accountId, sharedMailboxEmail.trim(), sharedMailboxDisplayName.trim())
      addToast({ type: 'success', message: $_('identity.sharedMailboxAdded') })
      showAddSharedMailbox = false
      sharedMailboxEmail = ''
      sharedMailboxDisplayName = ''
      await loadSharedMailboxes()
      // Refresh sidebar and trigger initial sync
      await accountStore.load()
      accountStore.syncAccount(newAccount.id)
    } catch (err) {
      console.error('Failed to add shared mailbox:', err)
      addToast({ type: 'error', message: $_('identity.failedToAddSharedMailbox') + ': ' + (err as Error).message })
    } finally {
      addingSharedMailbox = false
    }
  }

  async function handleDeleteSharedMailbox(mailbox: account.Account) {
    try {
      await RemoveAccount(mailbox.id)
      addToast({ type: 'success', message: $_('identity.sharedMailboxRemoved') })
      await loadSharedMailboxes()
    } catch (err) {
      console.error('Failed to delete shared mailbox:', err)
      addToast({ type: 'error', message: $_('identity.failedToDeleteSharedMailbox') })
    }
  }

  async function loadIdentities() {
    loading = true
    try {
      identities = await GetIdentities(accountId)
    } catch (err) {
      console.error('Failed to load identities:', err)
      addToast({
        type: 'error',
        message: $_('identity.failedToLoadAddresses'),
      })
    } finally {
      loading = false
    }
  }

  function handleAddIdentity() {
    editingIdentity = null
    identityEditorSaved = false
    defaultDisplayNameBeforeEdit = null
    showEditor = true
  }

  function handleEditIdentity(identity: account.Identity) {
    identityEditorSaved = false
    defaultDisplayNameBeforeEdit = identity.isDefault ? defaultDisplayName : null
    editingIdentity = identity.isDefault && displayNameLoaded
      ? new account.Identity({ ...identity, name: defaultDisplayName })
      : identity
    showEditor = true
  }

  async function handleSaveIdentity(config: account.IdentityConfig) {
    if (editingIdentity) {
      // Update existing
      await UpdateIdentity(editingIdentity.id, config)
      addToast({
        type: 'success',
        message: $_('identity.emailUpdated'),
      })
      if (editingIdentity.isDefault) {
        identityEditorSaved = true
        onDefaultDisplayNameChange?.(config.name)
      }
    } else {
      // Create new
      await CreateIdentity(accountId, config)
      addToast({
        type: 'success',
        message: $_('identity.emailAdded'),
      })
    }
    await loadIdentities()
  }

  function handleEditorNameChange(value: string) {
    if (editingIdentity?.isDefault) {
      onDefaultDisplayNameChange?.(value)
    }
  }

  function handleEditorClose() {
    if (editingIdentity?.isDefault && !identityEditorSaved && defaultDisplayNameBeforeEdit !== null) {
      onDefaultDisplayNameChange?.(defaultDisplayNameBeforeEdit)
    }
    showEditor = false
    editingIdentity = null
    identityEditorSaved = false
    defaultDisplayNameBeforeEdit = null
  }

  function handleDeleteIdentity(identity: account.Identity) {
    if (identity.isDefault) {
      addToast({
        type: 'error',
        message: $_('identity.cannotDeleteDefault'),
      })
      return
    }

    identityToDelete = identity
    showDeleteConfirm = true
  }

  async function confirmDeleteIdentity() {
    if (!identityToDelete) return

    const identity = identityToDelete
    deletingId = identity.id
    try {
      await DeleteIdentity(identity.id)
      addToast({
        type: 'success',
        message: $_('identity.emailDeleted'),
      })
      await loadIdentities()
    } catch (err) {
      console.error('Failed to delete identity:', err)
      addToast({
        type: 'error',
        message: $_('toast.failedToDeleteIdentity'),
      })
    } finally {
      deletingId = null
      identityToDelete = null
    }
  }

  function cancelDeleteIdentity() {
    identityToDelete = null
  }

  async function handleSetDefault(identity: account.Identity) {
    if (identity.isDefault) return

    try {
      await SetDefaultIdentity(accountId, identity.id)
      addToast({
        type: 'success',
        message: $_('identity.isNowDefault', { values: { email: identity.email } }),
      })
      await loadIdentities()
    } catch (err) {
      console.error('Failed to set default identity:', err)
      addToast({
        type: 'error',
        message: $_('identity.failedToSetDefault'),
      })
    }
  }

</script>

<div class="space-y-4">
  <div class="flex items-center justify-between">
    <div>
      <h3 class="text-sm font-medium flex items-center gap-2">
        <Icon icon="mdi:email-multiple-outline" class="w-4 h-4" />
        {$_('identity.emailAddresses')}
      </h3>
    </div>
    <Button size="sm" onclick={handleAddIdentity}>
      <Icon icon="mdi:plus" class="w-4 h-4 mr-1" />
      {$_('identity.addEmailAddress')}
    </Button>
  </div>

  {#if loading}
    <div class="flex items-center justify-center py-8">
      <Icon icon="mdi:loading" class="w-6 h-6 animate-spin text-muted-foreground" />
    </div>
  {:else if identities.length === 0}
    <div class="text-center py-8 text-muted-foreground">
      <Icon icon="mdi:email-outline" class="w-12 h-12 mx-auto mb-2 opacity-50" />
      <p>{$_('identity.noEmailAddresses')}</p>
    </div>
  {:else}
    <div class="space-y-2">
      {#each identities as identity (identity.id)}
        <div class="flex items-center gap-3 p-3 rounded-lg border border-border bg-card hover:bg-accent/50 transition-colors group">
          <!-- Default radio button -->
          <button
            type="button"
            onclick={() => handleSetDefault(identity)}
            class="flex-shrink-0 w-4 h-4 rounded-full border-2 flex items-center justify-center transition-colors
              {identity.isDefault
                ? 'border-primary bg-primary'
                : 'border-muted-foreground hover:border-primary'}"
            title={identity.isDefault ? $_('identity.defaultAddress') : $_('identity.setAsDefaultAddress')}
          >
            {#if identity.isDefault}
              <div class="w-1.5 h-1.5 rounded-full bg-white"></div>
            {/if}
          </button>

          <!-- Identity info -->
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2">
              <span class="font-medium text-sm truncate">{identity.email}</span>
              {#if identity.isDefault}
                <span class="text-xs bg-primary/10 text-primary px-1.5 py-0.5 rounded">{$_('identity.default')}</span>
              {/if}
              <span class="text-xs bg-muted/60 text-muted-foreground border border-border px-1.5 py-0.5 rounded shrink-0">
                {getSignatureBadge(identity)}
              </span>
            </div>
          </div>

          <!-- Actions -->
          <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
            <Button
              variant="ghost"
              size="sm"
              onclick={() => handleEditIdentity(identity)}
              class="h-8 w-8 p-0"
              title={$_('common.edit')}
            >
              <Icon icon="mdi:pencil" class="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onclick={() => handleDeleteIdentity(identity)}
              disabled={identity.isDefault || deletingId === identity.id}
              class="h-8 w-8 p-0 text-destructive hover:text-destructive"
              title={identity.isDefault ? $_('identity.cannotDeleteDefaultTitle') : $_('common.delete')}
            >
              {#if deletingId === identity.id}
                <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
              {:else}
                <Icon icon="mdi:delete" class="w-4 h-4" />
              {/if}
            </Button>
          </div>
        </div>
      {/each}
    </div>
  {/if}

</div>

{#if isMicrosoft}
  <!-- Shared Mailboxes (Microsoft 365 only) -->
  <div class="space-y-4 mt-6 pt-6 border-t border-border">
    <div class="flex items-center justify-between">
      <div>
        <h3 class="text-sm font-medium flex items-center gap-2">
          <Icon icon="mdi:email-multiple-outline" class="w-4 h-4" />
          {$_('identity.sharedMailboxes')}
        </h3>
        <p class="text-xs text-muted-foreground mt-1">
          {$_('identity.sharedMailboxesHelp')}
        </p>
      </div>
      <Button size="sm" onclick={() => showAddSharedMailbox = true}>
        <Icon icon="mdi:plus" class="w-4 h-4 mr-1" />
        {$_('identity.addSharedMailbox')}
      </Button>
    </div>

    {#if sharedMailboxLoading}
      <div class="flex items-center justify-center py-6">
        <Icon icon="mdi:loading" class="w-5 h-5 animate-spin text-muted-foreground" />
      </div>
    {:else if sharedMailboxes.length === 0}
      <div class="text-center py-6 text-muted-foreground">
        <p class="text-sm">{$_('identity.noSharedMailboxes')}</p>
      </div>
    {:else}
      <div class="space-y-2">
        {#each sharedMailboxes as mailbox (mailbox.id)}
          <div class="flex items-center gap-3 p-3 rounded-lg border border-border bg-card hover:bg-accent/50 transition-colors group">
            <Icon icon="mdi:email-outline" class="w-4 h-4 text-muted-foreground shrink-0" />
            <div class="flex-1 min-w-0">
              <div class="font-medium text-sm truncate">{mailbox.email}</div>
              {#if mailbox.name && mailbox.name !== mailbox.email}
                <div class="text-xs text-muted-foreground truncate">{mailbox.name}</div>
              {/if}
            </div>
            <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
              <Button
                variant="ghost"
                size="sm"
                onclick={() => { editingSharedMailbox = mailbox; showSharedMailboxEditor = true }}
                class="h-8 w-8 p-0"
                title={$_('common.edit')}
              >
                <Icon icon="mdi:pencil" class="w-4 h-4" />
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onclick={() => handleDeleteSharedMailbox(mailbox)}
                class="h-8 w-8 p-0 text-destructive hover:text-destructive"
                title={$_('common.delete')}
              >
                <Icon icon="mdi:delete" class="w-4 h-4" />
              </Button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
{/if}

<!-- Add Shared Mailbox Dialog -->
<Dialog.Root bind:open={showAddSharedMailbox}>
  <Dialog.Content class="max-w-sm">
    <Dialog.Header>
      <Dialog.Title>{$_('identity.addSharedMailbox')}</Dialog.Title>
    </Dialog.Header>
    <div class="space-y-4">
      <div class="space-y-2">
        <Label>{$_('identity.sharedMailboxEmail')}</Label>
        <Input
          type="email"
          bind:value={sharedMailboxEmail}
          placeholder="shared@company.com"
        />
      </div>
      <div class="space-y-2">
        <Label>{$_('identity.sharedMailboxDisplayName')}</Label>
        <Input
          bind:value={sharedMailboxDisplayName}
          placeholder={$_('common.optional')}
        />
      </div>
    </div>
    <Dialog.Footer>
      <Button variant="outline" onclick={() => showAddSharedMailbox = false}>
        {$_('common.cancel')}
      </Button>
      <Button
        onclick={handleAddSharedMailbox}
        disabled={!sharedMailboxEmail.trim() || addingSharedMailbox}
      >
        {#if addingSharedMailbox}
          <Icon icon="mdi:loading" class="w-4 h-4 animate-spin mr-1" />
        {/if}
        {$_('common.add')}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<!-- Identity Editor Dialog -->
<IdentityEditor
  bind:open={showEditor}
  {accountId}
  identity={editingIdentity}
  linkedName={editingIdentity?.isDefault && displayNameLoaded ? defaultDisplayName : undefined}
  onNameChange={handleEditorNameChange}
  onSave={handleSaveIdentity}
  onClose={handleEditorClose}
/>

<ConfirmDialog
  bind:open={showDeleteConfirm}
  title={$_('identity.deleteAliasTitle')}
  description={$_('identity.deleteAliasConfirm', { values: { email: identityToDelete?.email ?? '' } })}
  confirmLabel={$_('common.delete')}
  cancelLabel={$_('common.cancel')}
  variant="destructive"
  loading={deletingId !== null}
  onConfirm={confirmDeleteIdentity}
  onCancel={cancelDeleteIdentity}
/>

<!-- Shared Mailbox Editor Dialog (reuses AccountDialog) -->
{#if editingSharedMailbox}
  <AccountDialog
    bind:open={showSharedMailboxEditor}
    editAccount={editingSharedMailbox}
    onClose={() => { showSharedMailboxEditor = false; editingSharedMailbox = null; loadSharedMailboxes() }}
  />
{/if}
