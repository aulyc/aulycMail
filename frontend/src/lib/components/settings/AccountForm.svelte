<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import * as Select from '$lib/components/ui/select'
  import BoolSelect from '$lib/components/ui/bool-select/BoolSelect.svelte'
  import Switch from '$lib/components/ui/switch/Switch.svelte'
  import {
    providers,
    detectProvider,
    getCustomProvider,
    securityOptions,
    syncPeriodOptions,
    syncIntervalOptions,
    isOAuthProvider,
    allowsPasswordFallback,
    getOAuthProviderType,
    type EmailProvider,
    type OAuthProvider,
  } from '$lib/config/providers'
  import { oauthStore } from '$lib/stores/oauth.svelte'
  import { toasts } from '$lib/stores/toast'
  // @ts-ignore - wailsjs path
  import { account, certificate, app } from '../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import { GetAccountFoldersForMapping, GetAutoDetectedFolders, GetIdentities, AcceptCertificate, GetAllAccountIdentities } from '../../../../wailsjs/go/app/App'
  import CertificateDialog from './CertificateDialog.svelte'
  import ConnectionTestDialog from './ConnectionTestDialog.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { _ } from '$lib/i18n'

  // OAuth credentials to pass to parent
  export interface OAuthCredentials {
    provider: string
    accessToken: string
    refreshToken: string
    expiresIn: number
  }

  interface Props {
    /** Account to edit (null for new account) */
    editAccount?: account.Account | null
    /** Callback when form is submitted successfully */
    onSubmit?: (config: account.AccountConfig, oauthCredentials?: OAuthCredentials) => Promise<void>
    /** Callback when form is cancelled */
    onCancel?: () => void
    /** Callback for testing connection */
    onTestConnection?: (config: account.AccountConfig) => Promise<void>
  }

  let {
    editAccount = null,
    onSubmit,
    onCancel,
    onTestConnection: _onTestConnection,
  }: Props = $props()

  // Form state
  let step = $state<'provider' | 'details'>('details')
  let selectedProvider = $state<EmailProvider | null>(null)
  let showAdvanced = $state(false)

  // OAuth state
  let authMethod = $state<'password' | 'oauth2'>('password')
  let oauthConfigured = $state<Record<OAuthProvider, boolean>>({
    google: false,
    microsoft: false,
  })
  let oauthInitialized = $state(false)

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
  let smtpUseSameAsIncoming = $state(true)
  // Auto-mirror IMAP host into SMTP host for new Generic accounts: most
  // providers use the same hostname or a near-identical subdomain swap,
  // so typing the IMAP host pre-fills SMTP. Goes sticky-off the moment
  // the user types directly into the SMTP field — manual edits stick.
  let smtpHostMirrorsImap = $state(true)
  let replyForwardIdentityID = $state('')
  let availableIdentityGroups = $state<app.AccountIdentityGroup[]>([])
  // True only when the user explicitly picked Generic/Custom (or the
  // detector fell back to it). The "Same as incoming server" toggle is
  // gated on this; pre-configured providers always reuse IMAP creds.
  const isGenericProvider = $derived(selectedProvider?.id === 'custom' || selectedProvider?.id === 'generic')

  function handleSmtpUseSameAsIncomingChange(v: boolean) {
    smtpUseSameAsIncoming = v
    if (v) {
      smtpUsername = ''
      smtpPassword = ''
    }
  }
  let syncPeriodDays = $state<string>('180')
  let syncInterval = $state<string>('30') // Default: 30 minutes
  let readReceiptRequestPolicy = $state<string>('never')

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
      step = 'details'
      email = editAccount.email
      username = editAccount.username
      imapHost = editAccount.imapHost
      imapPort = editAccount.imapPort
      imapSecurity = editAccount.imapSecurity
      smtpHost = editAccount.smtpHost
      smtpPort = editAccount.smtpPort
      smtpSecurity = editAccount.smtpSecurity
      syncPeriodDays = String(editAccount.syncPeriodDays)
      // @ts-ignore - syncInterval from backend
      syncInterval = String(editAccount.syncInterval ?? 30)
      readReceiptRequestPolicy = editAccount.readReceiptRequestPolicy || 'never'
      // @ts-ignore - authType from backend
      authMethod = editAccount.authType === 'oauth2' ? 'oauth2' : 'password'
      // @ts-ignore - color from backend
      color = editAccount.color || ''

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
      showAdvanced = selectedProvider.id === 'custom'

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
    }
  }

  // Initialize OAuth configuration check
  $effect(() => {
    if (!oauthInitialized) {
      oauthInitialized = true
      checkOAuthConfiguration()
      // Initialize OAuth event listeners
      oauthStore.initEvents()
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

  // Update authMethod when OAuth configuration finishes loading.
  // If a user selects a provider before the async OAuth check completes,
  // this corrects the auth method once we know OAuth is available.
  // Does NOT depend on authMethod — otherwise clicking "App Password"
  // gets immediately reverted back to oauth2.
  $effect(() => {
    const _ = oauthConfigured
    if (!editAccount && selectedProvider && canUseOAuth(selectedProvider)) {
      authMethod = 'oauth2'
    }
  })

  // Check which OAuth providers are configured
  async function checkOAuthConfiguration() {
    try {
      const [googleConfigured, microsoftConfigured] = await Promise.all([
        oauthStore.isProviderConfigured('google'),
        oauthStore.isProviderConfigured('microsoft'),
      ])
      oauthConfigured = {
        google: googleConfigured,
        microsoft: microsoftConfigured,
      }
    } catch (err) {
      console.error('Failed to check OAuth configuration:', err)
    }
  }

  // Check if the selected provider supports OAuth and it's configured
  function canUseOAuth(provider: EmailProvider | null): boolean {
    if (!provider) return false
    if (!isOAuthProvider(provider)) return false
    const oauthType = getOAuthProviderType(provider)
    if (!oauthType) return false
    return oauthConfigured[oauthType] ?? false
  }

  // Start OAuth flow for the selected provider
  async function startOAuthFlow() {
    if (!selectedProvider) return
    const oauthType = getOAuthProviderType(selectedProvider)
    if (!oauthType) return

    try {
      await oauthStore.startFlow(oauthType)
    } catch (err) {
      console.error('Failed to start OAuth flow:', err)
    }
  }

  // Cancel OAuth flow
  function cancelOAuthFlow() {
    oauthStore.cancelFlow()
  }

  // Copy-link fallback for OAuth waiting state — used when the browser fails
  // to open and the user needs to paste the URL manually.
  let oauthLinkCopied = $state(false)
  let oauthCopiedResetTimer: ReturnType<typeof setTimeout> | null = null
  async function handleCopyOAuthLink() {
    if (!oauthStore.authURL) return
    try {
      await navigator.clipboard.writeText(oauthStore.authURL)
      oauthLinkCopied = true
      if (oauthCopiedResetTimer) clearTimeout(oauthCopiedResetTimer)
      oauthCopiedResetTimer = setTimeout(() => { oauthLinkCopied = false }, 1500)
    } catch {
      toasts.error($_('viewer.failedToCopy'))
    }
  }

  // Get OAuth button text based on provider
  function getOAuthButtonText(provider: EmailProvider | null): string {
    if (!provider) return $_('account.signIn')
    const oauthType = getOAuthProviderType(provider)
    if (oauthType === 'google') return $_('account.signInWith', { values: { provider: 'Google' } })
    if (oauthType === 'microsoft') return $_('account.signInWith', { values: { provider: 'Microsoft' } })
    return $_('account.signIn')
  }

  // Get OAuth button icon based on provider
  function getOAuthButtonIcon(provider: EmailProvider | null): string {
    if (!provider) return 'mdi:login'
    const oauthType = getOAuthProviderType(provider)
    if (oauthType === 'google') return 'logos:google-icon'
    if (oauthType === 'microsoft') return 'logos:microsoft-icon'
    return 'mdi:login'
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

    // Set auth method based on provider and configuration
    if (canUseOAuth(provider)) {
      authMethod = 'oauth2'
    } else {
      authMethod = 'password'
    }

    // Show advanced for custom provider
    showAdvanced = provider.id === 'custom'
    step = 'details'
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
      password: authMethod === 'oauth2' ? '' : password,
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
      authType: authMethod,
      syncPeriodDays: Number(syncPeriodDays),
      syncInterval: Number(syncInterval),
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

    // Password is only required for password auth on new accounts
    if (authMethod === 'password' && !password && !editAccount) {
      errors.password = $_('account.passwordRequired')
    }

    // For OAuth, check that the flow completed successfully
    if (authMethod === 'oauth2' && !editAccount && !oauthStore.isFlowSuccess) {
      errors.oauth = $_('account.pleaseCompleteSignIn')
    }

    if (!imapHost.trim()) errors.imapHost = $_('account.imapHostRequired')
    if (imapPort < 1 || imapPort > 65535) errors.imapPort = $_('account.invalidPort')
    // SMTP host/port checks only when the user wants outgoing.
    if (!noOutgoingServer) {
      if (!smtpHost.trim()) errors.smtpHost = $_('account.smtpHostRequired')
      if (smtpPort < 1 || smtpPort > 65535) errors.smtpPort = $_('account.invalidPort')
    }
    // Separate SMTP credentials (Generic only, toggle off): username
    // always required; password required on NEW accounts. Blank on EDIT
    // is "keep existing keyring entry."
    if (!noOutgoingServer && isGenericProvider && !smtpUseSameAsIncoming) {
      if (!smtpUsername.trim()) errors.smtpUsername = $_('account.usernameRequired')
      if (!editAccount && !smtpPassword) errors.smtpPassword = $_('account.passwordRequired')
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
      const result = await accountStore.testConnection(buildConfig())
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
      // Build OAuth credentials if using OAuth
      let oauthCredentials: OAuthCredentials | undefined
      if (authMethod === 'oauth2' && oauthStore.isFlowSuccess && oauthStore.flowResult) {
        // Note: The actual tokens are stored in the backend during the OAuth flow
        // We just pass the metadata so the parent can complete the account setup
        oauthCredentials = {
          provider: oauthStore.flowResult.provider,
          accessToken: '', // Tokens are handled by backend
          refreshToken: '', // Tokens are handled by backend
          expiresIn: oauthStore.flowResult.expiresIn,
        }
      }

      await onSubmit?.(buildConfig(), oauthCredentials)

      // Reset OAuth state on success
      if (authMethod === 'oauth2') {
        oauthStore.reset()
      }
    } catch (err) {
      console.error('Account save failed:', err)
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
              class="w-64 shrink-0 {errors.email ? 'border-destructive' : ''}"
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

        <!-- Authentication Section -->
        <div class="space-y-3">
          {#if canUseOAuth(selectedProvider) && !editAccount}
            <!-- OAuth Provider - Show Sign In Button -->
            <div class="space-y-3">
              <Label>{$_('account.authentication')}</Label>

              {#if allowsPasswordFallback(selectedProvider!)}
                <!-- Provider allows both OAuth and password -->
                <div class="flex gap-2">
                  <Button
                    type="button"
                    variant={authMethod === 'oauth2' ? 'default' : 'outline'}
                    size="sm"
                    onclick={() => authMethod = 'oauth2'}
                    class="flex-1"
                  >
                    <Icon icon={getOAuthButtonIcon(selectedProvider)} class="w-4 h-4 mr-2" />
                    OAuth
                  </Button>
                  <Button
                    type="button"
                    variant={authMethod === 'password' ? 'default' : 'outline'}
                    size="sm"
                    onclick={() => authMethod = 'password'}
                    class="flex-1"
                  >
                    <Icon icon="mdi:key" class="w-4 h-4 mr-2" />
                    {$_('account.appPassword')}
                  </Button>
                </div>
              {/if}

              {#if authMethod === 'oauth2'}
                <!-- OAuth Flow UI -->
                <div class="rounded-lg border border-border p-4 space-y-3">
                  {#if oauthStore.flowState === 'idle' || oauthStore.flowState === 'cancelled'}
                    <!-- Initial state - show sign in button -->
                    <Button
                      type="button"
                      variant="outline"
                      class="w-full h-12"
                      onclick={startOAuthFlow}
                    >
                      <Icon icon={getOAuthButtonIcon(selectedProvider)} class="w-5 h-5 mr-3" />
                      {getOAuthButtonText(selectedProvider)}
                    </Button>
                    <p class="text-xs text-muted-foreground text-center">
                      {$_('account.redirectToSignIn')}
                    </p>
                  {:else if oauthStore.flowState === 'pending'}
                    <!-- Waiting for OAuth callback -->
                    <div class="flex flex-col items-center gap-3 py-2">
                      <Icon icon="mdi:loading" class="w-8 h-8 animate-spin text-primary" />
                      <div class="text-center">
                        <p class="text-sm font-medium">{$_('account.waitingForAuth')}</p>
                        <p class="text-xs text-muted-foreground mt-1">
                          {$_('account.completeSignIn')}
                        </p>
                      </div>
                      {#if oauthStore.authURL}
                        <button
                          type="button"
                          class="text-xs text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 transition-colors"
                          onclick={handleCopyOAuthLink}
                        >
                          {oauthLinkCopied ? $_('account.linkCopied') : $_('viewer.copyLink')}
                          <Icon icon={oauthLinkCopied ? 'mdi:check' : 'mdi:content-copy'} class="w-3.5 h-3.5" />
                        </button>
                      {/if}
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onclick={cancelOAuthFlow}
                      >
                        {$_('common.cancel')}
                      </Button>
                    </div>
                  {:else if oauthStore.flowState === 'success'}
                    <!-- OAuth completed successfully -->
                    <div class="flex items-center gap-3 py-2">
                      <div class="flex-shrink-0 w-10 h-10 rounded-full bg-green-500/10 flex items-center justify-center">
                        <Icon icon="mdi:check" class="w-5 h-5 text-green-500" />
                      </div>
                      <div class="flex-1 min-w-0">
                        <p class="text-sm font-medium text-green-600 dark:text-green-400">
                          {$_('account.connectedSuccessfully')}
                        </p>
                        <p class="text-xs text-muted-foreground truncate">
                          {oauthStore.flowResult?.email}
                        </p>
                      </div>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onclick={() => {
                          oauthStore.reset()
                        }}
                      >
                        <Icon icon="mdi:refresh" class="w-4 h-4" />
                      </Button>
                    </div>
                  {:else if oauthStore.flowState === 'error'}
                    <!-- OAuth failed -->
                    <div class="space-y-3">
                      <div class="flex items-start gap-3">
                        <div class="flex-shrink-0 w-10 h-10 rounded-full bg-destructive/10 flex items-center justify-center">
                          <Icon icon="mdi:alert" class="w-5 h-5 text-destructive" />
                        </div>
                        <div class="flex-1">
                          <p class="text-sm font-medium text-destructive">
                            {$_('account.authFailed')}
                          </p>
                          <p class="text-xs text-muted-foreground mt-1">
                            {$_('account.authFailed')}
                          </p>
                        </div>
                      </div>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        class="w-full"
                        onclick={startOAuthFlow}
                      >
                        {$_('account.tryAgain')}
                      </Button>
                    </div>
                  {/if}
                </div>
                {#if errors.oauth}
                  <p class="text-sm text-destructive">{errors.oauth}</p>
                {/if}
              {:else}
                <!-- Password field for app password -->
                <div>
                  <div class="flex items-center justify-between gap-4">
                    <Label for="password">{$_('account.appPassword')}</Label>
                    <Input
                      id="password"
                      type="password"
                      placeholder={$_('account.enterAppPassword')}
                      bind:value={password}
                      class="w-64 shrink-0 {errors.password ? 'border-destructive' : ''}"
                    />
                  </div>
                  {#if errors.password}
                    <p class="text-sm text-destructive mt-1">{errors.password}</p>
                  {/if}
                </div>
              {/if}
            </div>
          {:else if editAccount && editAccount.authType === 'oauth2'}
            <!-- Editing an OAuth account -->
            <div class="space-y-2">
              <Label>{$_('account.authentication')}</Label>
              <div class="rounded-lg border border-border p-4">
                <div class="flex items-center gap-3">
                  <div class="flex-shrink-0 w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center">
                    <Icon icon={getOAuthButtonIcon(selectedProvider)} class="w-5 h-5" />
                  </div>
                  <div class="flex-1">
                    <p class="text-sm font-medium">{$_('account.oauthConnected')}</p>
                    <p class="text-xs text-muted-foreground">
                      {$_('account.signInAgainHelp')}
                    </p>
                  </div>
                </div>
              </div>
            </div>
          {:else}
            <!-- Standard password field -->
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
                      smtpHost = v
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

          <!-- Outgoing server (SMTP). Header carries the no-outgoing toggle
               (help text is a tooltip on the info icon). -->
          <div class="space-y-4">
            <div class="flex items-center justify-between gap-2">
              <span class="text-sm font-medium">{$_('account.outgoingServer')}</span>
              <div class="flex items-center gap-2">
                <Switch bind:checked={noOutgoingServer} />
                <span class="text-sm text-muted-foreground">{$_('account.noOutgoingServer')}</span>
                <span class="cursor-help text-muted-foreground" title={$_('account.noOutgoingServerHelp')}>
                  <Icon icon="mdi:information-outline" class="w-4 h-4" />
                </span>
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
                <!-- Label hidden (the section header already reads 发件服务器);
                     kept for vertical alignment with Port/Security labels. -->
                <Label for="smtpHost" class="invisible" aria-hidden="true">{$_('account.outgoingServer')}</Label>
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
            {$_('common.cancel')}
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
