<script lang="ts">
  // Load offline icon data before anything else
  import './lib/iconify-offline'

  import { onMount, tick, untrack } from 'svelte'
  import Sidebar from './lib/components/sidebar/Sidebar.svelte'
  import MessageList from './lib/components/list/MessageList.svelte'
  import ConversationViewer from './lib/components/viewer/ConversationViewer.svelte'
  import Composer from './lib/components/composer/Composer.svelte'
  import StatusBar from './lib/components/status/StatusBar.svelte'
  import TermsDialog from './lib/components/TermsDialog.svelte'
  import CertificateDialog from './lib/components/settings/CertificateDialog.svelte'
  import ActivityRail from './lib/components/rail/ActivityRail.svelte'
  import SettingsDialog from './lib/components/settings/SettingsDialog.svelte'
  import SearchOverlay from './lib/components/SearchOverlay.svelte'
  import BackupViewerDialog from './lib/components/backup/BackupViewerDialog.svelte'
  import KeyboardActionMenu from './lib/components/keyboard/KeyboardActionMenu.svelte'
  import { activateContactFromGlobalSearch } from '$contacts/stores/contactsView.svelte'
  import ContactsPane from '$contacts/components/ContactsPane.svelte'
  import { preloadContactAccountGroups } from '$contacts/stores/contactAccountGroups.svelte'
  import { handleGlobalShortcut } from '$lib/keyboard/globalShortcuts'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { addToast } from '$lib/stores/toast'
  import { loadSettings, getThemeMode } from '$lib/stores/settings.svelte'
  import { loadImageAllowlist } from '$lib/stores/imageAllowlist.svelte'
  import { initTheme, applyThemeFromMode, handleSystemThemeEvent, handleMediaQueryChange } from '$lib/stores/theme.svelte'
  import { DEFAULT_LIST_WIDTH, DEFAULT_SIDEBAR_WIDTH, loadUIState, saveUIState, getActivePane, setActivePane } from '$lib/stores/uiState.svelte'
  import { shouldClearRestoredFolderSelection } from '$lib/stores/restoredFolderSelection'
  import { backupStatistics } from '$lib/backup/backupStatistics'
  import { normalizeExternalOpenFiles, toExternalSmtpAttachment } from '$lib/externalFileCompose'
  import {
    type FocusablePane,
    getFocusedPane,
    isMainKeyboardScope,
    setFocusedPane,
    setComposerOpen,
  } from '$lib/stores/keyboard.svelte'
  import { initLayout, getLayoutMode, getResponsiveView, showViewer, hideViewer, showSidebar, hideSidebar, isResponsive } from '$lib/stores/layout.svelte'
  import { archiveMessages, setReadStateMessages, toggleSpamMessages, toggleStarMessages, undoLastMailAction } from '$lib/mailActions'
  // @ts-ignore - wailsjs path
  import { PrepareReply, GetPendingMailto, GetDraft, GetTermsAccepted, SetTermsAccepted, RefreshWindowConstraints, AcceptCertificate, GetStartHiddenActive, QuitApp, GetSystemTheme, NotifyStartupComplete, ReadFileAsAttachment } from '../wailsjs/go/app/App.js'
  // @ts-ignore - wailsjs path
  import { smtp, folder, certificate } from '../wailsjs/go/models'
  // @ts-ignore - wailsjs runtime
  import { WindowShow, WindowHide, EventsOn, WindowSetMinSize } from '../wailsjs/runtime/runtime'
  import { _ } from '$lib/i18n'
  import { keyboardActionMenu } from '$lib/stores/keyboardActionMenu.svelte'

  // Component refs for keyboard navigation. Plain `let` (not $state) is
  // intentional: svelte-check warns "Changing its value will not correctly
  // trigger updates" but nothing here actually reads these refs in a reactive
  // context — they're only used inside event handlers. Making them $state
  // added bookkeeping cost (visible in idle-CPU profiling) without any benefit.
  let sidebarRef: Sidebar | null = null
  let messageListRef: MessageList | null = null
  let viewerRef: ConversationViewer | null = null
  let activityRailRef: ActivityRail | null = null
  let messageListContainerRef: HTMLElement | null = null

  // React to theme mode changes from settings store
  $effect(() => {
    const mode = getThemeMode()
    applyThemeFromMode(mode)
  })

  // Selected folder state
  let selectedAccountId = $state<string | null>(null)
  let selectedFolderId = $state<string | null>(null)
  let selectedFolderName = $state('Inbox')
  let selectedFolderType = $state<string | null>(null)
  // Track where the selection came from: 'account' for account tree
  let selectionSource = $state<'account' | null>(null)

  // Selected conversation state
  let selectedThreadId = $state<string | null>(null)
  let selectedConversationFolderId = $state<string | null>(null)
  let selectedConversationAccountId = $state<string | null>(null)

  // Composer state
  let showComposer = $state(false)
  let composerAccountId = $state<string | null>(null)
  let composerInitialMessage = $state<smtp.ComposeMessage | null>(null)
  let composerDraftId = $state<string | null>(null)
  let composerImagesLoaded = $state(false)
  let externalFileComposeBusy = false
  let pendingExternalFileBatches: string[][] = []

  // Mirror composer visibility into the keyboard store so the viewer can
  // suppress its Delete/Backspace shortcut during the composer's mount→focus race.
  $effect(() => {
    setComposerOpen(showComposer)
  })

  // Focus mode state — viewer (or single message) takes the whole window.
  // Always resets on conversation change, on Esc, on back-arrow, and on app reload.
  let focusMode = $state<'off' | 'thread' | 'message'>('off')
  let focusedMessageIdInFocus = $state<string | null>(null)
  const viewerIsOverlay = $derived(isResponsive() || focusMode !== 'off')
  const viewerIsVisible = $derived(getResponsiveView() === 'viewer' || focusMode !== 'off')

  function toggleThreadFocus() {
    if (focusMode === 'thread') {
      focusMode = 'off'
      focusedMessageIdInFocus = null
      return
    }
    focusMode = 'thread'
    focusedMessageIdInFocus = null
  }

  // Auto-reset focus mode when the conversation changes (or is closed).
  // Prevents focus state from leaking across navigation.
  $effect(() => {
    void selectedThreadId
    focusMode = 'off'
    focusedMessageIdInFocus = null
  })

  // Route keyboard pane focus to the viewer while in focus mode so j/k/arrow
  // shortcuts scroll the viewer, then restore the prior pane on exit.
  let focusedPaneBeforeFocusMode: FocusablePane | null = null
  $effect(() => {
    const mode = focusMode
    if (mode !== 'off' && focusedPaneBeforeFocusMode === null) {
      focusedPaneBeforeFocusMode = untrack(() => getFocusedPane())
      setFocusedPane('viewer')
      return
    }
    if (mode === 'off' && focusedPaneBeforeFocusMode !== null) {
      setFocusedPane(focusedPaneBeforeFocusMode)
      focusedPaneBeforeFocusMode = null
    }
  })

  // Shutdown state
  let isShuttingDown = $state(false)

  // Terms acceptance state
  let showTermsDialog = $state(false)

  // Settings dialog — hosted at app level so the rail's gear opens it from
  // any view (Mail or Contacts), not just the mail sidebar.
  type SettingsPage = 'general' | 'appearance' | 'mail' | 'accounts' | 'backup' | 'activity' | 'about'
  let showSettings = $state(false)
  let settingsPage = $state<SettingsPage>('general')
  let showSearchOverlay = $state(false)
  let showBackupViewer = $state(false)

  function openSettings(page: SettingsPage = 'general') {
    settingsPage = page
    showBackupViewer = false
    activityRailRef?.selectSettingsEntry()
    showSettings = true
  }

  function handleSettingsClosed() {
    showSettings = false
    requestAnimationFrame(() => activityRailRef?.focusSettings())
  }

  // Certificate TOFU state (for background sync cert errors)
  let showCertDialog = $state(false)
  let pendingCertificate = $state<certificate.CertificateInfo | null>(null)
  let pendingCertAccountId = $state<string | null>(null)

  // Handle forced quit (Ctrl+Q) — always quits regardless of background mode.
  // Quit immediately so it feels as snappy as any other macOS app.
  function handleQuit() {
    isShuttingDown = true
    QuitApp()
  }

  // Handle terms acceptance
  async function handleTermsAccepted() {
    try {
      await SetTermsAccepted(true)
      showTermsDialog = false
    } catch (err) {
      console.error('Failed to save terms acceptance:', err)
    }
  }


  // Certificate TOFU handlers for background sync
  async function handleBgCertAcceptOnce() {
    if (!pendingCertificate || !pendingCertAccountId) return
    try {
      // Look up the account's IMAP host for the accept call
      const acc = accountStore.accounts.find(a => a.account.id === pendingCertAccountId)
      const host = acc?.account.imapHost || ''
      await AcceptCertificate(host, pendingCertificate, false)
    } catch (err) {
      console.error('Failed to accept certificate:', err)
    }
    showCertDialog = false
    pendingCertificate = null
    pendingCertAccountId = null
  }

  async function handleBgCertAcceptPermanently() {
    if (!pendingCertificate || !pendingCertAccountId) return
    try {
      const acc = accountStore.accounts.find(a => a.account.id === pendingCertAccountId)
      const host = acc?.account.imapHost || ''
      await AcceptCertificate(host, pendingCertificate, true)
    } catch (err) {
      console.error('Failed to accept certificate:', err)
    }
    showCertDialog = false
    pendingCertificate = null
    pendingCertAccountId = null
  }

  function handleBgCertDecline() {
    showCertDialog = false
    pendingCertificate = null
    pendingCertAccountId = null
  }

  // Helper to find folder info by ID from account store
  function findFolderById(accountId: string, folderId: string): { name: string; type: string; path: string; noSelect: boolean } | null {
    const acc = accountStore.accounts.find(a => a.account.id === accountId)
    if (!acc) return null

    function searchTree(trees: folder.FolderTree[]): { name: string; type: string; path: string; noSelect: boolean } | null {
      for (const tree of trees) {
        if (tree.folder?.id === folderId) {
          return {
            name: tree.folder.name,
            type: tree.folder.type,
            path: tree.folder.path,
            noSelect: tree.folder.noSelect === true,
          }
        }
        if (tree.children) {
          const found = searchTree(tree.children)
          if (found) return found
        }
      }
      return null
    }

    // Check if folders are loaded before searching
    if (!acc.folders || acc.folders.length === 0) return null
    return searchTree(acc.folders)
  }

  // A folder can become hierarchy-only after the first live IMAP LIST following
  // an upgrade. Drop a persisted/stale selection as soon as that fact is known.
  $effect(() => {
    if (!selectedAccountId || !selectedFolderId || selectionSource !== 'account') return
    const accountState = accountStore.accounts.find(a => a.account.id === selectedAccountId)
    const selected = findFolderById(selectedAccountId, selectedFolderId)
    const foldersLoaded = !accountStore.loading && accountState?.loading !== true
    if (!shouldClearRestoredFolderSelection(Boolean(accountState), foldersLoaded, selected)) return

    selectedAccountId = null
    selectedFolderId = null
    selectedFolderName = 'Inbox'
    selectedFolderType = null
    selectionSource = null
    selectedThreadId = null
    selectedConversationFolderId = null
    selectedConversationAccountId = null
    accountStore.selectedFolder = null
    saveUIState({
      selectedAccountId: null,
      selectedFolderId: null,
      selectedFolderName: 'Inbox',
      selectedFolderType: null,
      selectedThreadId: null,
      selectedConversationAccountId: null,
      selectedConversationFolderId: null,
    })
  })

  // Cmd/Ctrl+A inside any text field selects that field's text. Registered on
  // document in the CAPTURE phase so it fires before bits-ui dialogs (which can
  // stop keydown propagation and otherwise swallow it). The custom macOS menu
  // has no Select All, so without this the browser default never happens.
  function handleSelectAllInInput(e: KeyboardEvent) {
    if (!(e.metaKey || e.ctrlKey) || e.shiftKey || e.key.toLowerCase() !== 'a') return
    const t = e.target as HTMLElement | null
    if (!t) return
    const tag = t.tagName
    if (tag === 'INPUT' || tag === 'TEXTAREA') {
      e.preventDefault()
      e.stopPropagation()
      ;(t as HTMLInputElement | HTMLTextAreaElement).select()
    }
  }

  onMount(async () => {
    document.addEventListener('keydown', handleSelectAllInInput, true)

    // Native macOS App-menu items route here.
    EventsOn('menu:openSettings', () => {
      openSettings('general')
    })
    EventsOn('menu:openBackupViewer', () => {
      showSettings = false
      showBackupViewer = true
    })
    EventsOn('menu:openAbout', () => {
      openSettings('about')
    })
    EventsOn('backup:progress', (data: BackupProgress) => {
      if (data.phase === 'done') {
        const statistics = backupStatistics(data)
        addToast({
          type: statistics.notBackedUp > 0 ? 'warning' : 'success',
          message: $_('settingsBackup.backupComplete', {
            values: {
              checked: statistics.checked,
              backedUp: statistics.backedUp,
              notBackedUp: statistics.notBackedUp,
            },
          }),
        })
      } else if (data.phase === 'error') {
        addToast({
          type: 'error',
          message: data.message ? `${$_('settingsBackup.backupFailed')}: ${data.message}` : $_('settingsBackup.backupFailed'),
        })
      }
    })

    // Notification clicks (from Go), the Contacts related-mail list (via
    // EventsEmit), and the search overlay all route conversation-open through
    // openMailConversation (defined at component scope).
    EventsOn('notification:clicked', openMailConversation)
    EventsOn('mail:openConversation', openMailConversation)

    // Listen for window show requests (from single-instance activation, notification clicks)
    EventsOn('window:show', () => {
      window.focus()
    })

    // Listen for untrusted certificate events from background sync
    EventsOn('certificate:untrusted', (data: { accountId: string; certificate: certificate.CertificateInfo }) => {
      // Only show if not already showing a cert dialog
      if (!showCertDialog) {
        pendingCertificate = data.certificate
        pendingCertAccountId = data.accountId
        showCertDialog = true
      }
    })

    // Listen for external mailto from second instance (routed through backend)
    EventsOn('mailto:external', (data: MailtoData) => {
      handleMailtoData(data)
    })

    // Finder's Open With action routes regular files here. The backend waits
    // for NotifyStartupComplete before emitting, so accounts are loaded first.
    EventsOn('files:openAsAttachments', (payload: unknown) => {
      enqueueExternalFiles(payload)
    })

    // Listen for escape-iframe-focus event (from EmailBody when navigating away from iframe)
    const handleEscapeIframeFocus = () => {
      // Focus the message list container to take keyboard focus away from iframe
      messageListContainerRef?.focus()
    }
    window.addEventListener('escape-iframe-focus', handleEscapeIframeFocus)

    // Load application settings (including theme mode) and apply theme
    const storedThemeMode = await loadSettings()
    await initTheme(storedThemeMode, GetSystemTheme)

    // Load image allowlist cache for synchronous checks in EmailBody
    loadImageAllowlist()

    // Check if terms have been accepted
    try {
      const termsAccepted = await GetTermsAccepted()
      if (!termsAccepted) {
        showTermsDialog = true
      }
    } catch (err) {
      console.error('Failed to check terms acceptance:', err)
      // Show dialog on error to be safe
      showTermsDialog = true
    }

    // Load accounts and folders before restoring a persisted selection. This
    // prevents a hierarchy-only or removed folder from reaching MessageList or
    // ConversationViewer during startup.
    await accountStore.load()

    // Load persisted UI state
    const uiState = await loadUIState()

    // Preload Contacts account groups for the built-in Contacts pane.
    preloadContactAccountGroups()

    // Restore pane widths (already validated/clamped by loadUIState)
    sidebarWidth = uiState.sidebarWidth
    listWidth = uiState.listWidth
    // Apply the window minimum width for the restored pane layout.
    updateWindowMinWidth()

    // Restore folder selection if valid. A legacy 'unified' selection no longer
    // resolves to any account, so it falls through and nothing is restored.
    if (uiState.selectedAccountId && uiState.selectedFolderId) {
      const accountExists = accountStore.accounts.some(
        a => a.account.id === uiState.selectedAccountId
      )

      const restoredFolder = findFolderById(uiState.selectedAccountId, uiState.selectedFolderId)

      if (!shouldClearRestoredFolderSelection(accountExists, true, restoredFolder)) {
        selectedAccountId = uiState.selectedAccountId
        selectedFolderId = uiState.selectedFolderId
        selectedFolderName = uiState.selectedFolderName || 'Inbox'
        selectedFolderType = uiState.selectedFolderType
        selectionSource = 'account'

        // Restore conversation selection
        if (uiState.selectedThreadId) {
          selectedThreadId = uiState.selectedThreadId
          selectedConversationAccountId = uiState.selectedConversationAccountId
          selectedConversationFolderId = uiState.selectedConversationFolderId
        }
      } else {
        saveUIState({
          selectedAccountId: null,
          selectedFolderId: null,
          selectedFolderName: 'Inbox',
          selectedFolderType: null,
          selectedThreadId: null,
          selectedConversationAccountId: null,
          selectedConversationFolderId: null,
        })
      }
    }

    // Keep launch synchronization asynchronous, matching the previous Sidebar
    // behavior while ensuring account/folder state has initialized exactly once.
    void accountStore.syncAllComplete().catch((err) => {
      console.error('Failed to sync on launch:', err)
    })

    // Listen for system theme changes from backend (XDG Settings Portal)
    EventsOn('theme:system-preference', (newTheme: string) => {
      handleSystemThemeEvent(newTheme)
    })

    // Listen for system theme changes via matchMedia (fallback when portal unavailable)
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    mediaQuery.addEventListener('change', (e) => {
      handleMediaQueryChange(e.matches)
    })

    // App is fully initialized — dismiss the inline boot splash from
    // index.html. The CSS transition fades it out; we remove the element
    // shortly after to free the DOM. Done BEFORE the start-hidden check
    // so users in background mode don't see the splash linger on screen
    // before the window hides.
    const splash = document.getElementById('boot-splash')
    if (splash) {
      splash.hidden = true
      setTimeout(() => splash.remove(), 250)
    }

    // main.ts called WindowShow() at module load so the splash was visible
    // during slow startup work (migrations etc.). If the user has start-
    // hidden background mode, undo that now. Otherwise the window is already
    // visible — calling WindowShow again is harmless.
    const shouldStartHidden = await GetStartHiddenActive()
    if (shouldStartHidden) {
      WindowHide()
    }
    if (!shouldStartHidden) {
      WindowShow()
    }

    // Clear the desktop-environment startup indicator. Called after WindowShow()
    // so KDE/Plasma sees the placeholder → real window handoff cleanly (#154).
    // Fired unconditionally so the indicator clears even when starting hidden.
    NotifyStartupComplete()

    // Remove GTK max size constraints that Wails v2 sets at startup
    RefreshWindowConstraints()

    // Initialize responsive layout breakpoint listeners
    initLayout()

    // Check for pending mailto: URL from command line
    try {
      const mailtoData = await GetPendingMailto()
      if (mailtoData && (mailtoData.to?.length > 0 || mailtoData.subject || mailtoData.body)) {
        // Wait a moment for accounts to load
        await new Promise(resolve => setTimeout(resolve, 100))
        handleMailtoData(mailtoData)
      }
    } catch (err) {
      console.error('Failed to check pending mailto:', err)
    }
  })

  // Handle folder selection from sidebar (account tree)
  function handleFolderSelect(
    accountId: string,
    folderId: string,
    folderPath: string,
    folderName: string,
    folderType: string
  ) {
    if (findFolderById(accountId, folderId)?.noSelect) return
    selectedAccountId = accountId
    selectedFolderId = folderId
    selectedFolderName = folderName
    selectedFolderType = folderType
    selectionSource = 'account'
    // Don't clear the viewer here — the message list auto-opens the new folder's
    // first conversation (or calls onEmptyFolder for an empty folder), so the
    // old conversation stays in place until the new one loads, avoiding a
    // flicker/jump in the viewer when switching folders/accounts.
    hideSidebar()

    // Persist state (thread selection is persisted by the auto-open that follows)
    saveUIState({
      selectedAccountId: accountId,
      selectedFolderId: folderId,
      selectedFolderName: folderName,
      selectedFolderType: folderType,
    })
  }

  // The selected folder has no conversations — clear the viewer.
  function handleEmptyFolder() {
    selectedThreadId = null
    selectedConversationFolderId = null
    selectedConversationAccountId = null
    saveUIState({
      selectedThreadId: null,
      selectedConversationAccountId: null,
      selectedConversationFolderId: null,
    })
  }

  function selectMailFolder(accountId: string, folderId: string) {
    const folderInfo = findFolderById(accountId, folderId)
    if (folderInfo?.noSelect) return null
    const folderName = folderInfo?.name || 'Inbox'
    const folderType = folderInfo?.type || 'inbox'

    selectedAccountId = accountId
    selectedFolderId = folderId
    selectedFolderName = folderName
    selectedFolderType = folderType
    selectionSource = 'account'
    accountStore.selectFolder(accountId, folderId, folderInfo?.path || '', folderName)
    sidebarRef?.revealFolder(accountId, folderId)

    return { folderName, folderType }
  }

  // Keep the detail model synchronized with the list cursor without forcing a
  // responsive detail overlay open. Arrow navigation uses this path; Enter or
  // a row click follows with handleConversationSelect to reveal the item.
  function handleConversationFocus(threadId: string, folderId: string, accountId: string) {
    selectedThreadId = threadId
    selectedConversationFolderId = folderId
    selectedConversationAccountId = accountId

    saveUIState({
      selectedThreadId: threadId,
      selectedConversationAccountId: accountId,
      selectedConversationFolderId: folderId,
    })
  }

  // Handle activation from the list. The detail is already synchronized by
  // focus navigation; activation additionally reveals responsive layouts.
  function handleConversationSelect(threadId: string, folderId: string, accountId: string) {
    if (accountId && folderId && (accountId !== selectedAccountId || folderId !== selectedFolderId)) {
      openMailConversation({ accountId, folderId, threadId })
      return
    }
    handleConversationFocus(threadId, folderId, accountId)
    showViewer()
  }

  // Open a specific conversation in the mail view: switch the rail to mail,
  // select the folder, and highlight the thread. Shared by notification clicks
  // (from Go), the Contacts related-mail list (via EventsEmit), and the `/`
  // search overlay.
  function openMailConversation(data: { accountId: string; folderId: string; threadId: string }) {
    setActivePane('mail')

    const selected = selectMailFolder(data.accountId, data.folderId)
    if (!selected) return
    const { folderName, folderType } = selected

    selectedThreadId = data.threadId
    selectedConversationAccountId = data.accountId
    selectedConversationFolderId = data.folderId
    showViewer()

    // Highlight the thread in the message list (small delay so the list loads)
    setTimeout(async () => {
      await messageListRef?.selectThread(data.threadId)
      selectedThreadId = data.threadId
      selectedConversationAccountId = data.accountId
      selectedConversationFolderId = data.folderId
      showViewer()
    }, 100)

    saveUIState({
      selectedAccountId: data.accountId,
      selectedFolderId: data.folderId,
      selectedFolderName: folderName,
      selectedFolderType: folderType,
      selectedThreadId: data.threadId,
      selectedConversationAccountId: data.accountId,
      selectedConversationFolderId: data.folderId,
    })
  }

  // Resolve an account ID that may be 'unified' to a real account ID.
  // Returns the first real account ID if the input is 'unified' or falsy.
  function resolveAccountId(id: string | null): string | undefined {
    if (id && id !== 'unified') return id
    return accountStore.accounts[0]?.account.id
  }

  // Handle compose button click (new message)
  function handleCompose() {
    // Use the selected account, or the first account if none selected
    const accountId = resolveAccountId(selectedAccountId)
    if (!accountId) return

    composerAccountId = accountId
    composerInitialMessage = null
    composerDraftId = null
    showComposer = true
  }

  function enqueueExternalFiles(payload: unknown) {
    const paths = normalizeExternalOpenFiles(payload)
    if (paths.length === 0) return

    if (showComposer || externalFileComposeBusy) {
      pendingExternalFileBatches.push(paths)
      addToast({
        type: 'info',
        message: $_('toast.externalFilesQueued', { values: { count: paths.length } }),
      })
      return
    }

    void openExternalFilesAsNewMessage(paths)
  }

  function processNextExternalFileBatch() {
    if (showComposer || externalFileComposeBusy || pendingExternalFileBatches.length === 0) return
    const paths = pendingExternalFileBatches.shift()
    if (paths) void openExternalFilesAsNewMessage(paths)
  }

  async function openExternalFilesAsNewMessage(paths: string[]) {
    const accountId = resolveAccountId(selectedAccountId)
    if (!accountId) {
      addToast({
        type: 'error',
        message: $_('toast.noAccountConfigured'),
      })
      processNextExternalFileBatch()
      return
    }

    externalFileComposeBusy = true
    const attachments: smtp.Attachment[] = []
    let failedCount = 0

    for (const filePath of paths) {
      try {
        const attachment = await ReadFileAsAttachment(filePath)
        if (!attachment) {
          failedCount += 1
          continue
        }
        attachments.push(new smtp.Attachment(toExternalSmtpAttachment(attachment)))
      } catch {
        // Do not put user file paths into webview logs. The toast below reports
        // only the failure count while successful files still open normally.
        failedCount += 1
      }
    }

    // The user can still open a composer while attachment reads are in
    // flight. Preserve that message and retry the native request after it
    // closes instead of replacing unsaved content.
    if (showComposer) {
      pendingExternalFileBatches.unshift(paths)
      externalFileComposeBusy = false
      addToast({
        type: 'info',
        message: $_('toast.externalFilesQueued', { values: { count: paths.length } }),
      })
      return
    }

    if (failedCount > 0) {
      addToast({
        type: 'error',
        message: $_('toast.externalFilesFailed', { values: { count: failedCount } }),
      })
    }

    if (attachments.length > 0) {
      composerAccountId = accountId
      composerDraftId = null
      composerInitialMessage = new smtp.ComposeMessage({
        from: new smtp.Address({ name: '', address: '' }),
        to: [],
        cc: [],
        bcc: [],
        subject: '',
        text_body: '',
        html_body: '',
        attachments,
        request_read_receipt: false,
      })
      showComposer = true
    }

    externalFileComposeBusy = false
    if (!showComposer) processNextExternalFileBatch()
  }

  // Handle edit draft (opens composer with existing draft)
  async function handleEditDraft(draftId: string) {
    // Use conversation's account ID, fall back to selected account or first account
    const accountId = resolveAccountId(selectedConversationAccountId) || resolveAccountId(selectedAccountId)
    if (!accountId) return

    try {
      // Load the draft content from backend
      const draftMessage = await GetDraft(draftId)

      composerAccountId = accountId
      composerInitialMessage = draftMessage || null
      composerDraftId = draftId
      showComposer = true
    } catch (err) {
      console.error('Failed to load draft:', err)
      addToast({
        type: 'error',
        message: $_('composer.failedToLoadDraft'),
      })
    }
  }

  // Handle compose to a specific email address (from mailto: links in emails)
  function handleComposeToAddress(toAddress: string) {
    // Use conversation's account ID, or selected account, or first account
    const accountId = resolveAccountId(selectedConversationAccountId) || resolveAccountId(selectedAccountId)
    if (!accountId) return

    composerAccountId = accountId
    composerDraftId = null
    // Create a minimal ComposeMessage with just the To address
    composerInitialMessage = new smtp.ComposeMessage({
      from: new smtp.Address({ name: '', address: '' }),
      to: [new smtp.Address({ name: '', address: toAddress })],
      cc: [],
      bcc: [],
      subject: '',
      text_body: '',
      html_body: '',
      attachments: [],
      request_read_receipt: false,
    })
    showComposer = true
  }

  // Handle mailto: URL data (from command line launch)
  interface MailtoData {
    to?: string[]
    cc?: string[]
    bcc?: string[]
    subject?: string
    body?: string
  }

  interface BackupProgress {
    phase: string
    current: number
    total: number
    exported: number
    skipped: number
    missing?: number
    unavailable?: number
    failed: number
    message?: string
  }

  function handleMailtoData(data: MailtoData) {
    // Use selected account or first account (resolve 'unified' to real account)
    const accountId = resolveAccountId(selectedAccountId)
    if (!accountId) {
      // No accounts available, can't compose
      addToast({
        type: 'error',
        message: $_('toast.noAccountConfigured'),
      })
      return
    }

    composerAccountId = accountId
    composerDraftId = null
    composerInitialMessage = new smtp.ComposeMessage({
      from: new smtp.Address({ name: '', address: '' }),
      to: (data.to || []).map(addr => new smtp.Address({ name: '', address: addr })),
      cc: (data.cc || []).map(addr => new smtp.Address({ name: '', address: addr })),
      bcc: (data.bcc || []).map(addr => new smtp.Address({ name: '', address: addr })),
      subject: data.subject || '',
      text_body: data.body || '',
      html_body: '',
      attachments: [],
      request_read_receipt: false,
    })
    showComposer = true
  }

  // Handle reply/reply-all/forward - calls backend API
  async function handleReply(mode: 'reply' | 'reply-all' | 'forward', messageId: string, imagesLoaded?: boolean) {
    // Use conversation's account ID (important for unified inbox), fall back to selected account or first account
    const accountId = resolveAccountId(selectedConversationAccountId) || resolveAccountId(selectedAccountId)
    if (!accountId) return

    try {
      // Call backend to prepare the reply message (backend gets account from message)
      const composeMessage = await PrepareReply(messageId, mode)
      composerAccountId = accountId
      composerDraftId = null
      composerInitialMessage = composeMessage
      composerImagesLoaded = imagesLoaded || false
      showComposer = true
    } catch (err) {
      console.error(`Failed to prepare ${mode}:`, err)
      addToast({
        type: 'error',
        message: $_('toast.failedToPrepare', { values: { mode } }),
      })
      // Fallback: open blank composer
      composerAccountId = accountId
      composerDraftId = null
      composerInitialMessage = null
      showComposer = true
    }
  }

  // Close composer
  function closeComposer() {
    const wasOpen = showComposer
    showComposer = false
    composerAccountId = null
    composerInitialMessage = null
    if (wasOpen) {
      void tick().then(processNextExternalFileBatch)
    }
  }

  // Pane sizing — fixed widths (no dragging). Restored from saved state on
  // mount; the sidebar and list columns are not user-resizable.
  let sidebarWidth = $state(DEFAULT_SIDEBAR_WIDTH)
  let listWidth = $state(DEFAULT_LIST_WIDTH)

  // Window minimum width so the viewer's toolbar always fits. The viewer gets
  // (window − rail − sidebar − list), so the floor leaves VIEWER_MIN for it.
  const RAIL_WIDTH = 48        // w-12 icon rail
  const VIEWER_MIN = 520       // fits the full viewer action toolbar (~12 icons)
  const LAYOUT_BUFFER = 30     // pane borders
  const WINDOW_MIN_HEIGHT = 400
  function updateWindowMinWidth() {
    const minW = Math.round(RAIL_WIDTH + sidebarWidth + listWidth + VIEWER_MIN + LAYOUT_BUFFER)
    WindowSetMinSize(minW, WINDOW_MIN_HEIGHT)
  }

  // After a synthetic contextmenu event, bits-ui mounts the portal asynchronously.
  // Poll until [role="menu"] appears, then focus the first menuitem.
  function focusContextMenu() {
    let attempts = 0
    const tryFocus = () => {
      const menu = document.querySelector('[role="menu"]') as HTMLElement | null
      if (menu) {
        const firstItem = menu.querySelector('[role="menuitem"]:not([data-disabled])') as HTMLElement | null
        ;(firstItem || menu).focus()
        return
      }
      if (attempts++ < 10) {
        requestAnimationFrame(tryFocus)
      }
    }
    requestAnimationFrame(tryFocus)
  }

  // Track Left Alt held state for Left Alt + Right Alt combo
  let leftAltHeld = false

  function handleGlobalKeyUp(e: KeyboardEvent) {
    if (e.code === 'AltLeft') {
      leftAltHeld = false
    }
  }

  // Global keyboard shortcut handler
  function handleGlobalKeyDown(e: KeyboardEvent) {
    if (e.code === 'AltLeft') {
      leftAltHeld = true
    }

    handleGlobalShortcut(e, {
      leftAltHeld,
      showSearchOverlay,
      showComposer,
      selectedThreadId,
      selectedAccountId,
      selectedFolderId,
      focusMode,
      sidebarRef,
      messageListRef,
      viewerRef,
      setSearchOverlay: (open) => { showSearchOverlay = open },
      setFocusMode: (mode) => { focusMode = mode },
      setFocusedMessageIdInFocus: (messageId) => { focusedMessageIdInFocus = messageId },
      focusContextMenu,
      openRegionActionMenu: () => { keyboardActionMenu.showForRegion(getFocusedPane()) },
      handleQuit,
      handleCompose,
      handleReply,
      getLastMessageId,
      handleBulkMarkRead,
      handleBulkMarkUnread,
      handleBulkArchive,
      handleBulkSpam,
      handleBulkToggleStar,
      toggleThreadFocus,
    })
  }

  // Get the last message ID from the current conversation (for reply/forward)
  function getLastMessageId(): string | null {
    return viewerRef?.getLastMessageId() ?? null
  }

  // Handle click on pane to set focus
  function handlePaneClick(pane: FocusablePane) {
    setFocusedPane(pane)
  }

  function handlePaneMouseDown(pane: FocusablePane, event: MouseEvent) {
    handlePaneClick(pane)
    if (pane !== 'sidebar' || event.button !== 0) return
    if (!(event.target instanceof Element) || !event.target.closest('[data-sidebar-item]')) return

    // Folder/account rows are logical selections inside one keyboard region.
    // Keep DOM focus on that region so an old clicked row cannot retain a
    // native WebKit focus outline after arrow-key selection moves elsewhere.
    event.preventDefault()
    const region = event.currentTarget as HTMLElement | null
    region?.focus({ preventScroll: true })
  }

  // Bulk action handlers
  function handleBulkActionComplete(autoSelectNext?: boolean) {
    messageListRef?.clearChecked()
    messageListRef?.handleActionComplete(autoSelectNext)
  }

  async function handleBulkArchive(messageIds: string[]) {
    await archiveMessages(messageIds, {
      onUndo: handleUndo,
      onSuccess: handleBulkActionComplete,
      autoSelectNext: true,
    })
  }

  async function handleBulkSpam(messageIds: string[]) {
    await toggleSpamMessages(messageIds, selectedFolderType === 'spam', {
      onUndo: handleUndo,
      onSuccess: handleBulkActionComplete,
      autoSelectNext: true,
      spamSuccessMode: 'alwaysMarked',
    })
  }

  async function handleBulkMarkRead(messageIds: string[]) {
    await setReadStateMessages(messageIds, true, {
      onSuccess: handleBulkActionComplete,
      errorKey: 'toast.failedToMarkAsRead',
    })
  }

  async function handleBulkMarkUnread(messageIds: string[]) {
    await setReadStateMessages(messageIds, false, {
      onSuccess: handleBulkActionComplete,
      errorKey: 'toast.failedToMarkAsUnread',
    })
  }

  async function handleBulkToggleStar(messageIds: string[], shouldStar: boolean) {
    await toggleStarMessages(messageIds, shouldStar, {
      onSuccess: handleBulkActionComplete,
    })
  }

  async function handleUndo() {
    await undoLastMailAction({
      onSuccess: () => messageListRef?.handleActionComplete(),
    })
  }
