<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import ConfirmDialog from '$lib/components/ui/confirm-dialog/ConfirmDialog.svelte'
  import IdentityEditor from './IdentityEditor.svelte'
  import { addToast } from '$lib/stores/toast'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { account } from '../../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import { GetIdentities, CreateIdentity, UpdateIdentity, DeleteIdentity, SetDefaultIdentity } from '../../../../../wailsjs/go/app/App'

  interface Props {
    /** The account being edited */
    accountId: string
    /** The full account object */
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
    editAccount: _editAccount,
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

  function getSignatureBadge(identity: account.Identity): string {
    if (identity.signatureEnabled === false) return $_('identity.signatureBadgeNone')
    if ((identity.signatureHtml || '').trim()) return $_('identity.signatureBadgeHtml')
    if ((identity.signatureText || '').trim()) return $_('identity.signatureBadgePlain')
    return $_('identity.signatureBadgeNone')
  }

  onMount(async () => {
    await loadIdentities()
  })

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
