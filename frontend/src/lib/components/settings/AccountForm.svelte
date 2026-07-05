<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import * as Select from '$lib/components/ui/select'
  import Switch from '$lib/components/ui/switch/Switch.svelte'
  import {
    providers,
    detectProvider,
    getCustomProvider,
    securityOptions,
    syncPeriodOptions,
    syncIntervalOptions,
    type EmailProvider,
  } from '$lib/config/providers'
  // @ts-ignore - wailsjs path
  import { account, certificate, app, folder } from '../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import { GetAccountFoldersForMapping, GetAutoDetectedFolders, GetIdentities, AcceptCertificate, GetAllAccountIdentities, GetTrustedCertificates, RemoveTrustedCertificate, GetFolders, SubscribeFolder, UnsubscribeFolder, SubscribeAllFolders } from '../../../../wailsjs/go/app/App'
  import CertificateDialog from './CertificateDialog.svelte'
  import ConnectionTestDialog from './ConnectionTestDialog.svelte'
  import AccountIdentityTab from './account/AccountIdentityTab.svelte'
  import ConfirmDialog from '$lib/components/ui/confirm-dialog/ConfirmDialog.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { _ } from '$lib/i18n'

  interface Props {
    /** Account to edit (null for new account) */
    editAccount?: account.Account | null
    /** True when a new account was created and the dialog stayed open */
    createdInDialog?: boolean
    /** Callback when form is submitted successfully */
    onSubmit?: (config: account.AccountConfig) => Promise<void>
    /** Callback when form is cancelled */
    onCancel?: () => void
  }

  let {
    editAccount = null,
    createdInDialog = false,
    onSubmit,
    onCancel,
  }: Props = $props()

  // Form state
  let selectedProvider = $state<EmailProvider | null>(null)
  let showAdvanced = $state(false)

  // Form fields
  let displayName = $state('')
  let color = $state('')
  let email = $state('')
  let prevEmail = $state('')
  let username = $state('')
  let password = $state('')
  let imapHost = $state('')
  let imapPort = $state(993)
  let imapSecurity = $state<string>('tls')
  let smtpHost = $state('')
  let smtpPort = $state(465)
  let smtpSecurity = $state<string>('tls')
  let noOutgoingServer = $state(false)
  let smtpUsername = $state('')
  let smtpPassword = $state('')
  // Auto-infer SMTP host from the IMAP host for new Generic accounts. Goes
  // sticky-off the moment the user types directly into the SMTP field —
  // manual edits stick.
  let smtpHostMirrorsImap = $state(true)
  let replyForwardIdentityID = $state('')
  let availableIdentityGroups = $state<app.AccountIdentityGroup[]>([])
  // True only when the user explicitly picked Generic/Custom (or the detector
  // fell back to it). Used to mirror IMAP host edits into SMTP host for manual
  // setup until the user edits SMTP directly.
  const isGenericProvider = $derived(selectedProvider?.id === 'custom' || selectedProvider?.id === 'generic')
  let syncPeriodDays = $state<string>('180')
  let syncInterval = $state<string>('30') // Default: 30 minutes
  let syncAllFolders = $state(false)
  let syncFoldersEnabled = $state(false)
  let readReceiptRequestPolicy = $state<string>('never')
  let displayNameLoaded = $state(false)

  // Read receipt request policy options
  const readReceiptRequestOptions = $derived([
    { value: 'never', label: $_('account.neverRequest') },
    { value: 'ask', label: $_('account.askEachTime') },
    { value: 'always', label: $_('account.alwaysRequest') },
  ])

  // Helper functions to get labels
  function getSecurityLabel(value: string): string {
    return securityOptions.find(opt => opt.value === value)?.label || value
  }

  function getSyncPeriodLabel(value: string): string {
    const numValue = Number(value)
    const option = syncPeriodOptions.find(opt => opt.value === numValue)
    return option ? $_(option.labelKey) : `${value} days`
  }

  function getSyncIntervalLabel(value: string): string {
    const numValue = Number(value)
    const option = syncIntervalOptions.find(opt => opt.value === numValue)
    return option ? $_(option.labelKey) : `${value} min`
  }

  function getReadReceiptLabel(value: string): string {
    return readReceiptRequestOptions.find(opt => opt.value === value)?.label || value
  }

  function inferSmtpHostFromImapHost(value: string): string {
    const host = value.trim()
    if (!host) return ''
    if (/^imap(?=[.-])/i.test(host)) return host.replace(/^imap/i, 'smtp')
    if (/[.-]imap[.-]/i.test(host)) return host.replace(/([.-])imap([.-])/i, '$1smtp$2')
    return ''
  }

  // UI state
  let testing = $state(false)
  let testResult = $state<{ success: boolean; message: string } | null>(null)
  let showTestConnDialog = $state(false)
  let submitting = $state(false)
  let errors = $state<Record<string, string>>({})
  let initialized = $state(false)

  // Certificate TOFU state
  let showCertDialog = $state(false)
  let pendingCertificate = $state<certificate.CertificateInfo | null>(null)

  // Folder mapping state
  let showFolderMapping = $state(false)
  let loadingFolders = $state(false)
  let availableFolders = $state<any[]>([])
  let autoDetectedFolders = $state<Record<string, string>>({})

  // Folder mapping values
  let sentFolderPath = $state('')
  let draftsFolderPath = $state('')
  let trashFolderPath = $state('')
  let spamFolderPath = $state('')
  let archiveFolderPath = $state('')
  let allMailFolderPath = $state('')
  let starredFolderPath = $state('')

  // Folder sync subscription state
  let showFolderSync = $state(false)
  let loadingSyncFolders = $state(false)
  let syncFolders = $state<folder.Folder[]>([])
  const coreFolderTypes = ['inbox', 'drafts', 'sent']

  // Trusted certificates state
  let showTrustedCerts = $state(false)
  let loadingCerts = $state(false)
  let trustedCerts = $state<certificate.CertificateInfo[]>([])
  let confirmRemoveFingerprint = $state<string | null>(null)
  let showRemoveConfirm = $state(false)

  // SMTP authentication is hardcoded to "same as incoming server": there is
  // no separate-SMTP-credentials UI anymore. Force the hidden values empty so
  // updates drop any legacy separate SMTP keyring entry.
  $effect(() => {
    if (smtpUsername !== '') smtpUsername = ''
    if (smtpPassword !== '') smtpPassword = ''
  })

  // Load folders for mapping UI
  async function loadFoldersForMapping() {
    if (!editAccount || availableFolders.length > 0) return

    loadingFolders = true
    try {
      availableFolders = await GetAccountFoldersForMapping(editAccount.id)
      autoDetectedFolders = await GetAutoDetectedFolders(editAccount.id)

      // Pre-select: use saved value if exists, otherwise auto-detected
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      sentFolderPath = editAccount.sentFolderPath || autoDetectedFolders.sent || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      draftsFolderPath = editAccount.draftsFolderPath || autoDetectedFolders.drafts || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      trashFolderPath = editAccount.trashFolderPath || autoDetectedFolders.trash || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      spamFolderPath = editAccount.spamFolderPath || autoDetectedFolders.spam || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      archiveFolderPath = editAccount.archiveFolderPath || autoDetectedFolders.archive || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      allMailFolderPath = editAccount.allMailFolderPath || autoDetectedFolders.all || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      starredFolderPath = editAccount.starredFolderPath || autoDetectedFolders.starred || ''
    } catch (err) {
      console.error('Failed to load folders for mapping:', err)
    } finally {
      loadingFolders = false
    }
  }

  async function loadSyncFolders() {
    if (!editAccount || syncFolders.length > 0) return
    loadingSyncFolders = true
    try {
      syncFolders = await GetFolders(editAccount.id)
    } catch (err) {
      console.error('Failed to load folders for sync:', err)
    } finally {
      loadingSyncFolders = false
    }
  }

  function handleFolderSyncToggle() {
    showFolderSync = !showFolderSync
    if (showFolderSync) {
      loadSyncFolders()
    }
  }

  async function handleSyncAllToggle(checked: boolean) {
    syncAllFolders = checked
    if (!checked || !editAccount) return
    try {
      await SubscribeAllFolders(editAccount.id)
      syncFolders = await GetFolders(editAccount.id)
    } catch (err) {
      console.error('Failed to subscribe to all folders:', err)
    }
  }

  async function handleFolderSubscriptionToggle(f: folder.Folder, subscribed: boolean) {
    if (!editAccount) return
    const action = subscribed ? SubscribeFolder : UnsubscribeFolder
    try {
      await action(editAccount.id, f.id)
      syncFolders = syncFolders.map(sf =>
        sf.id === f.id ? { ...sf, subscribed } as folder.Folder : sf
      )
    } catch (err) {
      console.error('Failed to update folder subscription:', err)
    }
  }

  function handleTrustedCertsToggle() {
    showTrustedCerts = !showTrustedCerts
    if (showTrustedCerts) {
      loadTrustedCerts()
    }
  }

  async function loadTrustedCerts() {
    if (!editAccount) return
    loadingCerts = true
    try {
      const hosts = [imapHost, smtpHost].filter(h => h)
      const result = await GetTrustedCertificates(hosts)
      trustedCerts = result || []
    } catch (err) {
      console.error('Failed to load trusted certificates:', err)
      trustedCerts = []
    } finally {
      loadingCerts = false
    }
  }

  function handleRemoveCert(fingerprint: string) {
    confirmRemoveFingerprint = fingerprint
    showRemoveConfirm = true
  }

  async function confirmRemoveCert() {
    if (!confirmRemoveFingerprint) return
    try {
      await RemoveTrustedCertificate(confirmRemoveFingerprint)
      trustedCerts = trustedCerts.filter(c => c.fingerprint !== confirmRemoveFingerprint)
    } catch (err) {
      console.error('Failed to remove certificate:', err)
    }
    showRemoveConfirm = false
    confirmRemoveFingerprint = null
  }

  function formatFingerprint(fp: string): string {
    if (!fp) return ''
    const parts: string[] = []
    for (let i = 0; i < fp.length && i < 16; i += 2) {
      parts.push(fp.substring(i, i + 2).toUpperCase())
    }
    return parts.join(':') + '...'
  }

  function formatCertDate(iso: string): string {
    if (!iso) return 'N/A'
    try {
      return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
    } catch {
      return iso
    }
  }

  // New account: skip the provider picker grid — open directly into the
  // manual ("Other / Manual") form. Server fields start blank for manual entry.
  $effect(() => {
    if (!editAccount && !selectedProvider) {
      const custom = providers.find((p) => p.id === 'custom')
      if (custom) selectProvider(custom)
    }
  })

  // Initialize form when editing (only once)
  $effect(() => {
    if (editAccount && !initialized) {
      initialized = true
      email = editAccount.email
      username = editAccount.username
      password = ''
      imapHost = editAccount.imapHost
      imapPort = editAccount.imapPort
      imapSecurity = editAccount.imapSecurity
      smtpHost = editAccount.smtpHost
      smtpPort = editAccount.smtpPort
      smtpSecurity = editAccount.smtpSecurity
      noOutgoingServer = editAccount.noOutgoingServer || false
      smtpUsername = editAccount.smtpUsername || ''
      smtpPassword = ''
      replyForwardIdentityID = editAccount.replyForwardIdentityId || ''
      syncPeriodDays = String(editAccount.syncPeriodDays)
      // @ts-ignore - syncInterval from backend
      syncInterval = String(editAccount.syncInterval ?? 30)
      syncAllFolders = editAccount.syncAllFolders || false
      syncFoldersEnabled = editAccount.syncFoldersEnabled || false
      readReceiptRequestPolicy = editAccount.readReceiptRequestPolicy || 'never'
      // @ts-ignore - color from backend
      color = editAccount.color || ''
      displayNameLoaded = false

      // Initialize folder mappings (will be populated when section is expanded)
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      sentFolderPath = editAccount.sentFolderPath || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      draftsFolderPath = editAccount.draftsFolderPath || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      trashFolderPath = editAccount.trashFolderPath || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      spamFolderPath = editAccount.spamFolderPath || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      archiveFolderPath = editAccount.archiveFolderPath || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      allMailFolderPath = editAccount.allMailFolderPath || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      starredFolderPath = editAccount.starredFolderPath || ''

      // Try to detect provider
      selectedProvider = detectProvider(email) ?? getCustomProvider()
      showAdvanced = true

      // Load display name from the default identity
      loadDisplayName(editAccount.id)
    }
  })

  // Load display name from the account's default identity
  async function loadDisplayName(accountId: string) {
    try {
      const identities = await GetIdentities(accountId)
      const defaultIdentity = identities?.find((id: any) => id.isDefault) || identities?.[0]
      if (defaultIdentity) {
        displayName = defaultIdentity.name || ''
      }
    } catch (err) {
      console.error('Failed to load display name:', err)
    } finally {
      displayNameLoaded = true
    }
  }

  // One-time initialization on mount
  let formInitialized = $state(false)
  $effect(() => {
    if (!formInitialized) {
      formInitialized = true
      // Load sendable identity groups for the Reply/Forward-with picker.
      // Used only when the user toggles "No outgoing server" on; cheap
      // single Wails call so load it up-front for snappier UI.
      loadIdentityGroups()
    }
  })

  async function loadIdentityGroups() {
    try {
      const groups = (await GetAllAccountIdentities()) || []
      // Exclude the account being edited (its own identities can't be
      // a "Reply/Forward-with" target when it's marked no-outgoing) and
      // any other no-outgoing accounts (their identities aren't sendable
      // either).
      availableIdentityGroups = groups.filter((g: app.AccountIdentityGroup) => g.account?.id !== editAccount?.id && !g.account?.noOutgoingServer)
    } catch (err) {
      console.error('Failed to load identity groups for Reply/Forward-with picker:', err)
      availableIdentityGroups = []
    }
  }

  // Auto-fill settings when provider is selected
  function selectProvider(provider: EmailProvider) {
    selectedProvider = provider
    imapHost = provider.imap.host
    imapPort = provider.imap.port
    imapSecurity = provider.imap.security
    smtpHost = provider.smtp.host
    smtpPort = provider.smtp.port
    smtpSecurity = provider.smtp.security

    // Show advanced for custom provider
    showAdvanced = provider.id === 'custom'
  }

  // Auto-detect provider and auto-fill fields when email changes
  $effect(() => {
    if (!email) return

    // Auto-fill username with full email
    if (!username || username === prevEmail) {
      username = email
    }
    prevEmail = email

    // Try to detect provider
    const detected = detectProvider(email)
    if (detected && detected.id !== selectedProvider?.id) {
      selectProvider(detected)
    }
  })

  // Build config from form fields
  function buildConfig(): account.AccountConfig {
    return new account.AccountConfig({
      // Account name is no longer user-editable; the sidebar label is just the
      // email address. Keep name in sync with the email.
      name: email,
      displayName,
      color,
      email,
      username: username || email,
      password,
      imapHost,
      imapPort,
      imapSecurity,
      smtpHost,
      smtpPort,
      smtpSecurity,
      noOutgoingServer,
      smtpUsername,
      smtpPassword,
      replyForwardIdentityId: replyForwardIdentityID,
      authType: 'password',
      syncPeriodDays: Number(syncPeriodDays),
      syncInterval: Number(syncInterval),
      syncAllFolders,
      syncFoldersEnabled,
      readReceiptRequestPolicy,
      // Folder mappings
      sentFolderPath,
      draftsFolderPath,
      trashFolderPath,
      spamFolderPath,
      archiveFolderPath,
      allMailFolderPath,
      starredFolderPath,
    })
  }

  // Validate form
  function validate(): boolean {
    errors = {}

    if (!displayName.trim()) errors.displayName = $_('account.displayNameRequired')
    if (!email.trim()) errors.email = $_('account.emailRequired')
    else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) errors.email = $_('account.invalidEmail')

    // Password is only required on new accounts (blank keeps the stored one when editing)
    if (!password && !editAccount) {
      errors.password = $_('account.passwordRequired')
    }

    if (!imapHost.trim()) errors.imapHost = $_('account.imapHostRequired')
    if (imapPort < 1 || imapPort > 65535) errors.imapPort = $_('account.invalidPort')
    // SMTP host/port checks only when the user wants outgoing.
    if (!noOutgoingServer) {
      if (!smtpHost.trim()) errors.smtpHost = $_('account.smtpHostRequired')
      if (smtpPort < 1 || smtpPort > 65535) errors.smtpPort = $_('account.invalidPort')
    }
    return Object.keys(errors).length === 0
  }

  // Test connection
  async function handleTestConnection() {
    if (!validate()) return

    showTestConnDialog = true
    testing = true
    testResult = null

    try {
      const result = editAccount && !password
        ? await accountStore.testAccountConnection(editAccount.id)
        : await accountStore.testConnection(buildConfig())
      if (result.success) {
        testResult = { success: true, message: $_('account.connectionSuccessful') }
      } else if (result.certificateRequired && result.certificate) {
        showTestConnDialog = false
        pendingCertificate = result.certificate
        showCertDialog = true
      } else {
        testResult = { success: false, message: result.error || $_('account.connectionFailed') }
      }
    } catch (err) {
      console.error('Connection test failed:', err)
      testResult = {
        success: false,
        message: $_('account.connectionTestFailed'),
      }
    } finally {
      testing = false
    }
  }

  async function handleCertAcceptOnce() {
    if (!pendingCertificate) return
    await AcceptCertificate(imapHost, pendingCertificate, false)
    showCertDialog = false
    pendingCertificate = null
    handleTestConnection()
  }

  async function handleCertAcceptPermanently() {
    if (!pendingCertificate) return
    await AcceptCertificate(imapHost, pendingCertificate, true)
    showCertDialog = false
    pendingCertificate = null
    handleTestConnection()
  }

  function handleCertDecline() {
    showCertDialog = false
    pendingCertificate = null
    testResult = { success: false, message: $_('account.certificateDeclined') }
  }

  // Submit form
  async function handleSubmit(e: Event) {
    e.preventDefault()
    if (!validate()) return

    submitting = true
    testResult = null

    try {
      await onSubmit?.(buildConfig())
    } catch (err) {
      console.error('Account save failed:', err)
      showTestConnDialog = true
      testResult = {
        success: false,
        message: $_('account.saveFailed'),
      }
    } finally {
      submitting = false
    }
  }

