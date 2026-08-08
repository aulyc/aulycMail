<script lang="ts">
  import { onMount, onDestroy, getContext, setContext, untrack } from 'svelte'
  import Icon from '@iconify/svelte'
  import type { Editor } from '@tiptap/core'
  import { createComposerEditor } from './composerEditor'
  import { readComposerClipboardFiles } from './composerClipboard'
  // @ts-ignore - Wails generated imports
  import { smtp, account, app, contact } from '../../../../wailsjs/go/models'
  // @ts-ignore - Wails runtime for events
  import { EventsOn } from '../../../../wailsjs/runtime/runtime.js'
  import { type ComposerApi, COMPOSER_API_KEY, createMainWindowApi } from '$lib/composerApi'
  import { isImageAllowedSync } from '$lib/stores/imageAllowlist.svelte'
  import {
    getAlwaysLoadImages,
    getComposerFormat,
    getDarkMailContent,
    getEnhancedKeyboardNavigation,
  } from '$lib/stores/settings.svelte'
  import RecipientInput from './RecipientInput.svelte'
  import EditorToolbar from './EditorToolbar.svelte'
  import ComposerAttachmentList from './ComposerAttachmentList.svelte'
  import ComposerConfirmDialogs from './ComposerConfirmDialogs.svelte'
  import {
    addParagraphStyles,
    stripParagraphStyles,
    htmlToPlainText,
    parseFileUris,
    plainTextToHtml,
    textMentionsAttachment,
  } from './composerUtils'
  import {
    backendAttachmentToComposerAttachment,
    fileToComposerAttachment,
    hasFileDropPayload,
    isRecipientChipDrag,
    selectedFileToComposerAttachment,
    type ComposerAttachment,
    type InlineImage,
  } from './composerAttachments'
  import {
    filteredMentionResults,
    findMentionToken,
    getContactEmail,
    getMentionLabel,
    getPlainTextMentionSegments,
    hasRecipient,
    mentionKey,
    type MentionSurface,
  } from './composerMentions'
  import {
    findPlainQuoteBoundary,
    findRichQuoteBoundary,
    hasPlainQuoteBoundary,
  } from './composerQuoteBoundaries'
  import {
    buildSignatureHtml,
    getSignatureSeparator,
    shouldAppendSignature,
    insertSignatureIntoContent,
    removeSignatureFromContent,
    hasSignatureMarker,
  } from './composerSignature'
  import {
    handleComposerTabNavigation,
    type ComposerFocusRefs,
  } from './composerFocus'
  import { isOptionCodeShortcut } from '$lib/keyboard/keyboardPolicy'
  import { keyboardActionMenu } from '$lib/stores/keyboardActionMenu.svelte'
  import {
    buildDraftContentHash,
    getDraftStatusMeta,
    type DraftSaveStatus,
    type DraftSyncStatus,
  } from './composerDraft'
  import { discardDraftBeforeClose } from './composerDiscard'
  import {
    composerHasContent,
    formatIdentityLabel,
    getComposerDisplayMode,
    widenComposerLabel,
  } from './composerDisplay'
  import {
    clampMentionPosition,
    getPlainMentionPosition,
    MENTION_MENU_HEIGHT,
    MENTION_VISIBLE_ROWS,
  } from './composerMentionLayout'
  import {
    createInlineImageFromAttachment,
    createInlineImageFromDataUrl,
    createInlineImageCID,
  } from './composerInlineImages'
  import {
    base64DecodedSize,
    evaluateInlineImageBatch,
    formatInlineImageSize,
  } from './composerInlineImagePolicy'
  import {
    buildComposeMessage,
    restoreBlockedRemoteImages,
    toSmtpAddress,
  } from './composerMessage'
  import {
    clampMentionSelection,
    createMentionSearchController,
    moveMentionPointerSelection,
    selectMentionIndex,
    type MentionInputMode,
    type MentionSelectionState,
  } from './composerMentionController'
  import { createDraftSaveController } from './composerDraftController'
  import {
    registerInlineImage,
    replaceInlineImageSourcesWithCids,
  } from './composerInlinePipeline'
  import { getComposerSendBlocker } from './composerSendController'
  import * as Select from '$lib/components/ui/select'
  import { addToast } from '$lib/stores/toast'
  import { getIsDarkActive } from '$lib/stores/theme.svelte'
  import { buildDarkMailFilterStyles, getDarkMailSurfaceBackground } from '$lib/utils/dark-mail'
  import { _ } from '$lib/i18n'

  // Props
  interface Props {
    accountId: string
    /** Pre-populated message from backend (for reply/forward), or null for new message */
    initialMessage?: smtp.ComposeMessage | null
    /** Existing draft ID if editing a draft */
    draftId?: string | null
    onClose?: () => void
    onSent?: () => void
    /** Optional API override - if not provided, uses context or creates main window API */
    api?: ComposerApi
    /** Whether remote images were loaded in the viewer before reply/forward */
    imagesLoaded?: boolean
  }

  let { accountId, initialMessage = null, draftId = null, onClose, onSent, api: propApi, imagesLoaded = false }: Props = $props()

  // Widen two-character CJK labels so they align with three-character labels.
  const widenLabel = widenComposerLabel

  // Get API from context, props, or create default main window API
  const contextApi = getContext<ComposerApi | undefined>(COMPOSER_API_KEY)
  const defaultApi = createMainWindowApi()
  // Resolve once at init — the API never changes after mount
  // svelte-ignore state_referenced_locally
  const resolvedApi: ComposerApi = propApi || contextApi || defaultApi
  // Use $derived so propApi changes are detected (even though it typically doesn't change after mount)
  const api: ComposerApi = $derived(propApi || contextApi || defaultApi)

  // Propagate the resolved API to child components (e.g. RecipientInput)
  // so they can access it via getContext instead of falling back to the main window API
  setContext(COMPOSER_API_KEY, resolvedApi)

  // State
  let allGroups = $state<app.AccountIdentityGroup[]>([])  // All accounts + identities (main window only)
  let identities = $state<account.Identity[]>([])  // Flat list of all identities (union)
  let selectedIdentityId = $state<string>('')

  // Derive the active account ID from the selected identity's accountId.
  // Falls back to the prop accountId if no identity is selected yet.
  let activeAccountId = $derived.by(() => {
    if (!selectedIdentityId) return accountId
    const identity = identities.find(i => i.id === selectedIdentityId)
    return identity?.accountId || accountId
  })
  let toRecipients = $state<smtp.Address[]>([])
  let ccRecipients = $state<smtp.Address[]>([])
  let bccRecipients = $state<smtp.Address[]>([])
  let subject = $state('')
  let showCc = $state(false)
  let showBcc = $state(false)
  let sending = $state(false)
  let editorElement = $state<HTMLElement | null>(null)
  let editor = $state<Editor | null>(null)

  // Track In-Reply-To and References for threading
  let inReplyTo = $state<string | undefined>(undefined)
  let references = $state<string[]>([])
  let sourceMessageId = $state('')
  let replyType = $state<'reply' | 'reply-all' | 'forward' | ''>('')

  // Attachments
  let attachments = $state<ComposerAttachment[]>([])
  let isDraggingOver = $state(false)

  // Inline images (embedded in HTML body)
  let inlineImages = $state<InlineImage[]>([])
  let inlineImageCounter = 0  // Counter for generating unique CIDs
  let acknowledgedInlineImageBytes = 0

  // Read receipt request
  let requestReadReceipt = $state(false)
  let showReadReceiptOption = $state(false)  // Show checkbox when policy is 'ask'

  // Plain text mode toggle (default from user setting, can be toggled per-message)
  let isPlainTextMode = $state(getComposerFormat() === 'plain')
  // Runtime-only preview toggle. The filter is applied to the editor DOM and is
  // never serialized into the outgoing message HTML.
  let composerDarkFilterEnabled = $state(true)
  let plainTextContent = $state('')  // Store plain text when in plain text mode
  let plainTextRef = $state<HTMLTextAreaElement | null>(null)  // textarea element (plain text mode)
  let plainTextScrollTop = $state(0)
  let plainTextScrollLeft = $state(0)
  let plainMentionLabels = $state<string[]>([])
  let composerBodyElement = $state<HTMLDivElement | null>(null)
  let composerRootElement = $state<HTMLDivElement | null>(null)
  let fromFieldElement = $state<HTMLDivElement | null>(null)
  let toFieldElement = $state<HTMLDivElement | null>(null)
  let ccFieldElement = $state<HTMLDivElement | null>(null)
  let bccFieldElement = $state<HTMLDivElement | null>(null)
  let subjectInputElement = $state<HTMLInputElement | null>(null)
  let initialRichBody = ''  // original reply/forward rich body (images restored), for plain->rich reprocess

  // Component refs
  let toolbarRef = $state<{ focus: () => void } | null>(null)
  let toInputRef = $state<{ focus: () => void } | null>(null)
  let ccInputRef = $state<{ focus: () => void } | null>(null)
  let bccInputRef = $state<{ focus: () => void } | null>(null)

  // Draft auto-save state
  let currentDraftId = $state<string | null>(null)
  let saveStatus = $state<DraftSaveStatus>('idle')

  // Initialize currentDraftId from prop (runs once on mount)
  $effect(() => {
    if (draftId && !currentDraftId) {
      currentDraftId = draftId
    }
  })

  let syncStatus = $state<DraftSyncStatus>('pending') // IMAP sync status
  let unsubscribeDraftSync: (() => void) | null = null
  let lastSavedAt = $state<Date | null>(null)

  // Computed draft status indicator
  let draftStatusMeta = $derived(getDraftStatusMeta(saveStatus, syncStatus, !!lastSavedAt))
  let draftStatusIcon = $derived(draftStatusMeta.icon)
  let draftStatusColor = $derived(draftStatusMeta.color)
  let draftStatusLabel = $derived(draftStatusMeta.labelKey ? $_(draftStatusMeta.labelKey) : '')

  // 10-second debounce like Geary
  const DRAFT_SAVE_DELAY = 10000

  const MENTION_SUGGESTION_LIMIT = 100

  // Whether remote images are blocked in the composer's quoted content
  let composerImagesBlocked = $state(false)

  let mentionActive = $state(false)
  let mentionSurface = $state<MentionSurface>('plain')
  let mentionQuery = $state('')
  let mentionStart = $state(0)
  let mentionEnd = $state(0)
  let mentionSuggestions = $state<contact.Contact[]>([])
  let mentionSelectedIndex = $state(0)
  let mentionKeyboardMode = $state(false)
  let mentionWindowStart = $state(0)
  let mentionTop = $state(12)
  let mentionLeft = $state(12)
  let dismissedMentionKey = $state('')
  let composerTextComposing = $state(false)
  const visibleMentionSuggestions = $derived(mentionSuggestions.slice(mentionWindowStart, mentionWindowStart + MENTION_VISIBLE_ROWS))

  let mentionSelectionState: MentionSelectionState = {
    selectedIndex: 0,
    windowStart: 0,
    keyboardMode: false,
    pointerX: -1,
    pointerY: -1,
  }

  function applyMentionSelectionState(next: MentionSelectionState) {
    mentionSelectionState = next
    mentionSelectedIndex = next.selectedIndex
    mentionWindowStart = next.windowStart
    mentionKeyboardMode = next.keyboardMode
  }

  const mentionSearchController = createMentionSearchController<contact.Contact>({
    delayMs: 150,
    search: (query) => api.searchContacts(query.trim(), MENTION_SUGGESTION_LIMIT),
    onResults: (query, results) => {
      if (!mentionActive || mentionQuery !== query) return
      mentionSuggestions = filteredMentionResults(results, query.trim())
      setMentionSelectedIndex(mentionSuggestions.length > 0 ? 0 : -1)
    },
    onError: (err) => {
      console.error('Failed to search contacts for mention:', err)
      mentionSuggestions = []
      setMentionSelectedIndex(-1)
    },
  })

  $effect(() => {
    applyMentionSelectionState(clampMentionSelection(
      mentionSelectionState,
      mentionSuggestions.length,
      MENTION_VISIBLE_ROWS,
    ))
  })

  // Confirmation dialogs state
  let showEmptySubjectDialog = $state(false)
  let showMissingAttachmentDialog = $state(false)
  let showCloseConfirm = $state(false)
  let showInlineImageSizeDialog = $state(false)
  let inlineImageSizeDescription = $state('')
  let closeLoading = $state<'discard' | 'save' | null>(null)
  type InlineImageChoice = 'inline' | 'attachment' | 'cancel'
  let inlineImageChoiceResolver: ((choice: InlineImageChoice) => void) | null = null
  let imageBatchQueue = Promise.resolve()
  let imageBatchPending = 0
  let untrackedImageScanTimer: ReturnType<typeof setTimeout> | null = null

  // Get only the user-composed text, excluding quoted/forwarded content
  function getUserComposedText(): string {
    if (isPlainTextMode) {
      const cutoff = findPlainQuoteBoundary(plainTextContent)
      if (cutoff >= 0) return plainTextContent.substring(0, cutoff)
      return plainTextContent
    }
    const html = editor?.getHTML() || ''
    const cutoff = findRichQuoteBoundary(html)
    const userHtml = cutoff >= 0 ? html.substring(0, cutoff) : html
    const tmp = document.createElement('div')
    tmp.innerHTML = userHtml
    return tmp.textContent || ''
  }

  // Check if the email body contains keywords that suggest an attachment should be present
  function bodyMentionsAttachment(): boolean {
    const combinedText = getUserComposedText() + ' ' + subject
    return textMentionsAttachment(combinedText)
  }

  function addMentionedContactToRecipients(c: contact.Contact) {
    const email = getContactEmail(c)
    if (!email || hasRecipient(email, toRecipients) || hasRecipient(email, ccRecipients)) return

    toRecipients = [
      ...toRecipients,
      new smtp.Address({
        name: (c.display_name || '').trim(),
        address: (c.email || '').trim(),
      }),
    ]
  }

  function rememberPlainMentionLabel(label: string) {
    const normalized = label.trim()
    if (!normalized || plainMentionLabels.includes(normalized)) return
    plainMentionLabels = [...plainMentionLabels, normalized]
  }

  const plainTextMentionSegments = $derived(getPlainTextMentionSegments(plainTextContent, plainMentionLabels))

  function setMentionSelectedIndex(index: number, inputMode: MentionInputMode = 'program') {
    applyMentionSelectionState(selectMentionIndex(
      mentionSelectionState,
      mentionSuggestions.length,
      index,
      inputMode,
      MENTION_VISIBLE_ROWS,
    ))
  }

  function handleMentionPointerMove(e: PointerEvent, index: number) {
    const result = moveMentionPointerSelection(
      mentionSelectionState,
      mentionSuggestions.length,
      index,
      e.clientX,
      e.clientY,
      MENTION_VISIBLE_ROWS,
    )
    if (result.changed) applyMentionSelectionState(result.state)
  }

  function closeMentionSuggestions() {
    mentionActive = false
    mentionSuggestions = []
    mentionSelectedIndex = 0
    mentionKeyboardMode = false
    mentionWindowStart = 0
    mentionQuery = ''
    mentionSearchController.cancel()
  }

  function scheduleMentionSearch(query: string) {
    const normalizedQuery = query.trim()
    if (!normalizedQuery) {
      mentionSearchController.cancel()
      mentionSuggestions = []
      setMentionSelectedIndex(-1)
      return
    }
    mentionSearchController.schedule(query)
  }

  function setMentionPosition(left: number, top: number) {
    const position = clampMentionPosition({ left, top, container: composerBodyElement })
    mentionLeft = position.left
    mentionTop = position.top
  }

  function updatePlainMentionPosition(markerOffset: number) {
    const textarea = plainTextRef
    const container = composerBodyElement
    if (!textarea || !container) return

    const position = getPlainMentionPosition({ textarea, container, markerOffset })
    setMentionPosition(position.left, position.top)
  }

  function updatePlainMention() {
    const textarea = plainTextRef
    if (!textarea || !isPlainTextMode || composerTextComposing) return

    const cursor = textarea.selectionStart ?? 0
    const token = findMentionToken(plainTextContent.slice(0, cursor))
    if (!token || !token.query.trim()) {
      closeMentionSuggestions()
      dismissedMentionKey = ''
      return
    }

    const shouldSearch =
      !mentionActive ||
      mentionSurface !== 'plain' ||
      mentionQuery !== token.query ||
      mentionStart !== cursor - token.query.length - 1 ||
      mentionEnd !== cursor
    const tokenStart = cursor - token.query.length - 1
    const currentMentionKey = mentionKey('plain', token.query, tokenStart, cursor)
    if (!mentionActive && dismissedMentionKey === currentMentionKey) {
      return
    }
    mentionActive = true
    mentionSurface = 'plain'
    mentionQuery = token.query
    mentionStart = tokenStart
    mentionEnd = cursor
    updatePlainMentionPosition(tokenStart)
    if (shouldSearch) {
      scheduleMentionSearch(token.query)
    }
  }

  function updateRichMention() {
    if (!editor || isPlainTextMode || composerTextComposing) return
    const { state, view } = editor
    const { selection } = state
    if (!selection.empty) {
      closeMentionSuggestions()
      dismissedMentionKey = ''
      return
    }

    const textBeforeCursor = selection.$from.parent.textBetween(0, selection.$from.parentOffset, undefined, '\ufffc')
    const token = findMentionToken(textBeforeCursor)
    if (!token || !token.query.trim()) {
      closeMentionSuggestions()
      dismissedMentionKey = ''
      return
    }

    const tokenStart = selection.from - token.query.length - 1
    const coords = view.coordsAtPos(tokenStart)
    const container = composerBodyElement
    if (container) {
      const rect = container.getBoundingClientRect()
      setMentionPosition(
        coords.left - rect.left + container.scrollLeft,
        coords.bottom - rect.top + container.scrollTop
      )
    }

    const shouldSearch =
      !mentionActive ||
      mentionSurface !== 'rich' ||
      mentionQuery !== token.query ||
      mentionStart !== tokenStart ||
      mentionEnd !== selection.from
    const currentMentionKey = mentionKey('rich', token.query, tokenStart, selection.from)
    if (!mentionActive && dismissedMentionKey === currentMentionKey) {
      return
    }
    mentionActive = true
    mentionSurface = 'rich'
    mentionQuery = token.query
    mentionStart = tokenStart
    mentionEnd = selection.from
    if (shouldSearch) {
      scheduleMentionSearch(token.query)
    }
  }

  function selectMention(c: contact.Contact) {
    const label = getMentionLabel(c)
    if (!label) return

    addMentionedContactToRecipients(c)

    if (mentionSurface === 'plain') {
      const before = plainTextContent.slice(0, mentionStart)
      const after = plainTextContent.slice(mentionEnd)
      const inserted = `@${label}${after.length === 0 || !/^[ \t]/u.test(after) ? ' ' : ''}`
      rememberPlainMentionLabel(label)
      plainTextContent = before + inserted + after
      const nextCursor = before.length + inserted.length
      setTimeout(() => {
        plainTextRef?.focus()
        plainTextRef?.setSelectionRange(nextCursor, nextCursor)
      }, 0)
    } else if (editor) {
      const docSize = editor.state.doc.content.size
      const before = mentionStart > 1
        ? editor.state.doc.textBetween(mentionStart - 1, mentionStart, undefined, '\ufffc')
        : ''
      const after = mentionEnd < docSize
        ? editor.state.doc.textBetween(mentionEnd, Math.min(docSize, mentionEnd + 1), undefined, '\ufffc')
        : ''
      const content: any[] = []
      if (mentionStart > 1 && before && !/\s/u.test(before)) {
        content.push({ type: 'text', text: ' ' })
      }
      content.push({ type: 'contactMention', attrs: { label } })
      if (!after || !/^[ \t]/u.test(after)) {
        content.push({ type: 'text', text: ' ' })
      }

      editor
        .chain()
        .focus()
        .deleteRange({ from: mentionStart, to: mentionEnd })
        .insertContent(content)
        .run()
    }

    closeMentionSuggestions()
    dismissedMentionKey = ''
    scheduleDraftSave()
  }

  function handleMentionKeydown(e: KeyboardEvent): boolean {
    if (e.isComposing) return false
    if (!mentionActive) return false
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      e.stopPropagation()
      if (mentionSuggestions.length > 0) {
        setMentionSelectedIndex(mentionSelectedIndex + 1, 'keyboard')
      }
      return true
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      e.stopPropagation()
      if (mentionSuggestions.length > 0) {
        setMentionSelectedIndex(mentionSelectedIndex - 1, 'keyboard')
      }
      return true
    }
    if (e.key === 'Enter' || e.key === 'Tab') {
      if (mentionSuggestions.length > 0 && mentionSelectedIndex >= 0) {
        e.preventDefault()
        e.stopPropagation()
        selectMention(mentionSuggestions[mentionSelectedIndex])
        return true
      }
    }
    if (e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      dismissedMentionKey = mentionKey(mentionSurface, mentionQuery, mentionStart, mentionEnd)
      closeMentionSuggestions()
      return true
    }
    return false
  }

  function isMentionNavigationKey(key: string): boolean {
    return key === 'ArrowDown' || key === 'ArrowUp' || key === 'Enter' || key === 'Tab' || key === 'Escape'
  }

  function handlePlainTextInput(e?: Event) {
    scheduleDraftSave()
    if ((e as InputEvent | undefined)?.isComposing || composerTextComposing) return
    updatePlainMention()
  }

  function handlePlainTextScroll() {
    const textarea = plainTextRef
    if (!textarea) return
    plainTextScrollTop = textarea.scrollTop
    plainTextScrollLeft = textarea.scrollLeft
  }

  function handlePlainTextKeydown(e: KeyboardEvent) {
    handleMentionKeydown(e)
  }

  async function readClipboardFilePaths(): Promise<string[]> {
    try {
      return await api.getClipboardFilePaths?.() || []
    } catch {
      return []
    }
  }

  function handlePlainTextPaste(e: ClipboardEvent) {
    const payload = readComposerClipboardFiles(e.clipboardData)
    if (payload.files.length > 0) {
      e.preventDefault()
      void queueComposerFiles(payload.files, false)
      return
    }
    if (payload.paths.length > 0) {
      e.preventDefault()
      void handlePastedAttachmentPaths(payload.paths)
      return
    }

    if (payload.advertisesFiles) e.preventDefault()
    void readClipboardFilePaths().then(paths => {
      if (paths.length > 0) void handlePastedAttachmentPaths(paths)
    })
  }

  function handleBodyKeydown(e: KeyboardEvent) {
    handleMentionKeydown(e)
  }

  function handlePlainTextCursorChange(e?: Event) {
    if (composerTextComposing) return
    if (e instanceof KeyboardEvent && isMentionNavigationKey(e.key)) return
    if (e instanceof KeyboardEvent && e.isComposing) return
    setTimeout(updatePlainMention, 0)
  }

  function handleRichEditorUpdate(e?: KeyboardEvent) {
    if (composerTextComposing) return
    if (mentionActive && e && isMentionNavigationKey(e.key)) return
    if (e?.isComposing) return
    scheduleDraftSave()
    updateRichMention()
    scheduleUntrackedInlineImageScan()
  }

  function handleComposerCompositionStart() {
    composerTextComposing = true
    closeMentionSuggestions()
    dismissedMentionKey = ''
  }

  function handleComposerCompositionEnd() {
    composerTextComposing = false
    scheduleDraftSave()
    setTimeout(() => {
      if (isPlainTextMode) {
        updatePlainMention()
      } else {
        updateRichMention()
      }
    }, 0)
  }

  function handleMentionMenuWheel(e: WheelEvent) {
    e.stopPropagation()
  }

  function handleMentionMenuTouchMove(e: TouchEvent) {
    e.stopPropagation()
  }

  function handleMentionBlur() {
    setTimeout(() => {
      if (!document.activeElement?.closest?.('[data-composer-mention-menu]')) {
        closeMentionSuggestions()
      }
    }, 150)
  }

  function getDisplayMode(): 'new' | 'reply' | 'reply-all' | 'forward' {
    return getComposerDisplayMode({ initialMessage, replyType })
  }

  function composerDarkFilterAvailable(): boolean {
    return getDisplayMode() !== 'new' &&
      !isPlainTextMode &&
      getDarkMailContent() &&
      getIsDarkActive()
  }

  function composerDarkFilterActive(): boolean {
    return composerDarkFilterAvailable() && composerDarkFilterEnabled
  }

  function toggleComposerDarkFilter() {
    composerDarkFilterEnabled = !composerDarkFilterEnabled
  }

  function composerBodyStyle(): string {
    return composerDarkFilterActive()
      ? `background-color: ${getDarkMailSurfaceBackground()};`
      : ''
  }

  function composerEditorStyle(): string {
    if (!composerDarkFilterActive()) return ''
    const styles = buildDarkMailFilterStyles()
    return `--composer-dark-content-filter: ${styles.contentFilter}; --composer-dark-media-filter: ${styles.mediaFilter};`
  }

  function hasContent(): boolean {
    return composerHasContent({
      toCount: toRecipients.length,
      ccCount: ccRecipients.length,
      bccCount: bccRecipients.length,
      subject,
      isPlainTextMode,
      plainTextContent,
      editor,
      attachments,
    })
  }

  function addInlineImage(candidate: InlineImage): { image: InlineImage, added: boolean } {
    const result = registerInlineImage(inlineImages, candidate)
    inlineImages = result.images
    return { image: result.image, added: result.added }
  }

  interface UntrackedInlineImage {
    source: string
    image: InlineImage
  }

  function hasPotentialUntrackedInlineImages(): boolean {
    const editorEl = editor?.view?.dom
    if (!editorEl) return false
    return Array.from(editorEl.querySelectorAll('img')).some(img => {
      const src = img.getAttribute('src') || ''
      if (!src || src.startsWith('cid:') || src.startsWith('http://') || src.startsWith('https://')) return false
      if (img.hasAttribute('data-original-src')) return false
      return !inlineImages.some(image => image.dataUrl === src)
    })
  }

  // WebKit can insert data:, blob:, or webkit-fake-url images without exposing
  // File objects. Normalize those images so they enter the same 5/10 MiB batch
  // policy as Finder paste, drag-and-drop, and the image picker.
  function findUntrackedInlineImages(): UntrackedInlineImage[] {
    const editorEl = editor?.view?.dom
    if (!editorEl) return []

    const imgs = editorEl.querySelectorAll('img')
    const results: UntrackedInlineImage[] = []
    const seenData = new Set(inlineImages.map(image => image.data))

    for (const img of imgs) {
      const src = img.getAttribute('src') || ''

      // Skip tracked, cid:, http(s):, and blocked remote images
      if (src.startsWith('cid:') || src.startsWith('http://') || src.startsWith('https://')) continue
      if (img.hasAttribute('data-original-src')) continue
      if (inlineImages.some(i => i.dataUrl === src)) continue

      let candidate: InlineImage | null
      if (src.startsWith('data:')) {
        const cid = generateCID()
        candidate = createInlineImageFromDataUrl({
          cid,
          dataUrl: src,
          counter: inlineImageCounter,
          fallbackPrefix: 'pasted-image',
        })
      } else {
        // webkit-fake-url://, blob:, etc. — extract via canvas.
        if (!img.complete || img.naturalWidth === 0) continue
        try {
          const canvas = document.createElement('canvas')
          canvas.width = img.naturalWidth
          canvas.height = img.naturalHeight
          const ctx = canvas.getContext('2d')
          if (!ctx) continue
          ctx.drawImage(img, 0, 0)
          const cid = generateCID()
          candidate = createInlineImageFromDataUrl({
            cid,
            dataUrl: canvas.toDataURL('image/png'),
            counter: inlineImageCounter,
            fallbackPrefix: 'pasted-image',
          })
        } catch {
          continue
        }
      }

      if (!candidate || seenData.has(candidate.data)) continue
      seenData.add(candidate.data)
      results.push({ source: src, image: candidate })
    }

    return results
  }

  // Convert HTML with data URLs to use CID references for inline images
  function convertDataUrlsToCid(html: string): string {
    return replaceInlineImageSourcesWithCids(html, inlineImages)
  }

  // Build message object from current composer state
  function buildMessage(): smtp.ComposeMessage {
    const selectedIdentity = identities.find(i => i.id === selectedIdentityId)

    // Handle plain text vs rich text mode
    let htmlContent: string
    let textContent: string

    if (isPlainTextMode) {
      // In plain text mode, we only have plain text
      textContent = plainTextContent
      htmlContent = ''  // No HTML version when composing in plain text
    } else {
      // In rich text mode, we have both
      // Untracked WebKit images are normalized asynchronously before this
      // synchronous message builder runs.
      const rawHtml = editor?.getHTML() || ''
      htmlContent = convertDataUrlsToCid(addParagraphStyles(rawHtml))
      textContent = editor?.getText() || ''
    }

    return buildComposeMessage({
      identity: selectedIdentity,
      to: toRecipients,
      cc: ccRecipients,
      bcc: bccRecipients,
      subject,
      htmlBody: restoreBlockedRemoteImages(htmlContent),
      textBody: textContent,
      attachments,
      inlineImages,
      inReplyTo,
      references: references,
      sourceMessageId,
      replyType,
      requestReadReceipt,
    })
  }

  // Get a content hash to detect meaningful changes
  function getContentHash(): string {
    const bodyContent = isPlainTextMode ? plainTextContent : (editor?.getHTML() || '')
    const attachmentNames = attachments.map(a => a.filename).join(',')
    return buildDraftContentHash({
      toCount: toRecipients.length,
      ccCount: ccRecipients.length,
      bccCount: bccRecipients.length,
      subject,
      bodyContent,
      attachmentNames,
      isPlainTextMode,
    })
  }

  const draftSaveController = createDraftSaveController({
    delayMs: DRAFT_SAVE_DELAY,
    hasContent,
    getContentHash,
    hasPersistedDraft: () => !!currentDraftId,
    getStatus: () => saveStatus,
    setStatus: (status) => { saveStatus = status },
    save: async () => {
      const message = buildMessage()
      const result = await api.saveDraft(activeAccountId, message, currentDraftId || '')
      currentDraftId = result.id
      syncStatus = result.syncStatus as DraftSyncStatus
      lastSavedAt = new Date()
    },
    onError: (err) => {
      console.error('Failed to save draft:', err)
    },
  })

  function scheduleDraftSave() {
    draftSaveController.schedule()
  }

  async function saveDraft() {
    await draftSaveController.saveNow()
  }

  // Load blocked remote images in the composer editor
  function loadComposerImages() {
    if (!editor) return
    const imgs = editor.view.dom.querySelectorAll('img[data-original-src]')
    imgs.forEach(img => {
      const originalSrc = img.getAttribute('data-original-src')
      if (originalSrc) {
        img.setAttribute('src', originalSrc)
        img.removeAttribute('data-original-src')
      }
    })
    composerImagesBlocked = false
  }

  // Delete the current draft
  async function deleteDraft() {
    if (!currentDraftId) return

    try {
      await api.deleteDraft(currentDraftId)
      currentDraftId = null
    } catch (err) {
      console.error('Failed to delete draft:', err)
    }
  }

  // Watch for content changes and trigger auto-save
  $effect(() => {
    // Dependencies to watch
    const _ = [toRecipients, ccRecipients, bccRecipients, subject]
    // untrack prevents $effect from creating a reactive dependency on saveStatus
    // (which scheduleDraftSave reads), avoiding a circular re-run that causes flash
    untrack(() => scheduleDraftSave())
  })

  // Track current signature for swapping when identity changes
  // Apply read receipt policy from account settings
  function applyReadReceiptPolicy(policy: string) {
    switch (policy) {
      case 'always':
        requestReadReceipt = true
        showReadReceiptOption = false
        break
      case 'ask':
        requestReadReceipt = false
        showReadReceiptOption = true
        break
      default:
        requestReadReceipt = false
        showReadReceiptOption = false
    }
  }

  // Initialize
  onMount(async () => {
    // Load identities — try cross-account first (main window), fall back to single-account (detached)
    try {
      // When replying or forwarding on a no-outgoing account, honor that
      // account's configured Reply/Forward-with identity (if any). Captured
      // from the pre-filter list because no-outgoing accounts are removed
      // before the picker sees them.
      let replyForwardIdentity: account.Identity | null = null

      if (api.getAllAccountIdentities) {
        const groups = await api.getAllAccountIdentities()
        const sourceGroup = (groups || []).find(g => g.account?.id === accountId)
        // Exclude receive-only accounts: their identities can't actually
        // be used as a From address, so they don't belong in the picker.
        allGroups = (groups || []).filter(g => !g.account?.noOutgoingServer)
        identities = allGroups.flatMap(g => g.identities || [])

        const sourceReplyForwardId = sourceGroup?.account?.noOutgoingServer
          ? ((sourceGroup.account as any).replyForwardIdentityId || '')
          : ''
        if (sourceReplyForwardId) {
          replyForwardIdentity = identities.find(i => i.id === sourceReplyForwardId) || null
        }
      }
      if (!api.getAllAccountIdentities) {
        // Detached window — single account only
        identities = await api.getIdentities(accountId)
      }

      // Select identity: explicit recipient match wins; then the source
      // account's Reply/Forward-with preference (no-outgoing accounts
      // only); then this account's default identity; then the first
      // available identity as the ultimate fallback.
      const matchedIdentity = selectIdentityForReply()
      const accountIdentities = identities.filter(i => i.accountId === accountId)
      const defaultIdentity = accountIdentities.find(i => i.isDefault) || accountIdentities[0]
      const selectedIdentity = matchedIdentity || replyForwardIdentity || defaultIdentity || identities[0]
      if (selectedIdentity) {
        selectedIdentityId = selectedIdentity.id
      }
    } catch (err) {
      console.error('Failed to load identities:', err)
    }

    // Load account's read receipt request policy (use activeAccountId which derives from selected identity)
    try {
      const acc = await api.getAccount(activeAccountId)
      applyReadReceiptPolicy(acc.readReceiptRequestPolicy || 'never')
    } catch (err) {
      console.error('Failed to load account settings:', err)
    }

    // Initialize TipTap editor (no-op in plain text mode — its element isn't
    // mounted yet; created lazily when the user switches to rich text).
    ensureEditor()

    // Initialize from initialMessage if provided (reply/forward)
    if (initialMessage) {
      initializeFromMessage()
      // Store initial content hash so we don't immediately save
      draftSaveController.seed(getContentHash())
    }

    // Append signature for the selected identity (after editor is ready)
    // Only if signature doesn't already exist in content (e.g., from loaded draft)
    // Then focus the To field once everything is initialized
    setTimeout(() => {
      const identity = identities.find(i => i.id === selectedIdentityId)
      if (identity) {
        const content = editor?.getHTML() || ''
        // Don't append if signature marker already exists in the user's compose area.
        // Only check content before the quoted section — markers inside quoted history
        // (from previous replies with signatures) should not prevent injection.
        const quoteStart = findRichQuoteBoundary(content)
        const quoteBoundary = quoteStart >= 0 ? quoteStart : content.length
        const preQuoteContent = content.substring(0, quoteBoundary)
        if (!hasSignatureMarker(preQuoteContent)) {
          appendSignatureForIdentity(identity)
        }
      }
      // Focus editor body for reply/reply-all, To field for new/forward
      const mode = getDisplayMode()
      switch (mode) {
        case 'reply':
        case 'reply-all':
          // Plain text mode shows a textarea (the editor is hidden), so focus
          // that — cursor at the top, above the quoted original.
          if (isPlainTextMode) {
            plainTextRef?.focus()
            plainTextRef?.setSelectionRange(0, 0)
            break
          }
          editor?.commands.focus('start')
          break
        default:
          toInputRef?.focus()
      }
    }, 50)

    // Listen for draft sync status changes from backend
    unsubscribeDraftSync = EventsOn('draft:syncStatusChanged', (data: { draftId: string, syncStatus: string, imapUid: number, error: string }) => {
      if (data.draftId === currentDraftId) {
        syncStatus = data.syncStatus as 'pending' | 'synced' | 'failed'
      }
    })
  })

  // Select identity based on the From address the backend determined for reply/forward
  function selectIdentityForReply(): account.Identity | null {
    if (!initialMessage) return null

    // PrepareReply already determines the correct From based on the account
    // that owns the message. Match it to a local identity.
    const fromEmail = ((initialMessage.from as any)?.address || (initialMessage.from as any)?.email || '').toLowerCase()
    if (!fromEmail) return null

    return identities.find(identity =>
      identity.email.toLowerCase() === fromEmail
    ) || null
  }

  // Append signature for the current identity based on compose mode
  function appendSignatureForIdentity(identity: account.Identity) {
    const mode = getDisplayMode()
    if (!shouldAppendSignature(identity, mode)) return

    // Plain-text composer: the visible surface is the textarea (plainTextContent),
    // not the hidden rich editor — insert the signature there.
    if (isPlainTextMode) {
      let sig = (identity.signatureText || '').trim()
      if (!sig && identity.signatureHtml) sig = htmlToPlainText(identity.signatureHtml).trim()
      if (!sig) return
      const separator = getSignatureSeparator(identity)
      if (separator) sig = separator + '\n' + sig

      const body = plainTextContent
      const quoteStart = findPlainQuoteBoundary(body)

      if (quoteStart >= 0) {
        plainTextContent = body.slice(0, quoteStart).replace(/\s*$/, '\n\n') + sig + '\n\n' + body.slice(quoteStart)
      } else {
        plainTextContent = body ? body.replace(/\s*$/, '\n\n') + sig : '\n\n' + sig
      }
      return
    }

    if (!editor) return

    const signatureHtml = buildSignatureHtml(identity)
    if (!signatureHtml) return

    const content = editor.getHTML()
    const newContent = insertSignatureIntoContent(
      content,
      signatureHtml,
      mode,
      'above'
    )

    editor.commands.setContent(newContent)
  }

  // Handle identity change from the From dropdown
  function handleIdentityChange(newIdentityId: string) {
    if (newIdentityId === selectedIdentityId) return

    const newIdentity = identities.find(i => i.id === newIdentityId)
    const oldAccountId = activeAccountId
    selectedIdentityId = newIdentityId

    if (!editor || !newIdentity) return

    // If account changed, reload read receipt policy and migrate draft
    if (newIdentity.accountId !== oldAccountId) {
      api.getAccount(newIdentity.accountId).then(acc => {
        applyReadReceiptPolicy(acc.readReceiptRequestPolicy || 'never')
      }).catch(err => {
        console.error('Failed to load account settings:', err)
      })

      // Delete old draft (belongs to previous account) and clear ID
      // so the next save creates a fresh draft under the new account
      if (currentDraftId) {
        const oldDraftId = currentDraftId
        currentDraftId = null
        draftSaveController.seed('')
        api.deleteDraft(oldDraftId).catch(err => {
          console.error('Failed to delete old account draft:', err)
        })
      }
    }

    // Remove old signature and apply new one
    const content = removeSignatureFromContent(editor.getHTML())
    editor.commands.setContent(content)

    appendSignatureForIdentity(newIdentity)
    scheduleDraftSave()
  }

  onDestroy(() => {
    // Unsubscribe from draft sync events
    unsubscribeDraftSync?.()
    unsubscribeDraftSync = null
    mentionSearchController.destroy()
    draftSaveController.destroy()
    if (untrackedImageScanTimer) clearTimeout(untrackedImageScanTimer)
    untrackedImageScanTimer = null
    inlineImageChoiceResolver?.('cancel')
    inlineImageChoiceResolver = null
    editor?.destroy()
  })

  // Initialize composer fields from the pre-built message (from backend)
  function initializeFromMessage() {
    if (!initialMessage) return

    // Set recipients - ensure proper smtp.Address objects
    // The backend returns smtp.Address with 'address' field, but we need to handle
    // any edge cases where plain objects come through
    toRecipients = (initialMessage.to || []).map(toSmtpAddress)
    ccRecipients = (initialMessage.cc || []).map(toSmtpAddress)
    bccRecipients = (initialMessage.bcc || []).map(toSmtpAddress)

    // Show Cc field if there are Cc recipients
    if (ccRecipients.length > 0) {
      showCc = true
    }

    // Set subject
    subject = initialMessage.subject || ''

    // Set threading headers
    inReplyTo = initialMessage.in_reply_to
    references = initialMessage.references || []
    sourceMessageId = (initialMessage as any).source_message_id || ''
    const initialReplyType = (initialMessage as any).reply_type || ''
    replyType = initialReplyType === 'reply' || initialReplyType === 'reply-all' || initialReplyType === 'forward'
      ? initialReplyType
      : ''

    // Restore attachments and inline images from draft/reply/forward
    // Go []byte is serialized as base64 string via JSON, but TS type says number[]
    // content_base64 is used for efficient Wails RPC transfer (inline images in replies/forwards)
    let htmlBody = initialMessage.html_body || ''
    if (initialMessage.attachments?.length > 0) {
      for (const att of initialMessage.attachments) {
        const base64Data = att.content_base64 || (att.content as unknown as string)
        if (!base64Data) continue

        if (att.inline && att.content_id) {
          // Inline image - restore to inlineImages array and replace CID with data URL
          const dataUrl = `data:${att.content_type};base64,${base64Data}`
          inlineImages = [...inlineImages, {
            cid: att.content_id,
            dataUrl,
            contentType: att.content_type,
            data: base64Data,
            filename: att.filename,
            size: base64DecodedSize(base64Data),
          }]
          htmlBody = htmlBody.replaceAll(`cid:${att.content_id}`, dataUrl)
        } else if (!att.inline) {
          // Regular attachment
          attachments = [...attachments, {
            filename: att.filename,
            contentType: att.content_type,
            size: base64Data.length,
            data: base64Data,
          }]
        }
      }
      // Ensure new inline images get unique CIDs
      inlineImageCounter = Math.max(inlineImageCounter, inlineImages.length)
    }

    // Set editor content (with restored data URLs for inline images)
    // Strip email-client paragraph styles so TipTap doesn't double-space empty lines
    if (editor && htmlBody) {
      editor.commands.setContent(stripParagraphStyles(htmlBody))
      // Move cursor to beginning (before the quoted content)
      editor.commands.focus('start')
    }

    // Keep the original rich quote/forward body (images already restored) so a
    // later plain -> rich switch can reprocess the original instead of the
    // lossy plain-text conversion (which permanently drops images).
    initialRichBody = htmlBody

    // Plaintext mode shows a textarea bound to plainTextContent (not the editor),
    // so seed it with the backend's plaintext quote — otherwise replies/forwards
    // open with an empty body when the composer defaults to plaintext.
    if (isPlainTextMode) {
      plainTextContent = initialMessage.text_body || ''
    }

    // Check for blocked remote images in quoted content.
    // If the sender is allowlisted or always-load is enabled, unblock immediately.
    if (htmlBody.includes('data-original-src')) {
      composerImagesBlocked = true
      const senderEmail = ((initialMessage.from as any)?.address || (initialMessage.from as any)?.email || '').toLowerCase()
      if (getAlwaysLoadImages() || imagesLoaded || (senderEmail && isImageAllowedSync(senderEmail))) {
        // Use setTimeout to ensure editor has rendered the content first
        setTimeout(() => loadComposerImages(), 0)
      }
    }

  }

  async function handleSend() {
    const selectedIdentity = identities.find(i => i.id === selectedIdentityId)
    const blocker = getComposerSendBlocker({
      recipientCount: toRecipients.length,
      hasIdentity: !!selectedIdentity,
      attachmentCount: attachments.length,
      mentionsAttachment: bodyMentionsAttachment(),
      subject,
    })
    switch (blocker) {
      case 'no-recipients':
        addToast({ type: 'error', message: $_('composer.noRecipients') })
        return
      case 'missing-identity':
        addToast({ type: 'error', message: $_('composer.selectSenderIdentity') })
        return
      case 'missing-attachment':
        showMissingAttachmentDialog = true
        return
      case 'empty-subject':
        showEmptySubjectDialog = true
        return
    }

    await doSend()
  }

  // Actually send the message (called directly or after confirmation)
  async function doSend() {
    draftSaveController.cancelPending()
    await draftSaveController.waitForIdle()

    sending = true

    try {
      const needsImagePreparation = imageBatchPending > 0 || (!isPlainTextMode && (
        untrackedImageScanTimer !== null || hasPotentialUntrackedInlineImages()
      ))
      if (needsImagePreparation) {
        if (untrackedImageScanTimer) {
          clearTimeout(untrackedImageScanTimer)
          untrackedImageScanTimer = null
        }
        await imageBatchQueue
        if (!await processUntrackedInlineImages(false)) return
      }
      if (!isPlainTextMode && !await enforceTrackedInlineImagePolicyBeforeSend()) return

      const message = buildMessage()
      await api.sendMessage(activeAccountId, message)

      // Delete the draft on successful send (fire-and-forget - don't block UI)
      if (currentDraftId) {
        deleteDraft().catch(err => console.error('Failed to delete draft after send:', err))
      }

      addToast({
        type: 'success',
        message: $_('composer.messageSent'),
      })

      onSent?.()
      onClose?.()
    } catch (err) {
      console.error('Failed to send message:', err)
      addToast({
        type: 'error',
        message: $_('composer.failedToSend'),
      })
    } finally {
      sending = false
    }
  }

  // Handlers for confirmation dialogs
  function requestInlineImageChoice(projectedBytes: number, count: number): Promise<InlineImageChoice> {
    inlineImageChoiceResolver?.('cancel')
    inlineImageSizeDescription = $_('composer.inlineImageSizeDescription', {
      values: { size: formatInlineImageSize(projectedBytes), count },
    })
    showInlineImageSizeDialog = true
    return new Promise(resolve => {
      inlineImageChoiceResolver = resolve
    })
  }

  function resolveInlineImageChoice(choice: InlineImageChoice) {
    const resolve = inlineImageChoiceResolver
    if (!resolve) return
    inlineImageChoiceResolver = null
    showInlineImageSizeDialog = false
    resolve(choice)
  }

  function handleConfirmEmptySubject() {
    showEmptySubjectDialog = false
    // Check for missing attachment next (if applicable)
    if (attachments.length === 0 && bodyMentionsAttachment()) {
      showMissingAttachmentDialog = true
    } else {
      doSend()
    }
  }

  function handleConfirmMissingAttachment() {
    showMissingAttachmentDialog = false
    doSend()
  }

  function handleClose() {
    draftSaveController.cancelPending()

    // Always show confirmation dialog (even for empty content, since a draft may have been saved)
    showCloseConfirm = true
  }

  // Discard: Delete draft from local DB and IMAP, then close
  async function handleDiscardAndClose() {
    draftSaveController.setDiscarding(true)
    await draftSaveController.waitForIdle()
    closeLoading = 'discard'
    try {
      await discardDraftBeforeClose(currentDraftId, api.deleteDraft, () => {
        currentDraftId = null
        showCloseConfirm = false
        onClose?.()
      })
    } catch (err) {
      console.error('Failed to delete draft:', err)
      draftSaveController.setDiscarding(false)
      addToast({
        type: 'error',
        message: $_('composer.failedToDiscardDraft'),
      })
    } finally {
      closeLoading = null
    }
  }

  // Save & Close: Save current content as draft, then close
  async function handleSaveAndClose() {
    closeLoading = 'save'
    try {
      if (hasContent()) {
        await saveDraft()
      }
    } catch (err) {
      console.error('Failed to save draft:', err)
      // Still close even if save fails
    }
    showCloseConfirm = false
    closeLoading = null
    onClose?.()
  }

  // Keep Editing: Just close the dialog
  function handleKeepEditing() {
    showCloseConfirm = false
  }

  // Insert image via file picker
  function insertImage() {
    // Create a hidden file input, append to DOM (required for WebKitGTK),
    // then click it to open the file picker
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = 'image/*'
    input.style.display = 'none'
    document.body.appendChild(input)
    input.onchange = async (e) => {
      input.remove()
      const file = (e.target as HTMLInputElement).files?.[0]
      if (file) {
        await queueComposerFiles([file], true)
      }
    }
    input.click()
  }

  function handleComposerTabKeydown(e: KeyboardEvent) {
    const refs: ComposerFocusRefs = {
      fromFieldElement,
      toFieldElement,
      ccFieldElement,
      bccFieldElement,
      subjectInputElement,
      composerBodyElement,
      plainTextRef,
      toInputRef,
      ccInputRef,
      bccInputRef,
      editor,
      isPlainTextMode,
    }
    handleComposerTabNavigation(e, refs, {
      showCc,
      showBcc,
      disabled: !composerRootElement ||
        showEmptySubjectDialog ||
        showMissingAttachmentDialog ||
        showInlineImageSizeDialog ||
        showCloseConfirm,
      activeElement: document.activeElement,
    })
  }

  // Create the TipTap editor on demand. No-op if it already exists or its
  // element isn't mounted yet — it isn't while the composer is in plain text
  // mode, since the editor <div> lives in the {:else} branch.
  function ensureEditor() {
    if (editor || !editorElement) return
    editor = createComposerEditor(editorElement, {
      onUpdate: handleRichEditorUpdate,
      onMentionKeyDown: handleMentionKeydown,
      onFiles: (files) => queueComposerFiles(files, true),
      onFilePaths: handleDroppedFilePaths,
      readClipboardFilePaths,
      onShiftTab: () => document.getElementById('composer-subject')?.focus(),
      isEnhancedKeyboardNavigationEnabled: getEnhancedKeyboardNavigation,
      getDarkFilterMode: getDisplayMode,
    })
  }

  // Toggle between rich text and plain text mode. Both surfaces stay mounted
  // (the template toggles visibility), so the editor is always live and
  // setContent lands reliably — no async, no teardown, no race.
  function togglePlainTextMode() {
    // Rich -> plain
    if (!isPlainTextMode) {
      plainTextContent = htmlToPlainText(editor?.getHTML() || '')
      isPlainTextMode = true
      scheduleDraftSave()
      return
    }
    // Plain -> rich. Plain text can't carry images, and converting the degraded
    // plain text back to HTML would drop them permanently. So for a FRESH
    // reply/forward, REPROCESS the original: keep the user's typed intro but
    // restore the original rich quote/forward body (with images). Skipped for
    // drafts (whose saved body already includes the intro) and when the quote
    // boundary is gone, to avoid duplicating the body — those fall back to a
    // straight plain-text conversion.
    const hasQuote = hasPlainQuoteBoundary(plainTextContent)
    let html: string
    if (!draftId && initialRichBody && hasQuote) {
      const intro = getUserComposedText().trim()
      html = (intro ? plainTextToHtml(intro) : '') + initialRichBody
    } else {
      html = plainTextToHtml(plainTextContent)
    }
    editor?.commands.setContent(stripParagraphStyles(html))
    isPlainTextMode = false
    editor?.commands.focus('start')
    scheduleDraftSave()
  }

  // Keyboard shortcuts
  function focusComposerBody() {
    if (isPlainTextMode) {
      plainTextRef?.focus()
      return
    }
    editor?.commands.focus()
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.defaultPrevented || !getEnhancedKeyboardNavigation()) return

    if (e.key === 'F10' && e.shiftKey && !e.ctrlKey && !e.metaKey && !e.altKey) {
      e.preventDefault()
      keyboardActionMenu.showForRoot(composerRootElement)
      return
    }
    if (e.key === 'Tab') {
      handleComposerTabKeydown(e)
      return
    }
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault()
      handleSend()
    }
    // Alt+T to focus toolbar (hint mode)
    if (isOptionCodeShortcut(e, 'KeyT')) {
      e.preventDefault()
      toolbarRef?.focus()
    }
    // Alt+A to attach files
    if (isOptionCodeShortcut(e, 'KeyA')) {
      e.preventDefault()
      handleAttachFiles()
    }
    if (e.key === 'Escape') {
      handleClose()
    }
  }

  // Generate a unique Content-ID for inline images
  function generateCID(): string {
    inlineImageCounter++
    return createInlineImageCID(inlineImageCounter)
  }

  function scheduleUntrackedInlineImageScan() {
    if (untrackedImageScanTimer) clearTimeout(untrackedImageScanTimer)
    untrackedImageScanTimer = setTimeout(() => {
      untrackedImageScanTimer = null
      void enqueueImageBatch(() => processUntrackedInlineImages(true).then(() => {}))
    }, 0)
  }

  function enqueueImageBatch(run: () => Promise<void>): Promise<void> {
    imageBatchPending++
    const task = imageBatchQueue.then(run).finally(() => {
      imageBatchPending--
    })
    imageBatchQueue = task.catch(() => {})
    return task
  }

  async function chooseInlineImageHandling(
    images: Array<{ data: string, size?: number }>,
    current: Array<{ data: string, size?: number }> = inlineImages,
  ) {
    const policy = evaluateInlineImageBatch(current, images)
    let choice: InlineImageChoice = policy.decision === 'attachment' ? 'attachment' : 'inline'
    if (policy.decision === 'confirm' && policy.projectedBytes > acknowledgedInlineImageBytes) {
      choice = await requestInlineImageChoice(policy.projectedBytes, images.length)
    }
    if (choice === 'inline') {
      acknowledgedInlineImageBytes = Math.max(acknowledgedInlineImageBytes, policy.projectedBytes)
    }
    return { choice, policy }
  }

  function notifyAutomaticImageAttachments(projectedBytes: number, count: number) {
    addToast({
      type: 'info',
      message: $_('composer.inlineImagesAttachedAutomatically', {
        values: { size: formatInlineImageSize(projectedBytes), count },
      }),
    })
  }

  function updateUntrackedImageMarkup(entries: UntrackedInlineImage[], keepInline: boolean) {
    if (!editor || entries.length === 0) return
    const container = document.createElement('div')
    container.innerHTML = editor.getHTML()
    const bySource = new Map(entries.map(entry => [entry.source, entry.image]))
    for (const img of container.querySelectorAll('img')) {
      const source = img.getAttribute('src') || ''
      const image = bySource.get(source)
      if (!image) continue
      if (keepInline) {
        img.setAttribute('src', image.dataUrl)
      } else {
        img.remove()
      }
    }
    editor.commands.setContent(container.innerHTML)
  }

  async function processUntrackedInlineImages(removeOnCancel: boolean): Promise<boolean> {
    const entries = findUntrackedInlineImages()
    if (entries.length === 0) return true

    const { choice, policy } = await chooseInlineImageHandling(entries.map(entry => entry.image))
    if (choice === 'cancel') {
      if (removeOnCancel) {
        updateUntrackedImageMarkup(entries, false)
        scheduleDraftSave()
      }
      return false
    }

    if (choice === 'attachment') {
      attachments = [
        ...attachments,
        ...entries.map(({ image }) => ({
          filename: image.filename,
          contentType: image.contentType,
          size: image.size,
          data: image.data,
        })),
      ]
      updateUntrackedImageMarkup(entries, false)
      if (policy.decision === 'attachment') {
        notifyAutomaticImageAttachments(policy.projectedBytes, entries.length)
      }
    } else {
      for (const { image } of entries) addInlineImage(image)
      updateUntrackedImageMarkup(entries, true)
    }

    scheduleDraftSave()
    return true
  }

  function convertTrackedInlineImagesToAttachments() {
    if (!editor || inlineImages.length === 0) return
    const converted = inlineImages.map(image => ({
      filename: image.filename,
      contentType: image.contentType,
      size: image.size,
      data: image.data,
    }))
    const inlineSources = new Set(inlineImages.map(image => image.dataUrl))
    const container = document.createElement('div')
    container.innerHTML = editor.getHTML()
    for (const img of container.querySelectorAll('img')) {
      if (inlineSources.has(img.getAttribute('src') || '')) img.remove()
    }
    editor.commands.setContent(container.innerHTML)
    attachments = [...attachments, ...converted]
    inlineImages = []
    acknowledgedInlineImageBytes = 0
    scheduleDraftSave()
  }

  async function enforceTrackedInlineImagePolicyBeforeSend(): Promise<boolean> {
    if (inlineImages.length === 0) return true
    const images = [...inlineImages]
    const { choice, policy } = await chooseInlineImageHandling(images, [])
    if (choice === 'cancel') return false
    if (choice === 'inline') return true

    convertTrackedInlineImagesToAttachments()
    if (policy.decision === 'attachment') {
      notifyAutomaticImageAttachments(policy.projectedBytes, images.length)
    }
    return true
  }

  function queueComposerFiles(files: File[], inlineEligible: boolean): Promise<void> {
    return enqueueImageBatch(() => processComposerFiles(files, inlineEligible))
  }

  function queueComposerPaths(paths: string[], inlineEligible: boolean): Promise<void> {
    return enqueueImageBatch(() => processComposerPaths(paths, inlineEligible))
  }

  async function processComposerFiles(files: File[], inlineEligible: boolean) {
    const prepared: ComposerAttachment[] = []
    for (const file of files) {
      try {
        prepared.push(await fileToComposerAttachment(file))
      } catch (err) {
        if (file.type.startsWith('image/')) {
          console.error('Failed to process inline image:', err)
          addToast({ type: 'error', message: $_('composer.failedToInsertImage') })
        } else {
          console.error('Failed to read dropped file:', err)
        }
      }
    }
    await processComposerAttachments(prepared, inlineEligible)
  }

  async function processComposerPaths(paths: string[], inlineEligible: boolean) {
    const prepared: ComposerAttachment[] = []
    for (const filePath of paths) {
      try {
        const attachment = await api.readFileAsAttachment(filePath)
        if (attachment) prepared.push(backendAttachmentToComposerAttachment(attachment))
      } catch {
        // Continue through a multi-file Finder copy when one path becomes
        // unreadable between copy and paste.
      }
    }
    await processComposerAttachments(prepared, inlineEligible)
  }

  function insertInlineImageAttachments(images: ComposerAttachment[]) {
    for (const image of images) {
      const dataUrl = `data:${image.contentType};base64,${image.data}`
      const cid = generateCID()
      const registered = addInlineImage(createInlineImageFromAttachment({
        cid,
        dataUrl,
        contentType: image.contentType,
        data: image.data,
        filename: image.filename,
        size: image.size,
      }))
      editor?.chain().focus().setImage({
        src: registered.image.dataUrl,
        alt: registered.image.filename,
      }).run()
    }
  }

  async function processComposerAttachments(prepared: ComposerAttachment[], inlineEligible: boolean) {
    if (prepared.length === 0) return
    if (!inlineEligible) {
      attachments = [...attachments, ...prepared]
      scheduleDraftSave()
      return
    }

    const images = prepared.filter(attachment => attachment.contentType.startsWith('image/'))
    const regularAttachments = prepared.filter(attachment => !attachment.contentType.startsWith('image/'))
    let changed = false

    if (regularAttachments.length > 0) {
      attachments = [...attachments, ...regularAttachments]
      changed = true
    }

    if (images.length > 0) {
      const { choice, policy } = await chooseInlineImageHandling(images)

      if (choice === 'attachment') {
        attachments = [...attachments, ...images]
        changed = true
        if (policy.decision === 'attachment') {
          notifyAutomaticImageAttachments(policy.projectedBytes, images.length)
        }
      } else if (choice === 'inline') {
        insertInlineImageAttachments(images)
        changed = true
      }
    }

    if (changed) scheduleDraftSave()
  }

  // Plain-text messages cannot carry inline images. Finder-copied paths are
  // therefore attached as ordinary files regardless of MIME type.
  function handlePastedAttachmentPaths(paths: string[]) {
    return queueComposerPaths(paths, false)
  }

  // Files dropped or pasted inside the rich editor share the same batch policy.
  function handleDroppedFilePaths(paths: string[]) {
    return queueComposerPaths(paths, true)
  }

  // Attachment handling via the browser file input.
  function handleAttachFiles() {
    // Append to DOM before clicking (required for WebKitGTK to reliably
    // open the file chooser dialog on the first click)
    const input = document.createElement('input')
    input.type = 'file'
    input.multiple = true
    input.style.display = 'none'
    document.body.appendChild(input)
    input.onchange = async (e) => {
      input.remove()
      const fileList = (e.target as HTMLInputElement).files
      if (!fileList || fileList.length === 0) return

      try {
        const newAttachments: typeof attachments = []
        for (const file of Array.from(fileList)) {
          const attachment = await selectedFileToComposerAttachment(file)
          if (attachment) newAttachments.push(attachment)
        }
        if (newAttachments.length > 0) {
          attachments = [...attachments, ...newAttachments]
          scheduleDraftSave()
        }
      } catch (err) {
        console.error('Failed to attach files:', err)
        addToast({
          type: 'error',
          message: $_('composer.failedToAttachFiles'),
        })
      }
    }
    input.click()
  }

  function removeAttachment(index: number) {
    attachments = attachments.filter((_, i) => i !== index)
    scheduleDraftSave()
  }

  function handleDragOver(e: DragEvent) {
    if (isRecipientChipDrag(e) || !hasFileDropPayload(e)) {
      isDraggingOver = false
      return
    }
    e.preventDefault()
    e.stopPropagation()
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy'
    isDraggingOver = true
  }

  function handleDragLeave(e: DragEvent) {
    if (isRecipientChipDrag(e)) return
    e.preventDefault()
    e.stopPropagation()
    isDraggingOver = false
  }

  async function handleDrop(e: DragEvent) {
    if (isRecipientChipDrag(e)) return
    e.stopPropagation()
    isDraggingOver = false

    if (!hasFileDropPayload(e)) return

    // Already handled by TipTap editor's handleDrop
    if (e.defaultPrevented) return
    e.preventDefault()

    // Case 1: File objects from browser-internal drag operations
    const files = e.dataTransfer?.files
    if (files && files.length > 0) {
      const newAttachments: ComposerAttachment[] = []
      for (const file of Array.from(files)) {
        try {
          newAttachments.push(await fileToComposerAttachment(file))
        } catch (err) {
          console.error('Failed to read dropped file:', err)
        }
      }
      if (newAttachments.length > 0) {
        attachments = [...attachments, ...newAttachments]
        scheduleDraftSave()
      }
      return
    }

    // Case 2: File URIs (drops outside editor — all as attachments)
    const uriList = e.dataTransfer?.getData('text/uri-list')
    const textData = e.dataTransfer?.getData('text/plain')
    const pathData = uriList || textData
    if (pathData) {
      const paths = parseFileUris(pathData)
      if (paths.length > 0) {
        let directReadFailed = false
        for (const filePath of paths) {
          try {
            const att = await api.readFileAsAttachment(filePath)
            if (!att) continue
            attachments = [...attachments, backendAttachmentToComposerAttachment(att)]
          } catch {
            directReadFailed = true
            break
          }
        }
        if (directReadFailed) {
          return
        }
        scheduleDraftSave()
      }
    }
  }

  function handleWindowDragEnd() {
    isDraggingOver = false
  }

