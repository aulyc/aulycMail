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
    localRetentionOptions,
    syncStrategyOptions,
    fullCheckIntervalOptions,
    bodyDownloadOptions,
    syncIntervalOptions,
    type EmailProvider,
  } from '$lib/config/providers'
  // @ts-ignore - wailsjs path
  import { account, certificate, app, folder } from '../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import { GetAccountFoldersForMapping, GetAutoDetectedFolders, GetIdentities, AcceptCertificate, GetAllAccountIdentities, GetTrustedCertificates, RemoveTrustedCertificate, GetFolders, SubscribeFolder, UnsubscribeFolder, SubscribeAllFolders, ClearOfflineBodyCache } from '../../../../wailsjs/go/app/App'
  import CertificateDialog from './CertificateDialog.svelte'
  import ConnectionTestDialog from './ConnectionTestDialog.svelte'
  import AccountIdentityTab from './account/AccountIdentityTab.svelte'
  import ConfirmDialog from '$lib/components/ui/confirm-dialog/ConfirmDialog.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { _ } from '$lib/i18n'
  import { isEmailAddress } from '$lib/utils/email'
  import { formatLocalDate } from '$lib/utils/date'

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
  let localRetentionDays = $state<string>('0')
  let syncStrategy = $state<string>('incremental')
  let fullCheckIntervalDays = $state<string>('7')
  let bodyDownloadPolicy = $state<string>('on_demand')
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

  function getLocalRetentionLabel(value: string): string {
    const numValue = Number(value)
    const option = localRetentionOptions.find(opt => opt.value === numValue)
    return option ? $_(option.labelKey) : `${value} days`
  }

  function getSyncStrategyLabel(value: string): string {
    const option = syncStrategyOptions.find(opt => opt.value === value)
    return option ? $_(option.labelKey) : value
  }

  function getFullCheckIntervalLabel(value: string): string {
    const numValue = Number(value)
    const option = fullCheckIntervalOptions.find(opt => opt.value === numValue)
    return option ? $_(option.labelKey) : `${value} days`
  }

  function getBodyDownloadLabel(value: string): string {
    const option = bodyDownloadOptions.find(opt => opt.value === value)
    return option ? $_(option.labelKey) : value
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

  type FolderMappingKey = 'sent' | 'drafts' | 'trash' | 'spam' | 'archive' | 'allMail' | 'starred'
  type FolderMappingPathField =
    | 'sentFolderPath'
    | 'draftsFolderPath'
    | 'trashFolderPath'
    | 'spamFolderPath'
    | 'archiveFolderPath'
    | 'allMailFolderPath'
    | 'starredFolderPath'
  type FolderMappingConfig = {
    key: FolderMappingKey
    labelKey: string
    pathField: FolderMappingPathField
    detectedKey: string
  }

  const folderMappingConfigs: FolderMappingConfig[] = [
    { key: 'sent', labelKey: 'account.folderSent', pathField: 'sentFolderPath', detectedKey: 'sent' },
    { key: 'drafts', labelKey: 'account.folderDrafts', pathField: 'draftsFolderPath', detectedKey: 'drafts' },
    { key: 'trash', labelKey: 'account.folderTrash', pathField: 'trashFolderPath', detectedKey: 'trash' },
    { key: 'spam', labelKey: 'account.folderSpam', pathField: 'spamFolderPath', detectedKey: 'spam' },
    { key: 'archive', labelKey: 'account.folderArchive', pathField: 'archiveFolderPath', detectedKey: 'archive' },
    { key: 'allMail', labelKey: 'account.folderAllMail', pathField: 'allMailFolderPath', detectedKey: 'all' },
    { key: 'starred', labelKey: 'account.folderStarred', pathField: 'starredFolderPath', detectedKey: 'starred' },
  ]

  let folderMappingPaths = $state<Record<FolderMappingKey, string>>({
    sent: '',
    drafts: '',
    trash: '',
    spam: '',
    archive: '',
    allMail: '',
    starred: '',
  })

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
  let showClearOfflineBodyCacheConfirm = $state(false)
  let clearingOfflineBodyCache = $state(false)
  let offlineBodyCacheStatus = $state<{ kind: 'success' | 'error'; message: string } | null>(null)

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
      initializeFolderMappings(editAccount, autoDetectedFolders)
    } catch (err) {
      console.error('Failed to load folders for mapping:', err)
    } finally {
      loadingFolders = false
    }
  }

  function initializeFolderMappings(acc: account.Account, detected: Record<string, string> = {}) {
    for (const config of folderMappingConfigs) {
      folderMappingPaths[config.key] = acc[config.pathField] || detected[config.detectedKey] || ''
    }
  }

  function getFolderOptionLabel(path: string, config: FolderMappingConfig): string {
    return path + (autoDetectedFolders[config.detectedKey] === path ? ' ' + $_('account.detected') : '')
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

  async function confirmClearOfflineBodyCache() {
    if (!editAccount) return
    clearingOfflineBodyCache = true
    offlineBodyCacheStatus = null
    try {
      const result = await ClearOfflineBodyCache(editAccount.id)
      offlineBodyCacheStatus = {
        kind: 'success',
        message: $_('account.offlineBodyCacheCleared', { values: { count: result?.bodiesCleared ?? 0 } }),
      }
    } catch (err) {
      console.error('Failed to clear offline body cache:', err)
      offlineBodyCacheStatus = {
        kind: 'error',
        message: $_('account.offlineBodyCacheClearFailed'),
      }
    } finally {
      clearingOfflineBodyCache = false
    }
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
      return formatLocalDate(iso)
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
      // @ts-ignore - new sync fields are present after Wails bindings regenerate
      localRetentionDays = String(editAccount.localRetentionDays ?? editAccount.syncPeriodDays ?? 0)
      // @ts-ignore - new sync fields are present after Wails bindings regenerate
      syncStrategy = editAccount.syncStrategy || 'incremental'
      // @ts-ignore - new sync fields are present after Wails bindings regenerate
      fullCheckIntervalDays = String(editAccount.fullCheckIntervalDays ?? 7)
      // @ts-ignore - new sync fields are present after Wails bindings regenerate
      bodyDownloadPolicy = editAccount.bodyDownloadPolicy || 'on_demand'
      // @ts-ignore - syncInterval from backend
      syncInterval = String(editAccount.syncInterval ?? 30)
      syncAllFolders = editAccount.syncAllFolders || false
      syncFoldersEnabled = editAccount.syncFoldersEnabled || false
      readReceiptRequestPolicy = editAccount.readReceiptRequestPolicy || 'never'
      // @ts-ignore - color from backend
      color = editAccount.color || ''
      displayNameLoaded = false

      // Initialize folder mappings; auto-detected fallbacks are filled when the section expands.
      initializeFolderMappings(editAccount)

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
      syncPeriodDays: Number(localRetentionDays),
      localRetentionDays: Number(localRetentionDays),
      syncStrategy,
      fullCheckIntervalDays: Number(fullCheckIntervalDays),
      bodyDownloadPolicy,
      bodyDownloadDays: 180,
      syncInterval: Number(syncInterval),
      syncAllFolders,
      syncFoldersEnabled,
      readReceiptRequestPolicy,
      // Folder mappings
      sentFolderPath: folderMappingPaths.sent,
      draftsFolderPath: folderMappingPaths.drafts,
      trashFolderPath: folderMappingPaths.trash,
      spamFolderPath: folderMappingPaths.spam,
      archiveFolderPath: folderMappingPaths.archive,
      allMailFolderPath: folderMappingPaths.allMail,
      starredFolderPath: folderMappingPaths.starred,
    })
  }

  // Validate form
  function validate(): boolean {
    errors = {}

    if (!displayName.trim()) errors.displayName = $_('account.displayNameRequired')
    if (!email.trim()) errors.email = $_('account.emailRequired')
    else if (!isEmailAddress(email)) errors.email = $_('account.invalidEmail')

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
            <Label for="displayName">{$_('account.displayName')}</Label>
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
          <Label for="username">{$_('account.username')}</Label>
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

          <!-- Sender addresses. Requires a persisted account because aliases are
               stored separately from AccountConfig and need accountId. -->
          <div class="pt-1">
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
                <Label>{$_('identity.emailAddresses')}</Label>
                <Button type="button" variant="outline" size="sm" disabled>
                  <Icon icon="mdi:email-multiple-outline" class="w-4 h-4 mr-1" />
                  {$_('identity.addEmailAddress')}
                </Button>
              </div>
            {/if}
          </div>

          <!-- Divider -->
          <div class="border-t border-border"></div>

          <!-- Sync settings (label + control on one row) -->
          <div class="flex items-center justify-between gap-4">
            <Label>{$_('account.localRetention')}</Label>
            <Select.Root bind:value={localRetentionDays}>
              <Select.Trigger class="w-48 shrink-0">
                <Select.Value placeholder="Select">
                  {getLocalRetentionLabel(localRetentionDays)}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                {#each localRetentionOptions as opt (opt.value)}
                  <Select.Item value={String(opt.value)} label={$_(opt.labelKey)} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>

          <div class="flex items-center justify-between gap-4">
            <Label>{$_('account.syncStrategy')}</Label>
            <Select.Root bind:value={syncStrategy}>
              <Select.Trigger class="w-48 shrink-0">
                <Select.Value placeholder="Select">
                  {getSyncStrategyLabel(syncStrategy)}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                {#each syncStrategyOptions as opt (opt.value)}
                  <Select.Item value={opt.value} label={$_(opt.labelKey)} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>

          {#if syncStrategy === 'incremental'}
            <div class="flex items-center justify-between gap-4">
              <Label>{$_('account.fullCheckInterval')}</Label>
              <Select.Root bind:value={fullCheckIntervalDays}>
                <Select.Trigger class="w-48 shrink-0">
                  <Select.Value placeholder="Select">
                    {getFullCheckIntervalLabel(fullCheckIntervalDays)}
                  </Select.Value>
                </Select.Trigger>
                <Select.Content>
                  {#each fullCheckIntervalOptions as opt (opt.value)}
                    <Select.Item value={String(opt.value)} label={$_(opt.labelKey)} />
                  {/each}
                </Select.Content>
              </Select.Root>
            </div>
          {/if}

          <div class="flex items-center justify-between gap-4">
            <Label>{$_('account.bodyDownload')}</Label>
            <Select.Root bind:value={bodyDownloadPolicy}>
              <Select.Trigger class="w-48 shrink-0">
                <Select.Value placeholder="Select">
                  {getBodyDownloadLabel(bodyDownloadPolicy)}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                {#each bodyDownloadOptions as opt (opt.value)}
                  <Select.Item value={opt.value} label={$_(opt.labelKey)} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>

          {#if editAccount}
            <div class="flex items-center justify-between gap-4">
              <div class="min-w-0">
                <Label>{$_('account.offlineBodyCache')}</Label>
                {#if offlineBodyCacheStatus}
                  <p
                    class="mt-1 truncate text-xs {offlineBodyCacheStatus.kind === 'error' ? 'text-destructive' : 'text-muted-foreground'}"
                    title={offlineBodyCacheStatus.message}
                  >
                    {offlineBodyCacheStatus.message}
                  </p>
                {/if}
              </div>
              <Button
                type="button"
                variant="outline"
                size="sm"
                class="w-48 shrink-0"
                onclick={() => showClearOfflineBodyCacheConfirm = true}
                disabled={clearingOfflineBodyCache || submitting}
              >
                {#if clearingOfflineBodyCache}
                  <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
                {:else}
                  <Icon icon="mdi:database-remove-outline" class="w-4 h-4 mr-2" />
                {/if}
                {$_('account.clearOfflineBodyCache')}
              </Button>
            </div>
          {/if}

          <div class="flex items-center justify-between gap-4">
            <Label>{$_('account.checkNewMail')}</Label>
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
            <Label>{$_('account.requestReadReceipts')}</Label>
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
                {#if loadingFolders}
                  <div class="flex items-center gap-2 text-sm text-muted-foreground">
                    <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
                    {$_('account.loadingFolders')}
                  </div>
                {:else if availableFolders.length === 0}
                  <p class="text-sm text-muted-foreground">{$_('account.noFoldersAvailable')}</p>
                {:else}
                  <div class="grid gap-3">
                    {#each folderMappingConfigs as mapping (mapping.key)}
                      <div class="grid grid-cols-[100px_1fr] items-center gap-2">
                        <Label class="text-sm">{$_(mapping.labelKey)}:</Label>
                        <Select.Root bind:value={folderMappingPaths[mapping.key]}>
                          <Select.Trigger class="h-9">
                            <Select.Value placeholder={$_('account.none')}>
                              {folderMappingPaths[mapping.key] || $_('account.none')}
                            </Select.Value>
                          </Select.Trigger>
                          <Select.Content>
                            <Select.Item value="" label={$_('account.none')} />
                            {#each availableFolders as f (f.path)}
                              <Select.Item value={f.path} label={getFolderOptionLabel(f.path, mapping)} />
                            {/each}
                          </Select.Content>
                        </Select.Root>
                      </div>
                    {/each}
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

<ConfirmDialog
  bind:open={showClearOfflineBodyCacheConfirm}
  title={$_('account.clearOfflineBodyCache')}
  description={$_('account.clearOfflineBodyCacheDescription')}
  confirmLabel={$_('account.clearOfflineBodyCache')}
  cancelLabel={$_('common.cancel')}
  variant="destructive"
  loading={clearingOfflineBodyCache}
  onConfirm={confirmClearOfflineBodyCache}
/>