</script>

<svelte:window onkeydown={handleGlobalKeyDown} onkeyup={handleGlobalKeyUp} />

<div class="flex flex-col h-full w-full overflow-hidden bg-background">
  <!-- Main Content -->
  <div class="flex flex-1 min-h-0 overflow-hidden relative">
    <ActivityRail bind:this={activityRailRef} onOpenSettings={() => openSettings()} />

    {#if getActivePane() === 'contacts'}
      <ContactsPane />
    {/if}

    <!-- Mail layout is ALWAYS mounted; only its visibility is toggled when
         Contacts takes over the pane. Unmounting+remounting the mail tree on
         every rail switch was leaking state (zombie listeners) and pinning
         the main thread on the second mount. display:contents keeps the flex
         children as direct flex items so the layout doesn't shift. -->
    <div style:display={getActivePane() === 'mail' ? 'contents' : 'none'}>
    <!-- Sidebar (Folder List) -->
    <aside
      data-keyboard-region="sidebar"
      data-keyboard-region-visible={getLayoutMode() !== 'narrow' || getResponsiveView() === 'sidebar'}
      data-keyboard-region-focus-target
      data-region-active={isMainKeyboardScope() && getActivePane() === 'mail' && getFocusedPane() === 'sidebar'}
      tabindex="-1"
      class="keyboard-region outline-none {getLayoutMode() === 'narrow' ? `responsive-sidebar-overlay w-72 border-r border-border bg-background ${getResponsiveView() === 'sidebar' ? 'responsive-sidebar-visible' : ''}` : 'flex-shrink-0 border-r border-border bg-muted/30'}"
      style="{getLayoutMode() === 'full' ? `width: ${sidebarWidth}px` : ''}"
      role="presentation"
      onmousedown={(event) => handlePaneMouseDown('sidebar', event)}
    >
      <Sidebar
        bind:this={sidebarRef}
        onFolderSelect={handleFolderSelect}
        onCompose={handleCompose}
        onMessagesMoved={() => messageListRef?.handleActionComplete(false)}
        selectedAccountId={selectedAccountId}
        selectedFolderId={selectedFolderId}
        selectionSource={selectionSource}
        isFocused={getFocusedPane() === 'sidebar'}
        showBackButton={getLayoutMode() === 'narrow'}
        onBack={hideSidebar}
      />
    </aside>

    <!-- Scrim for narrow sidebar overlay -->
    {#if getLayoutMode() === 'narrow'}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div
        role="button"
        tabindex="-1"
        class="responsive-scrim {getResponsiveView() === 'sidebar' ? 'responsive-scrim-visible' : ''}"
        onclick={hideSidebar}
        aria-label={$_('aria.closeSidebar')}
      ></div>
    {/if}

    <!-- Message List -->
    <section
      bind:this={messageListContainerRef}
      data-keyboard-region="messageList"
      data-keyboard-region-visible={getLayoutMode() === 'full' || getResponsiveView() === 'default'}
      data-keyboard-region-focus-target
      data-region-active={isMainKeyboardScope() && getActivePane() === 'mail' && getFocusedPane() === 'messageList'}
      class="keyboard-region outline-none {isResponsive() ? 'flex-1 min-w-0 border-r border-border bg-background' : 'flex-shrink-0 border-r border-border bg-background'}"
      style="{getLayoutMode() === 'full' ? `width: ${listWidth}px` : ''}"
      role="presentation"
      data-pane="messageList"
      tabindex="-1"
      onmousedown={() => handlePaneClick('messageList')}
    >
      <MessageList
        bind:this={messageListRef}
        accountId={selectedAccountId}
        folderId={selectedFolderId}
        folderName={selectedFolderName}
        folderType={selectedFolderType || 'inbox'}
        onConversationSelect={handleConversationSelect}
        onConversationFocus={handleConversationFocus}
        onEmptyFolder={handleEmptyFolder}
        onReply={handleReply}
        onOpenDraft={handleEditDraft}
        onSearch={() => { showSearchOverlay = true }}
        onRowActionComplete={() => viewerRef?.refreshFlags()}
        isFocused={getFocusedPane() === 'messageList'}
        showFolderToggle={getLayoutMode() === 'narrow'}
        onToggleSidebar={showSidebar}
      />
    </section>

    <!-- Conversation Viewer -->
    <main
      data-keyboard-region="viewer"
      data-keyboard-region-visible={!viewerIsOverlay || viewerIsVisible}
      data-keyboard-region-focus-target
      data-region-active={isMainKeyboardScope() && getActivePane() === 'mail' && getFocusedPane() === 'viewer'}
      tabindex="-1"
      class="keyboard-region outline-none {viewerIsOverlay ? `responsive-viewer-overlay bg-background ${viewerIsVisible ? 'responsive-viewer-visible' : ''}` : 'flex-1 min-w-0 bg-background'}"
      role="presentation"
      data-pane="viewer"
      onmousedown={() => handlePaneClick('viewer')}
    >
      <ConversationViewer
        bind:this={viewerRef}
        threadId={selectedThreadId}
        folderId={selectedConversationFolderId}
        folderType={selectedFolderType}
        accountId={selectedConversationAccountId}
        onReply={handleReply}
        onComposeToAddress={handleComposeToAddress}
        onEditDraft={handleEditDraft}
        onActionComplete={(autoSelectNext) => messageListRef?.handleActionComplete(autoSelectNext)}
        isFocused={getFocusedPane() === 'viewer'}
        showBackButton={isResponsive()}
        onBack={() => { focusMode = 'off'; focusedMessageIdInFocus = null; hideViewer() }}
        inFocusMode={focusMode !== 'off'}
        focusModeKind={focusMode === 'off' ? null : focusMode}
        focusedMessageIdInFocus={focusedMessageIdInFocus}
        onToggleThreadFocus={toggleThreadFocus}
      />
    </main>
    </div>
  </div>
  <StatusBar />
</div>

<!-- Composer Modal -->
{#if showComposer && composerAccountId}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/80">
    <div class="{getLayoutMode() === 'narrow' ? 'w-full h-full bg-background overflow-hidden' : 'w-full max-w-3xl h-[80vh] bg-background border rounded-lg shadow-xl overflow-hidden'}">
      <Composer
        accountId={composerAccountId}
        initialMessage={composerInitialMessage}
        draftId={composerDraftId}
        imagesLoaded={composerImagesLoaded}
        onClose={closeComposer}
        onSent={closeComposer}
      />
    </div>
  </div>
{/if}

<!-- Shutdown Overlay -->
{#if isShuttingDown}
  <div class="fixed inset-0 z-[100] flex items-center justify-center bg-black/80">
    <p class="text-white/90 text-sm font-medium">{$_('window.shuttingDown')}</p>
  </div>
{/if}

<!-- Terms Acceptance Dialog -->
<TermsDialog bind:open={showTermsDialog} onAccept={handleTermsAccepted} />

<!-- App Settings dialog — opened from the rail's gear (works in every view) -->
<SettingsDialog bind:open={showSettings} bind:activePage={settingsPage} onClose={handleSettingsClosed} />
<BackupViewerDialog bind:open={showBackupViewer} onClose={() => { showBackupViewer = false }} />

<SearchOverlay
  bind:open={showSearchOverlay}
  mode={getActivePane() === 'mail' ? 'mail' : 'contacts'}
  onClose={() => { showSearchOverlay = false }}
  onSelectMail={(r) => {
    showSearchOverlay = false
    openMailConversation({
      accountId: r.accountId || resolveAccountId(selectedAccountId) || '',
      folderId: r.folderId || selectedFolderId || '',
      threadId: r.threadId,
    })
  }}
  onSelectContact={async (c) => {
    showSearchOverlay = false
    setActivePane('contacts')
    await tick()
    await activateContactFromGlobalSearch(c.id)
  }}
/>

<KeyboardActionMenu />


<!-- Certificate TOFU Dialog (for background sync cert errors) -->
<CertificateDialog
  bind:open={showCertDialog}
  certificate={pendingCertificate}
  onAcceptOnce={handleBgCertAcceptOnce}
  onAcceptPermanently={handleBgCertAcceptPermanently}
  onDecline={handleBgCertDecline}
/>