</script>

<form onsubmit={handleSubmit} class="flex flex-col h-[460px] max-h-[calc(90vh-140px)]">
  {#if true}
    <!-- Account Details (manual entry — provider grid removed). The fields
         scroll inside this flex-1 area; the actions footer below stays pinned. -->
    <div class="flex-1 min-h-0 overflow-y-auto space-y-4 pt-1.5 pl-1 pr-3">
      {#if selectedProvider?.notes}
        <div class="flex items-start gap-2 p-3 rounded-lg bg-amber-500/10 border border-amber-500/20">
          <Icon icon="mdi:information-outline" class="w-5 h-5 text-amber-500 flex-shrink-0 mt-0.5" />
          <p class="text-sm text-amber-600 dark:text-amber-400">
            {selectedProvider.notesKey ? $_(selectedProvider.notesKey) : selectedProvider.notes}
          </p>
        </div>
      {/if}

      <!-- Basic Fields (label + control on one row, matching the edit dialog) -->
      <div class="space-y-4">
        <div>
          <div class="flex items-center justify-between gap-4">
            <div class="min-w-0">
              <Label for="displayName">{$_('account.displayName')}</Label>
              <p class="text-xs text-muted-foreground">{$_('account.displayNameHelp')}</p>
            </div>
            <Input
              id="displayName"
              type="text"
              placeholder={$_('account.displayNamePlaceholder')}
              bind:value={displayName}
              class="w-64 shrink-0 {errors.displayName ? 'border-destructive' : ''}"
            />
          </div>
          {#if errors.displayName}
            <p class="text-sm text-destructive mt-1">{errors.displayName}</p>
          {/if}
        </div>

        <div>
          <div class="flex items-center justify-between gap-4">
            <Label for="email">{$_('account.emailAddress')}</Label>
            <Input
              id="email"
              type="email"
              placeholder="you@example.com"
              bind:value={email}
              disabled={!!editAccount}
              class="w-64 shrink-0 {editAccount ? 'bg-muted' : ''} {errors.email ? 'border-destructive' : ''}"
            />
          </div>
          {#if errors.email}
            <p class="text-sm text-destructive mt-1">{errors.email}</p>
          {/if}
        </div>

        <div class="flex items-center justify-between gap-4">
          <div class="min-w-0">
            <Label for="username">{$_('account.username')}</Label>
            <p class="text-xs text-muted-foreground">{$_('account.usernameHelp')}</p>
          </div>
          <Input
            id="username"
            type="text"
            placeholder={$_('account.usernamePlaceholder')}
            bind:value={username}
            class="w-64 shrink-0"
          />
        </div>

        <!-- Password field -->
        <div>
          <div class="flex items-center justify-between gap-4">
            <Label for="password">
              {selectedProvider?.notes?.includes('App Password') ? $_('account.appPassword') : $_('account.password')}
            </Label>
            <Input
              id="password"
              type="password"
              placeholder={editAccount ? $_('account.leaveEmptyToKeep') : $_('account.password')}
              bind:value={password}
              class="w-64 shrink-0 {errors.password ? 'border-destructive' : ''}"
            />
          </div>
          {#if errors.password}
            <p class="text-sm text-destructive mt-1">{errors.password}</p>
          {/if}
        </div>
      </div>

      <!-- Sender addresses. Requires a persisted account because aliases are
           stored separately from AccountConfig and need accountId. -->
      <div class="pt-2 border-t border-border">
        {#if editAccount}
          <AccountIdentityTab
            accountId={editAccount.id}
            {editAccount}
            defaultDisplayName={displayName}
            {displayNameLoaded}
            onDefaultDisplayNameChange={(v) => displayName = v}
          />
        {:else}
          <div class="flex items-center justify-between gap-4 opacity-60">
            <div class="min-w-0">
              <Label>{$_('identity.emailAddresses')}</Label>
              <p class="text-xs text-muted-foreground">{$_('account.saveAccountFirst')}</p>
            </div>
            <Button type="button" variant="outline" size="sm" disabled>
              <Icon icon="mdi:email-multiple-outline" class="w-4 h-4 mr-1" />
              {$_('identity.addEmailAddress')}
            </Button>
          </div>
        {/if}
      </div>

      <!-- Advanced Settings Toggle — label (styled like a field label) on the
           left, an up/down triangle on the right (▼ = expand, ▲ = collapse). -->
      <button
        type="button"
        class="flex items-center gap-1.5 transition-colors"
        onclick={() => (showAdvanced = !showAdvanced)}
      >
        <span class="text-sm font-medium text-foreground">{$_('account.advancedSettings')}</span>
        <Icon
          icon={showAdvanced ? 'mdi:menu-up' : 'mdi:menu-down'}
          class="w-4 h-4 text-muted-foreground"
        />
      </button>

      {#if showAdvanced}
        <div class="space-y-4 pt-2 border-t border-border">
          <!-- Incoming server (IMAP) — no group header -->
          <div class="space-y-3">
            <div class="grid grid-cols-2 gap-3">
              <div class="space-y-2">
                <Label for="imapHost">{$_('account.incomingServer')}</Label>
                <Input
                  id="imapHost"
                  type="text"
                  placeholder="imap.example.com"
                  bind:value={imapHost}
                  oninput={(e) => {
                    const v = (e.target as HTMLInputElement).value
                    if (isGenericProvider && !editAccount && smtpHostMirrorsImap) {
                      smtpHost = inferSmtpHostFromImapHost(v)
                    }
                  }}
                  class={errors.imapHost ? 'border-destructive' : ''}
                />
                {#if errors.imapHost}
                  <p class="text-sm text-destructive">{errors.imapHost}</p>
                {/if}
              </div>
              <div class="grid grid-cols-2 gap-2">
                <div class="space-y-2">
                  <Label for="imapPort">{$_('account.port')}</Label>
                  <Input
                    id="imapPort"
                    type="number"
                    bind:value={imapPort}
                    class={errors.imapPort ? 'border-destructive' : ''}
                  />
                </div>
                <div class="space-y-2">
                  <Label>{$_('account.security')}</Label>
                  <Select.Root bind:value={imapSecurity}>
                    <Select.Trigger class="h-10">
                      <Select.Value placeholder="Select">
                        {getSecurityLabel(imapSecurity)}
                      </Select.Value>
                    </Select.Trigger>
                    <Select.Content>
                      {#each securityOptions as opt (opt.value)}
                        <Select.Item value={opt.value} label={opt.label} />
                      {/each}
                    </Select.Content>
                  </Select.Root>
                </div>
              </div>
            </div>
          </div>

          <!-- Divider -->
          <div class="border-t border-border"></div>

          <!-- Outgoing server (SMTP). Header carries the no-outgoing toggle. -->
          <div class="space-y-4">
            <div class="flex items-center justify-between gap-2">
              <span class={noOutgoingServer ? 'text-sm font-medium' : 'sr-only'}>{$_('account.outgoingServer')}</span>
              <div class="flex items-center gap-2">
                <Switch bind:checked={noOutgoingServer} />
                <span class="text-sm text-muted-foreground">{$_('account.noOutgoingServer')}</span>
              </div>
            </div>

            {#if noOutgoingServer}
              <!-- Reply/Forward-with picker. Same shape as the composer's
                   From dropdown. Default = empty value, which the composer
                   resolves to the user's default sending identity. -->
              <div class="pt-2 space-y-1">
                <Label>{$_('account.replyForwardWith')}</Label>
                <Select.Root bind:value={replyForwardIdentityID}>
                  <Select.Trigger class="h-10">
                    <Select.Value placeholder={$_('account.replyForwardWithDefault')}>
                      {#if replyForwardIdentityID}
                        {@const allIdentities = availableIdentityGroups.flatMap(g => (g.identities || []).map(i => ({ identity: i, group: g })))}
                        {@const found = allIdentities.find(x => x.identity.id === replyForwardIdentityID)}
                        {#if found}
                          {#if found.group.account?.color}
                            <span class="inline-block w-2 h-2 rounded-full mr-1.5 flex-shrink-0" style="background-color: {found.group.account.color}"></span>
                          {/if}
                          {found.identity.name} &lt;{found.identity.email}&gt;
                        {:else}
                          {$_('account.replyForwardWithDefault')}
                        {/if}
                      {:else}
                        {$_('account.replyForwardWithDefault')}
                      {/if}
                    </Select.Value>
                  </Select.Trigger>
                  <Select.Content>
                    <Select.Item value="" label={$_('account.replyForwardWithDefault')} />
                    {#each availableIdentityGroups as group (group.account?.id)}
                      <Select.Group>
                        <Select.GroupHeading class="flex items-center gap-1.5 px-2 py-1 text-xs font-medium text-muted-foreground">
                          {#if group.account?.color}
                            <span class="inline-block w-2 h-2 rounded-full flex-shrink-0" style="background-color: {group.account.color}"></span>
                          {/if}
                          {group.account?.name || group.account?.email}
                        </Select.GroupHeading>
                        {#each group.identities || [] as identity (identity.id)}
                          <Select.Item value={identity.id} label="{identity.name} <{identity.email}>" />
                        {/each}
                      </Select.Group>
                    {/each}
                  </Select.Content>
                </Select.Root>
                <p class="text-xs text-muted-foreground">{$_('account.replyForwardWithHelp')}</p>
              </div>
            {:else}
            <div class="grid grid-cols-2 gap-3">
              <div class="space-y-2">
                <Label for="smtpHost">{$_('account.outgoingServer')}</Label>
                <Input
                  id="smtpHost"
                  type="text"
                  placeholder="smtp.example.com"
                  bind:value={smtpHost}
                  oninput={() => { smtpHostMirrorsImap = false }}
                  class={errors.smtpHost ? 'border-destructive' : ''}
                />
                {#if errors.smtpHost}
                  <p class="text-sm text-destructive">{errors.smtpHost}</p>
                {/if}
              </div>
              <div class="grid grid-cols-2 gap-2">
                <div class="space-y-2">
                  <Label for="smtpPort">{$_('account.port')}</Label>
                  <Input
                    id="smtpPort"
                    type="number"
                    bind:value={smtpPort}
                    class={errors.smtpPort ? 'border-destructive' : ''}
                  />
                </div>
                <div class="space-y-2">
                  <Label>{$_('account.security')}</Label>
                  <Select.Root bind:value={smtpSecurity}>
                    <Select.Trigger class="h-10">
                      <Select.Value placeholder="Select">
                        {getSecurityLabel(smtpSecurity)}
                      </Select.Value>
                    </Select.Trigger>
                    <Select.Content>
                      {#each securityOptions as opt (opt.value)}
                        <Select.Item value={opt.value} label={opt.label} />
                      {/each}
                    </Select.Content>
                  </Select.Root>
                </div>
              </div>
            </div>

            {/if}
          </div>

          <!-- Divider -->
          <div class="border-t border-border"></div>

          <!-- Sync settings (label + control on one row) -->
          <div class="flex items-center justify-between gap-4">
            <div class="min-w-0">
              <Label>{$_('account.syncPeriod')}</Label>
              <p class="text-xs text-muted-foreground">{$_('account.syncPeriodHelp')}</p>
            </div>
            <Select.Root bind:value={syncPeriodDays}>
              <Select.Trigger class="w-48 shrink-0">
                <Select.Value placeholder="Select">
                  {getSyncPeriodLabel(syncPeriodDays)}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                {#each syncPeriodOptions as opt (opt.value)}
                  <Select.Item value={String(opt.value)} label={$_(opt.labelKey)} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>

          <div class="flex items-center justify-between gap-4">
            <div class="min-w-0">
              <Label>{$_('account.checkNewMail')}</Label>
              <p class="text-xs text-muted-foreground">{$_('account.checkNewMailHelp')}</p>
            </div>
            <Select.Root bind:value={syncInterval}>
              <Select.Trigger class="w-48 shrink-0">
                <Select.Value placeholder="Select">
                  {getSyncIntervalLabel(syncInterval)}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                {#each syncIntervalOptions as opt (opt.value)}
                  <Select.Item value={String(opt.value)} label={$_(opt.labelKey)} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>

          <div class="flex items-center justify-between gap-4">
            <div class="min-w-0">
              <Label>{$_('account.requestReadReceipts')}</Label>
              <p class="text-xs text-muted-foreground">{$_('account.requestReadReceiptsHelp')}</p>
            </div>
            <Select.Root bind:value={readReceiptRequestPolicy}>
              <Select.Trigger class="w-48 shrink-0">
                <Select.Value placeholder="Select">
                  {getReadReceiptLabel(readReceiptRequestPolicy)}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                {#each readReceiptRequestOptions as opt (opt.value)}
                  <Select.Item value={opt.value} label={opt.label} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>

          <!-- Folder Mapping -->
          <div class="space-y-2">
            <button
              type="button"
              class="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
              onclick={() => {
                showFolderMapping = !showFolderMapping
                if (showFolderMapping) loadFoldersForMapping()
              }}
              disabled={!editAccount}
            >
              <Icon
                icon={showFolderMapping ? 'mdi:chevron-down' : 'mdi:chevron-right'}
                class="w-4 h-4"
              />
              {$_('account.folderMapping')}
              {#if !editAccount}
                <span class="text-xs text-muted-foreground">{$_('account.saveAccountFirst')}</span>
              {/if}
            </button>

            {#if showFolderMapping}
              <div class="space-y-3 pl-6 pt-2 border-l border-border ml-2">
                <p class="text-xs text-muted-foreground">
                  {$_('account.folderMappingHelp2')}
                </p>

                {#if loadingFolders}
                  <div class="flex items-center gap-2 text-sm text-muted-foreground">
                    <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
                    {$_('account.loadingFolders')}
                  </div>
                {:else if availableFolders.length === 0}
                  <p class="text-sm text-muted-foreground">{$_('account.noFoldersAvailable')}</p>
                {:else}
                  <div class="grid gap-3">
                    <!-- Sent -->
                    <div class="grid grid-cols-[100px_1fr] items-center gap-2">
                      <Label class="text-sm">{$_('account.folderSent')}:</Label>
                      <Select.Root bind:value={sentFolderPath}>
                        <Select.Trigger class="h-9">
                          <Select.Value placeholder={$_('account.none')}>
                            {sentFolderPath || $_('account.none')}
                          </Select.Value>
                        </Select.Trigger>
                        <Select.Content>
                          <Select.Item value="" label={$_('account.none')} />
                          {#each availableFolders as f (f.path)}
                            <Select.Item value={f.path} label={f.path + (autoDetectedFolders.sent === f.path ? ' ' + $_('account.detected') : '')} />
                          {/each}
                        </Select.Content>
                      </Select.Root>
                    </div>

                    <!-- Drafts -->
                    <div class="grid grid-cols-[100px_1fr] items-center gap-2">
                      <Label class="text-sm">{$_('account.folderDrafts')}:</Label>
                      <Select.Root bind:value={draftsFolderPath}>
                        <Select.Trigger class="h-9">
                          <Select.Value placeholder={$_('account.none')}>
                            {draftsFolderPath || $_('account.none')}
                          </Select.Value>
                        </Select.Trigger>
                        <Select.Content>
                          <Select.Item value="" label={$_('account.none')} />
                          {#each availableFolders as f (f.path)}
                            <Select.Item value={f.path} label={f.path + (autoDetectedFolders.drafts === f.path ? ' ' + $_('account.detected') : '')} />
                          {/each}
                        </Select.Content>
                      </Select.Root>
                    </div>

                    <!-- Trash -->
                    <div class="grid grid-cols-[100px_1fr] items-center gap-2">
                      <Label class="text-sm">{$_('account.folderTrash')}:</Label>
                      <Select.Root bind:value={trashFolderPath}>
                        <Select.Trigger class="h-9">
                          <Select.Value placeholder={$_('account.none')}>
                            {trashFolderPath || $_('account.none')}
                          </Select.Value>
                        </Select.Trigger>
                        <Select.Content>
                          <Select.Item value="" label={$_('account.none')} />
                          {#each availableFolders as f (f.path)}
                            <Select.Item value={f.path} label={f.path + (autoDetectedFolders.trash === f.path ? ' ' + $_('account.detected') : '')} />
                          {/each}
                        </Select.Content>
                      </Select.Root>
                    </div>

                    <!-- Spam/Junk -->
                    <div class="grid grid-cols-[100px_1fr] items-center gap-2">
                      <Label class="text-sm">{$_('account.folderSpam')}:</Label>
                      <Select.Root bind:value={spamFolderPath}>
                        <Select.Trigger class="h-9">
                          <Select.Value placeholder={$_('account.none')}>
                            {spamFolderPath || $_('account.none')}
                          </Select.Value>
                        </Select.Trigger>
                        <Select.Content>
                          <Select.Item value="" label={$_('account.none')} />
                          {#each availableFolders as f (f.path)}
                            <Select.Item value={f.path} label={f.path + (autoDetectedFolders.spam === f.path ? ' ' + $_('account.detected') : '')} />
                          {/each}
                        </Select.Content>
                      </Select.Root>
                    </div>

                    <!-- Archive -->
                    <div class="grid grid-cols-[100px_1fr] items-center gap-2">
                      <Label class="text-sm">{$_('account.folderArchive')}:</Label>
                      <Select.Root bind:value={archiveFolderPath}>
                        <Select.Trigger class="h-9">
                          <Select.Value placeholder={$_('account.none')}>
                            {archiveFolderPath || $_('account.none')}
                          </Select.Value>
                        </Select.Trigger>
                        <Select.Content>
                          <Select.Item value="" label={$_('account.none')} />
                          {#each availableFolders as f (f.path)}
                            <Select.Item value={f.path} label={f.path + (autoDetectedFolders.archive === f.path ? ' ' + $_('account.detected') : '')} />
                          {/each}
                        </Select.Content>
                      </Select.Root>
                    </div>

                    <!-- All Mail -->
                    <div class="grid grid-cols-[100px_1fr] items-center gap-2">
                      <Label class="text-sm">{$_('account.folderAllMail')}:</Label>
                      <Select.Root bind:value={allMailFolderPath}>
                        <Select.Trigger class="h-9">
                          <Select.Value placeholder={$_('account.none')}>
                            {allMailFolderPath || $_('account.none')}
                          </Select.Value>
                        </Select.Trigger>
                        <Select.Content>
                          <Select.Item value="" label={$_('account.none')} />
                          {#each availableFolders as f (f.path)}
                            <Select.Item value={f.path} label={f.path + (autoDetectedFolders.all === f.path ? ' ' + $_('account.detected') : '')} />
                          {/each}
                        </Select.Content>
                      </Select.Root>
                    </div>

                    <!-- Starred -->
                    <div class="grid grid-cols-[100px_1fr] items-center gap-2">
                      <Label class="text-sm">{$_('account.folderStarred')}:</Label>
                      <Select.Root bind:value={starredFolderPath}>
                        <Select.Trigger class="h-9">
                          <Select.Value placeholder={$_('account.none')}>
                            {starredFolderPath || $_('account.none')}
                          </Select.Value>
                        </Select.Trigger>
                        <Select.Content>
                          <Select.Item value="" label={$_('account.none')} />
                          {#each availableFolders as f (f.path)}
                            <Select.Item value={f.path} label={f.path + (autoDetectedFolders.starred === f.path ? ' ' + $_('account.detected') : '')} />
                          {/each}
                        </Select.Content>
                      </Select.Root>
                    </div>
                  </div>
                {/if}
              </div>
            {/if}
          </div>

          <!-- Folder Sync Subscriptions -->
          <div class="space-y-2">
            <button
              type="button"
              class="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors disabled:cursor-not-allowed disabled:opacity-60"
              onclick={handleFolderSyncToggle}
              disabled={!editAccount}
            >
              <Icon
                icon={showFolderSync ? 'mdi:chevron-down' : 'mdi:chevron-right'}
                class="w-4 h-4"
              />
              <Icon icon="mdi:folder-sync-outline" class="w-4 h-4" />
              {$_('account.folderSync')}
              {#if !editAccount}
                <span class="text-xs text-muted-foreground">{$_('account.saveAccountFirst')}</span>
              {/if}
            </button>

            {#if showFolderSync && editAccount}
              <div class="space-y-4 pl-6 pt-2 border-l border-border ml-2">
                <div class="space-y-1">
                  <div class="flex items-center gap-3">
                    <Label>{$_('account.manageFolderSync')}</Label>
                    <Switch
                      bind:checked={syncFoldersEnabled}
                      onCheckedChange={(v) => { syncFoldersEnabled = v; if (v) loadSyncFolders() }}
                    />
                  </div>
                  <p class="text-xs text-muted-foreground">
                    {$_('account.manageFolderSyncHelp')}
                  </p>
                </div>

                {#if syncFoldersEnabled}
                  <div class="flex items-center gap-3">
                    <Label>{$_('account.syncAllFolders')}</Label>
                    <Switch
                      bind:checked={syncAllFolders}
                      onCheckedChange={handleSyncAllToggle}
                    />
                  </div>

                  {#if !syncAllFolders}
                    {#if loadingSyncFolders}
                      <div class="flex items-center gap-2 text-sm text-muted-foreground">
                        <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
                        {$_('account.loadingFolders')}
                      </div>
                    {:else}
                      <div class="space-y-1 max-h-48 overflow-y-auto">
                        {#each syncFolders as f (f.id)}
                          {@const isCore = coreFolderTypes.includes(f.type)}
                          <label class="flex items-center gap-2 cursor-pointer py-0.5 {isCore ? 'opacity-60' : ''}">
                            <input
                              type="checkbox"
                              checked={isCore || f.subscribed}
                              disabled={isCore}
                              onchange={(e) => handleFolderSubscriptionToggle(f, (e.target as HTMLInputElement).checked)}
                              class="rounded border-border"
                            />
                            <span class="text-sm truncate">{f.path}</span>
                            {#if isCore}
                              <span class="text-xs text-muted-foreground">({$_('account.alwaysSynced')})</span>
                            {/if}
                          </label>
                        {/each}
                      </div>
                    {/if}
                  {/if}
                {/if}
              </div>
            {/if}
          </div>

          <!-- Trusted Certificates -->
          <div class="space-y-2">
            <button
              type="button"
              class="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors disabled:cursor-not-allowed disabled:opacity-60"
              onclick={handleTrustedCertsToggle}
              disabled={!editAccount}
            >
              <Icon
                icon={showTrustedCerts ? 'mdi:chevron-down' : 'mdi:chevron-right'}
                class="w-4 h-4"
              />
              <Icon icon="mdi:shield-lock-outline" class="w-4 h-4" />
              {$_('account.trustedCertificates')}
              {#if !editAccount}
                <span class="text-xs text-muted-foreground">{$_('account.saveAccountFirst')}</span>
              {/if}
            </button>

            {#if showTrustedCerts && editAccount}
              <div class="space-y-3 pl-6 pt-2 border-l border-border ml-2">
                {#if loadingCerts}
                  <div class="flex items-center gap-2 text-sm text-muted-foreground">
                    <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
                    {$_('account.loadingCerts')}
                  </div>
                {:else if trustedCerts.length === 0}
                  <p class="text-sm text-muted-foreground">
                    {$_('account.noTrustedCerts')}
                  </p>
                {:else}
                  <div class="space-y-3">
                    {#each trustedCerts as cert, index (`${cert.host ?? ''}:${cert.fingerprint}:${index}`)}
                      <div class="flex items-start justify-between gap-3 rounded-lg border bg-muted/30 p-3">
                        <div class="space-y-1 min-w-0">
                          <div class="flex items-center gap-2">
                            <Icon icon="mdi:shield-check-outline" class="w-4 h-4 text-muted-foreground shrink-0" />
                            <span class="text-sm font-medium truncate">{cert.subject}</span>
                          </div>
                          <div class="text-xs text-muted-foreground space-y-0.5 pl-6">
                            <p>{$_('account.certFingerprint')} <span class="font-mono">{formatFingerprint(cert.fingerprint)}</span></p>
                            <p>{$_('account.certExpires')} {formatCertDate(cert.notAfter)}</p>
                          </div>
                        </div>
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          class="shrink-0 text-destructive hover:text-destructive hover:bg-destructive/10"
                          onclick={() => handleRemoveCert(cert.fingerprint)}
                        >
                          {$_('common.remove')}
                        </Button>
                      </div>
                    {/each}
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        </div>
      {/if}
    </div>

    <!-- Actions (pinned footer — stays visible while the fields scroll) -->
    <div class="flex items-center justify-between pt-4 mt-4 border-t border-border shrink-0">
        <Button
          type="button"
          variant="outline"
          onclick={handleTestConnection}
          disabled={testing || submitting}
        >
          {#if testing}
            <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
          {:else}
            <Icon icon="mdi:connection" class="w-4 h-4 mr-2" />
          {/if}
          {$_('account.testConnection')}
        </Button>

        <div class="flex gap-2">
          <Button type="button" variant="ghost" onclick={onCancel} disabled={submitting}>
            {createdInDialog ? $_('common.close') : $_('common.cancel')}
          </Button>
          <Button type="submit" disabled={submitting || testing}>
            {#if submitting}
              <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
            {/if}
            {editAccount ? $_('common.saveChanges') : $_('account.addAccount')}
          </Button>
        </div>
      </div>
  {/if}
</form>

<CertificateDialog
  bind:open={showCertDialog}
  certificate={pendingCertificate}
  onAcceptOnce={handleCertAcceptOnce}
  onAcceptPermanently={handleCertAcceptPermanently}
  onDecline={handleCertDecline}
/>

<!-- Connection test result popup (same one Settings → Accounts uses) -->
<ConnectionTestDialog bind:open={showTestConnDialog} {testing} result={testResult} />

<ConfirmDialog
  bind:open={showRemoveConfirm}
  title={$_('account.removeTrustedCert')}
  description={$_('account.removeTrustedCertDescription')}
  confirmLabel={$_('common.remove')}
  onConfirm={confirmRemoveCert}
/>