</script>

<svelte:window on:keydown={handleKeyDown} on:dragend={handleWindowDragEnd} on:drop={handleWindowDragEnd} />

<div
  bind:this={composerRootElement}
  class="flex flex-col h-full bg-background relative"
  class:ring-2={isDraggingOver}
  class:ring-primary={isDraggingOver}
  class:ring-inset={isDraggingOver}
  ondragover={handleDragOver}
  ondragleave={handleDragLeave}
  ondrop={handleDrop}
  role="region"
  aria-label={$_('aria.emailComposer')}
>
  <!-- Header -->
  <div class="flex items-center justify-between px-4 py-3 border-b border-border">
    <div class="flex items-center gap-3">
      <h2 class="text-lg font-semibold">
        {#if getDisplayMode() === 'new'}
          {$_('composer.newMessage')}
        {:else if getDisplayMode() === 'reply'}
          {$_('composer.reply')}
        {:else if getDisplayMode() === 'reply-all'}
          {$_('composer.replyAll')}
        {:else if getDisplayMode() === 'forward'}
          {$_('composer.forward')}
        {/if}
      </h2>
      <!-- Draft status indicator -->
      {#if draftStatusLabel}
        <span class="text-xs text-muted-foreground flex items-center gap-1">
          <Icon icon={draftStatusIcon} class="w-3 h-3 {draftStatusColor} {saveStatus === 'saving' ? 'animate-spin' : ''}" />
          {draftStatusLabel}
        </span>
      {/if}
    </div>
    <div class="flex items-center gap-2">
      <button
        onclick={handleClose}
        class="px-3 py-1.5 text-sm text-muted-foreground hover:text-foreground hover:bg-muted rounded-md transition-colors disabled:opacity-50"
      >
        {$_('composer.close')}
      </button>
      <button
        onclick={handleSend}
        disabled={sending || toRecipients.length === 0}
        class="px-4 py-1.5 text-sm font-medium text-primary-foreground bg-primary hover:bg-primary/90 rounded-md transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
      >
        {#if sending}
          <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
          {$_('composer.sending')}
        {:else}
          <Icon icon="mdi:send" class="w-4 h-4" />
          {$_('composer.send')}
        {/if}
      </button>
    </div>
  </div>

  <!-- Compose form -->
  <div class="flex-1 flex flex-col min-h-0 overflow-hidden">
    <!-- From -->
    <div class="flex items-center gap-1 px-4 min-h-[44px] border-b border-border">
      <span class="text-sm text-muted-foreground w-14 flex-shrink-0">{widenLabel($_('composer.from'))}:</span>
      <div bind:this={fromFieldElement} class="flex-1">
        <Select.Root value={selectedIdentityId} onValueChange={handleIdentityChange}>
          <Select.Trigger class="h-6 px-0 border-0 bg-transparent shadow-none focus:ring-0">
            <Select.Value placeholder={$_('composer.selectIdentity')}>
              {#if selectedIdentityId}
                {@const identity = identities.find(i => i.id === selectedIdentityId)}
                {#if identity}
                  {formatIdentityLabel(identity)}
                {/if}
              {/if}
            </Select.Value>
          </Select.Trigger>
          <Select.Content>
            {#each identities as identity (identity.id)}
              <Select.Item value={identity.id} label={formatIdentityLabel(identity)} />
            {/each}
          </Select.Content>
        </Select.Root>
      </div>
    </div>

    <!-- To -->
    <div class="flex items-center gap-1 px-4 min-h-[44px] border-b border-border">
      <span class="text-sm text-muted-foreground w-14 flex-shrink-0">{widenLabel($_('composer.to'))}:</span>
      <div bind:this={toFieldElement} class="flex-1">
        <RecipientInput
          bind:this={toInputRef}
          bind:recipients={toRecipients}
          placeholder={$_('composer.addRecipients')}
        />
      </div>
      {#if !showCc || !showBcc}
        <div class="flex items-center gap-1 text-sm text-muted-foreground flex-shrink-0">
          {#if !showCc}
            <button onclick={() => showCc = true} class="hover:text-foreground">{$_('composer.cc')}</button>
          {/if}
          {#if !showBcc}
            <button onclick={() => showBcc = true} class="hover:text-foreground">{$_('composer.bcc')}</button>
          {/if}
        </div>
      {/if}
    </div>

    <!-- Cc -->
    {#if showCc}
      <div class="flex items-center gap-1 px-4 min-h-[44px] border-b border-border">
        <span class="text-sm text-muted-foreground w-14 flex-shrink-0">{widenLabel($_('composer.cc'))}:</span>
        <div bind:this={ccFieldElement} class="flex-1">
          <RecipientInput
            bind:this={ccInputRef}
            bind:recipients={ccRecipients}
            placeholder={$_('composer.addCcRecipients')}
          />
        </div>
      </div>
    {/if}

    <!-- Bcc -->
    {#if showBcc}
      <div class="flex items-center gap-1 px-4 min-h-[44px] border-b border-border">
        <span class="text-sm text-muted-foreground w-14 flex-shrink-0">{widenLabel($_('composer.bcc'))}:</span>
        <div bind:this={bccFieldElement} class="flex-1">
          <RecipientInput
            bind:this={bccInputRef}
            bind:recipients={bccRecipients}
            placeholder={$_('composer.addBccRecipients')}
          />
        </div>
      </div>
    {/if}

    <!-- Subject -->
    <div class="flex items-center gap-1 px-4 min-h-[44px] border-b border-border">
      <label for="composer-subject" class="text-sm text-muted-foreground w-14 flex-shrink-0">{widenLabel($_('composer.subject'))}:</label>
      <input
        id="composer-subject"
        bind:this={subjectInputElement}
        bind:value={subject}
        type="text"
        placeholder={$_('composer.subject')}
        class="flex-1 bg-transparent text-sm focus:outline-none"
      />
    </div>

    <!-- Toolbar - extracted to separate component for performance -->
    <!-- Alt+T to focus toolbar, Tab skips it -->
    <EditorToolbar
      bind:this={toolbarRef}
      {editor}
      {isPlainTextMode}
      onTogglePlainText={togglePlainTextMode}
      onInsertImage={insertImage}
      showDarkFilter={composerDarkFilterAvailable()}
      darkFilterEnabled={composerDarkFilterActive()}
      onToggleDarkFilter={toggleComposerDarkFilter}
      onReturnFocus={focusComposerBody}
    />

    <!-- Remote images blocked bar -->
    {#if composerImagesBlocked}
      <div class="flex items-center gap-2 px-3 py-2 mx-2 mt-2 rounded-md bg-yellow-500/10 border border-yellow-500/30 text-sm">
        <Icon icon="mdi:image-off" class="w-4 h-4 text-yellow-600 flex-shrink-0" />
        <span class="text-yellow-700 dark:text-yellow-400">{$_('viewer.remoteImagesBlocked')}</span>
        <button
          class="ml-auto px-2 py-1 text-xs font-medium rounded bg-yellow-600 text-white hover:bg-yellow-700 transition-colors"
          onclick={loadComposerImages}
        >
          {$_('viewer.loadImages')}
        </button>
      </div>
    {/if}

    <!-- Editor -->
    <div
      bind:this={composerBodyElement}
      class="relative flex-1 bg-white dark:bg-zinc-900 {isPlainTextMode ? 'overflow-hidden' : 'overflow-auto'}"
      style={composerBodyStyle()}
    >
      <!-- Both surfaces stay mounted; we toggle visibility instead of using
           {#if}/{:else}. Unmounting the editor <div> orphaned the TipTap
           instance, so a later switch back to rich text wrote into a dead
           editor and the body vanished. -->
      {#if isPlainTextMode}
        <div
          aria-hidden="true"
          class="composer-plain-overlay pointer-events-none absolute inset-0 overflow-hidden p-3 font-mono text-sm text-foreground whitespace-pre-wrap break-words"
        >
          <div style="transform: translate({-plainTextScrollLeft}px, {-plainTextScrollTop}px);">
            {#each plainTextMentionSegments as segment, index (index)}
              {#if segment.type === 'mention'}
                <span class="contact-mention">{segment.text}</span>
              {:else}
                {segment.text}
              {/if}
            {/each}
          </div>
        </div>
      {/if}
      <textarea
        bind:this={plainTextRef}
        bind:value={plainTextContent}
        placeholder={$_('composer.writePlaceholder')}
        class="relative z-10 w-full h-full p-3 bg-transparent resize-none focus:outline-none font-mono text-sm caret-foreground selection:bg-primary/30 placeholder:text-muted-foreground {isPlainTextMode ? 'text-transparent' : 'hidden'}"
        oninput={handlePlainTextInput}
        onpaste={handlePlainTextPaste}
        onkeydown={handlePlainTextKeydown}
        onkeyup={handlePlainTextCursorChange}
        onmouseup={handlePlainTextCursorChange}
        oncompositionstart={handleComposerCompositionStart}
        oncompositionend={handleComposerCompositionEnd}
        onscroll={handlePlainTextScroll}
        onblur={handleMentionBlur}
      ></textarea>
      <div
        bind:this={editorElement}
        class="h-full {isPlainTextMode ? 'hidden' : ''} {composerDarkFilterActive() ? 'composer-dark-filter' : ''}"
        style={composerEditorStyle()}
        role="textbox"
        aria-multiline="true"
        aria-label={$_('composer.writePlaceholder')}
        tabindex="-1"
        onkeydown={handleBodyKeydown}
        onkeyup={(e) => {
          if (isMentionNavigationKey(e.key)) return
          setTimeout(updateRichMention, 0)
        }}
        onmouseup={() => setTimeout(updateRichMention, 0)}
        oncompositionstart={handleComposerCompositionStart}
        oncompositionend={handleComposerCompositionEnd}
        onblur={handleMentionBlur}
      ></div>

      {#if mentionActive && mentionSuggestions.length > 0}
        <div
          data-composer-mention-menu
          role="listbox"
          tabindex="-1"
          class="absolute z-50 w-72 overflow-hidden rounded-md border border-border bg-popover shadow-lg"
          style="left: {mentionLeft}px; top: {mentionTop}px; max-height: {MENTION_MENU_HEIGHT}px; overscroll-behavior: contain;"
          onwheel={handleMentionMenuWheel}
          ontouchmove={handleMentionMenuTouchMove}
        >
          {#each visibleMentionSuggestions as suggestion, visibleIndex (visibleIndex)}
            {@const index = mentionWindowStart + visibleIndex}
            <button
              type="button"
              role="option"
              aria-selected={index === mentionSelectedIndex}
              data-mention-index={index}
              class="flex h-[52px] w-full items-center gap-3 px-3 py-2 text-left transition-colors {mentionKeyboardMode ? '' : 'hover:bg-muted'} {index === mentionSelectedIndex ? 'bg-muted' : ''}"
              onpointermove={(e) => handleMentionPointerMove(e, index)}
              onmousedown={(e) => {
                e.preventDefault()
                selectMention(suggestion)
              }}
            >
              <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-xs font-medium text-primary">
                {(getMentionLabel(suggestion) || suggestion.email || '?')[0].toUpperCase()}
              </div>
              <div class="min-w-0 flex-1">
                <div class="truncate text-sm font-medium">
                  {getMentionLabel(suggestion)}
                </div>
                {#if suggestion.email && suggestion.email !== getMentionLabel(suggestion)}
                  <div class="truncate text-xs text-muted-foreground">
                    {suggestion.email}
                  </div>
                {/if}
              </div>
            </button>
          {/each}
        </div>
      {/if}
    </div>

    <!-- Attachments List -->
    <ComposerAttachmentList {attachments} onRemove={removeAttachment} />

    <!-- Footer -->
    <div class="flex items-center gap-2 px-4 py-2 border-t border-border text-sm text-muted-foreground">
      <button
        onclick={handleAttachFiles}
        class="flex items-center gap-1 hover:text-foreground transition-colors"
      >
        <Icon icon="mdi:attachment" class="w-4 h-4" />
        {$_('composer.attachFiles')}
      </button>
      {#if attachments.length > 0}
        <span class="text-xs">
          {$_('composer.filesAttached', { values: { count: attachments.length } })}
        </span>
      {/if}
      <div class="flex-1"></div>
      {#if showReadReceiptOption}
        <label class="flex items-center gap-1.5 text-xs cursor-pointer hover:text-foreground transition-colors">
          <input
            type="checkbox"
            bind:checked={requestReadReceipt}
            class="w-3.5 h-3.5 rounded border-border accent-primary"
          />
          {$_('composer.requestReadReceipt')}
        </label>
      {/if}
      <span class="text-xs">{$_('composer.ctrlEnterToSend')}</span>
    </div>
  </div>

  <!-- Drag overlay -->
  {#if isDraggingOver}
    <div class="absolute inset-0 bg-primary/10 flex items-center justify-center pointer-events-none z-10">
      <div class="bg-background border-2 border-dashed border-primary rounded-lg px-8 py-6 text-center">
        <Icon icon="mdi:attachment" class="w-12 h-12 text-primary mx-auto mb-2" />
        <p class="text-lg font-medium">{$_('composer.dropToAttach')}</p>
      </div>
    </div>
  {/if}
</div>

<ComposerConfirmDialogs
  bind:showEmptySubjectDialog
  bind:showMissingAttachmentDialog
  bind:showCloseConfirm
  bind:showInlineImageSizeDialog
  {inlineImageSizeDescription}
  {closeLoading}
  onConfirmEmptySubject={handleConfirmEmptySubject}
  onConfirmMissingAttachment={handleConfirmMissingAttachment}
  onKeepImagesInline={() => resolveInlineImageChoice('inline')}
  onAttachImagesInstead={() => resolveInlineImageChoice('attachment')}
  onCancelInlineImages={() => resolveInlineImageChoice('cancel')}
  onDiscardAndClose={handleDiscardAndClose}
  onSaveAndClose={handleSaveAndClose}
  onKeepEditing={handleKeepEditing}
/>

<style>
  /* Zero-margin paragraphs so Enter looks like a single line break */
  :global(.composer-editor p) {
    margin: 0;
    line-height: 1.25;
  }

  :global(.ProseMirror p.is-editor-empty:first-child::before) {
    color: #adb5bd;
    content: attr(data-placeholder);
    float: left;
    height: 0;
    pointer-events: none;
  }

  /* Table styling for composer */
  :global(.composer-editor table) {
    border-collapse: collapse;
    margin: 0;
    overflow: hidden;
    table-layout: fixed;
    width: 100%;
  }

  :global(.composer-editor td),
  :global(.composer-editor th) {
    border: 1px solid hsl(var(--border));
    box-sizing: border-box;
    min-width: 1em;
    padding: 6px 8px;
    position: relative;
    vertical-align: top;
  }

  :global(.composer-editor th) {
    background-color: hsl(var(--muted));
    font-weight: 600;
  }

  /* Only quoted/original mail is filtered. Composer-owned content above it
     keeps native theme colors so the caret, selection, signature and forward
     header remain readable in WebKit. */
  :global(.composer-dark-filter .composer-dark-filter-content) {
    background: #fff;
    color: #000;
    filter: var(--composer-dark-content-filter);
  }

  :global(.composer-dark-filter .composer-dark-filter-content img:not([data-original-src])),
  :global(.composer-dark-filter .composer-dark-filter-content video),
  :global(.composer-dark-filter .composer-dark-filter-content iframe),
  :global(.composer-dark-filter .composer-dark-filter-content [data-no-invert]) {
    filter: var(--composer-dark-media-filter);
  }
</style>
