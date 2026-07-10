<script lang="ts">
  import { onMount, onDestroy, getContext, setContext, untrack } from 'svelte'
  import Icon from '@iconify/svelte'
  import type { Editor } from '@tiptap/core'
  import { createComposerEditor } from './composerEditor'
  // @ts-ignore - Wails generated imports
  import { smtp, account, app, contact } from '../../../../wailsjs/go/models'
  // @ts-ignore - Wails runtime for events
  import { EventsOn } from '../../../../wailsjs/runtime/runtime.js'
  import { type ComposerApi, COMPOSER_API_KEY, createMainWindowApi } from '$lib/composerApi'
  import { isImageAllowedSync } from '$lib/stores/imageAllowlist.svelte'
  import { getAlwaysLoadImages } from '$lib/stores/settings.svelte'
  import RecipientInput from './RecipientInput.svelte'
  import EditorToolbar from './EditorToolbar.svelte'
  import ComposerAttachmentList from './ComposerAttachmentList.svelte'
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
    estimateBase64DecodedSize,
    fileToDataUrl,
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
  import {
    buildDraftContentHash,
    getDraftStatusMeta,
    type DraftSaveStatus,
    type DraftSyncStatus,
  } from './composerDraft'
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
    createInlineImageCID,
    MAX_INLINE_IMAGE_SIZE,
  } from './composerInlineImages'
  import * as Select from '$lib/components/ui/select'
  import * as AlertDialog from '$lib/components/ui/alert-dialog'
  import { ThreeOptionDialog } from '$lib/components/ui/confirm-dialog'
  import { addToast } from '$lib/stores/toast'
  import { getComposerFormat } from '$lib/stores/settings.svelte'
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

  // Read receipt request
  let requestReadReceipt = $state(false)
  let showReadReceiptOption = $state(false)  // Show checkbox when policy is 'ask'

  // Plain text mode toggle (default from user setting, can be toggled per-message)
  let isPlainTextMode = $state(getComposerFormat() === 'plain')
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
  let saveTimeoutId: ReturnType<typeof setTimeout> | null = null
  let lastContent = ''  // Track content changes to avoid unnecessary saves

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
  let mentionSearchTimer: ReturnType<typeof setTimeout> | null = null
  let mentionSearchSeq = 0
  let dismissedMentionKey = $state('')
  let lastMentionPointerX = -1
  let lastMentionPointerY = -1
  let composerTextComposing = $state(false)
  const visibleMentionSuggestions = $derived(mentionSuggestions.slice(mentionWindowStart, mentionWindowStart + MENTION_VISIBLE_ROWS))

  $effect(() => {
    const maxStart = Math.max(0, mentionSuggestions.length - MENTION_VISIBLE_ROWS)
    if (mentionWindowStart > maxStart) {
      mentionWindowStart = maxStart
    }
  })

  // Confirmation dialogs state
  let showEmptySubjectDialog = $state(false)
  let showMissingAttachmentDialog = $state(false)
  let showFlatpakDndDialog = $state(false)
  let showCloseConfirm = $state(false)
  let closeLoading = $state<'discard' | 'save' | null>(null)

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

  function setMentionSelectedIndex(index: number, inputMode: 'keyboard' | 'mouse' | 'program' = 'program') {
    if (mentionSuggestions.length === 0) {
      mentionSelectedIndex = -1
      mentionWindowStart = 0
      return
    }

    if (inputMode === 'keyboard') mentionKeyboardMode = true
    if (inputMode === 'mouse') mentionKeyboardMode = false
    const nextIndex = Math.min(Math.max(index, 0), mentionSuggestions.length - 1)
    let nextWindowStart = mentionWindowStart
    if (nextIndex < mentionWindowStart) {
      nextWindowStart = nextIndex
    } else if (nextIndex >= mentionWindowStart + MENTION_VISIBLE_ROWS) {
      nextWindowStart = nextIndex - MENTION_VISIBLE_ROWS + 1
    }
    mentionWindowStart = nextWindowStart
    mentionSelectedIndex = nextIndex
  }

  function handleMentionPointerMove(e: PointerEvent, index: number) {
    if (e.clientX === lastMentionPointerX && e.clientY === lastMentionPointerY) return
    lastMentionPointerX = e.clientX
    lastMentionPointerY = e.clientY
    setMentionSelectedIndex(index, 'mouse')
  }

  function closeMentionSuggestions() {
    mentionActive = false
    mentionSuggestions = []
    mentionSelectedIndex = 0
    mentionKeyboardMode = false
    mentionWindowStart = 0
    mentionQuery = ''
    if (mentionSearchTimer) {
      clearTimeout(mentionSearchTimer)
      mentionSearchTimer = null
    }
  }

  function scheduleMentionSearch(query: string) {
    if (mentionSearchTimer) {
      clearTimeout(mentionSearchTimer)
    }

    const normalizedQuery = query.trim()
    if (!normalizedQuery) {
      mentionSuggestions = []
      mentionSelectedIndex = -1
      mentionWindowStart = 0
      return
    }

    const seq = ++mentionSearchSeq
    mentionSearchTimer = setTimeout(async () => {
      try {
        const results = await api.searchContacts(normalizedQuery, MENTION_SUGGESTION_LIMIT)
        if (seq !== mentionSearchSeq || !mentionActive || mentionQuery !== query) return
        mentionSuggestions = filteredMentionResults(results, normalizedQuery)
        setMentionSelectedIndex(mentionSuggestions.length > 0 ? 0 : -1)
      } catch (err) {
        if (seq === mentionSearchSeq) {
          console.error('Failed to search contacts for mention:', err)
          mentionSuggestions = []
          mentionSelectedIndex = -1
          mentionWindowStart = 0
        }
      }
    }, 150)
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

  // Collect any images in the editor DOM that aren't tracked in inlineImages.
  // WebKitGTK doesn't expose pasted screenshots via clipboardData, so TipTap's
  // default handler inserts them with a webkit-fake-url:// src. This function
  // extracts the pixel data via canvas and registers them for CID conversion.
  function collectUnregisteredInlineImages(html: string): string {
    const editorEl = editor?.view?.dom
    if (!editorEl) return html

    const imgs = editorEl.querySelectorAll('img')
    let result = html

    for (const img of imgs) {
      const src = img.getAttribute('src') || ''

      // Skip tracked, cid:, http(s):, and blocked remote images
      if (src.startsWith('cid:') || src.startsWith('http://') || src.startsWith('https://')) continue
      if (img.hasAttribute('data-original-src')) continue
      if (inlineImages.some(i => i.dataUrl === src)) continue

      // data: URLs — parse and register directly
      if (src.startsWith('data:')) {
        const match = src.match(/^data:([^;]+);base64,(.+)$/)
        if (!match) continue
        const cid = generateCID()
        inlineImages = [...inlineImages, {
          cid,
          dataUrl: src,
          contentType: match[1],
          data: match[2],
          filename: `pasted-image${inlineImageCounter}.${match[1].split('/')[1] || 'png'}`,
        }]
        continue
      }

      // webkit-fake-url://, blob:, etc. — extract via canvas
      if (!img.complete || img.naturalWidth === 0) continue
      try {
        const canvas = document.createElement('canvas')
        canvas.width = img.naturalWidth
        canvas.height = img.naturalHeight
        const ctx = canvas.getContext('2d')
        if (!ctx) continue
        ctx.drawImage(img, 0, 0)
        const dataUrl = canvas.toDataURL('image/png')
        const base64Data = dataUrl.split(',')[1]

        // If the same canvas-extracted content was already registered
        // (e.g. user pasted the same screenshot twice), reuse that cid —
        // the viewer is what handles same-cid resolution (see
        // EmailBody.svelte querySelectorAll fix).
        const dup = inlineImages.find(i => i.dataUrl === dataUrl)
        if (dup) {
          result = result.replaceAll(src, dup.dataUrl)
          continue
        }

        const cid = generateCID()
        inlineImages = [...inlineImages, {
          cid,
          dataUrl,
          contentType: 'image/png',
          data: base64Data,
          filename: `pasted-image${inlineImageCounter}.png`,
        }]
        result = result.replaceAll(src, dataUrl)
      } catch {
        continue
      }
    }

    return result
  }

  // Convert HTML with data URLs to use CID references for inline images
  function convertDataUrlsToCid(html: string): string {
    let result = html

    // For each inline image, replace its data URL with cid: reference
    for (const img of inlineImages) {
      result = result.replaceAll(img.dataUrl, `cid:${img.cid}`)
    }

    return result
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
      // Collect any untracked pasted images (WebKitGTK webkit-fake-url, etc.),
      // add paragraph styles for email clients, then convert data URLs to CID references
      const rawHtml = collectUnregisteredInlineImages(editor?.getHTML() || '')
      htmlContent = convertDataUrlsToCid(addParagraphStyles(rawHtml))
      textContent = editor?.getText() || ''
    }

    // Restore blocked remote images for sending — replace placeholder with original URL
    htmlContent = htmlContent.replace(
      /<img([^>]*)\sdata-original-src="([^"]+)"([^>]*)>/gi,
      (match, _before, originalSrc, _after) => {
        return match
          .replace(/src="[^"]*"/, `src="${originalSrc}"`)
          .replace(/\s*data-original-src="[^"]*"/, '')
      }
    )

    // Convert ComposerAttachment to smtp.Attachment format (regular attachments)
    // Use content_base64 (string) instead of content (number[]) to avoid
    // pathologically slow JSON serialization of large byte arrays through Wails RPC.
    const smtpAttachments: smtp.Attachment[] = attachments.map(att => new smtp.Attachment({
      filename: att.filename,
      content_type: att.contentType,
      content_base64: att.data,
      content_id: '',
      inline: false,
    }))

    // Add inline images as inline attachments with Content-ID
    for (const img of inlineImages) {
      smtpAttachments.push(new smtp.Attachment({
        filename: img.filename,
        content_type: img.contentType,
        content_base64: img.data,
        content_id: img.cid,
        inline: true,
      }))
    }

    return new smtp.ComposeMessage({
      from: new smtp.Address({
        name: selectedIdentity?.name || '',
        address: selectedIdentity?.email || '',
      }),
      to: toRecipients,
      cc: ccRecipients,
      bcc: bccRecipients,
      subject: subject,
      html_body: htmlContent,
      text_body: textContent,
      attachments: smtpAttachments,
      in_reply_to: inReplyTo,
      references: references,
      source_message_id: sourceMessageId,
      reply_type: replyType,
      request_read_receipt: requestReadReceipt,
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

  // Schedule a draft save (debounced)
  // Note: All expensive operations (hasContent, getContentHash) are inside the timeout
  // to avoid lag on every keystroke
  function scheduleDraftSave() {
    // Clear any pending save
    if (saveTimeoutId) {
      clearTimeout(saveTimeoutId)
    }

    // Reset indicator immediately when content changes (makes it disappear on input)
    if (saveStatus === 'saved') {
      saveStatus = 'idle'
    }

    saveTimeoutId = setTimeout(async () => {
      // Only save if there's content
      if (!hasContent()) {
        return
      }

      // Check if content actually changed
      const currentHash = getContentHash()
      if (currentHash === lastContent) {
        return
      }

      await saveDraft()
    }, DRAFT_SAVE_DELAY)
  }

  // Guard to prevent concurrent save requests (which cause orphaned drafts)
  let isSaving = false
  let discarding = false
  let savingComplete: Promise<void> = Promise.resolve()

  // Actually save the draft
  async function saveDraft() {
    if (discarding) return
    if (!hasContent()) return

    // If a save is already in flight, skip — next edit will trigger a fresh save
    if (isSaving) return

    // Check again for content changes before saving
    const currentHash = getContentHash()
    if (currentHash === lastContent && currentDraftId) {
      return  // No changes since last save
    }

    let resolveSaving: () => void
    savingComplete = new Promise<void>(resolve => { resolveSaving = resolve })

    isSaving = true
    saveStatus = 'saving'
    try {
      const message = buildMessage()
      const result = await api.saveDraft(activeAccountId, message, currentDraftId || '')
      currentDraftId = result.id
      lastContent = currentHash
      saveStatus = 'saved'
      syncStatus = result.syncStatus as DraftSyncStatus
      lastSavedAt = new Date()
    } catch (err) {
      console.error('Failed to save draft:', err)
      saveStatus = 'error'
    } finally {
      isSaving = false
      resolveSaving!()
    }
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
      lastContent = getContentHash()
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
        lastContent = ''
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
    // Clear any pending save timeout
    if (saveTimeoutId) {
      clearTimeout(saveTimeoutId)
    }
    editor?.destroy()
  })

  // Helper to ensure proper smtp.Address object (handles both 'address' and 'email' field names)
  function toSmtpAddress(addr: any): smtp.Address {
    if (!addr) return new smtp.Address({ name: '', address: '' })
    return new smtp.Address({
      name: addr.name || '',
      address: addr.address || addr.email || ''
    })
  }

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

  // Pre-send validation - returns true if we should proceed, false if waiting for confirmation
  function validateBeforeSend(): boolean {
    // Check for missing attachment
    if (attachments.length === 0 && bodyMentionsAttachment()) {
      showMissingAttachmentDialog = true
      return false
    }

    // Check for empty subject
    if (!subject.trim()) {
      showEmptySubjectDialog = true
      return false
    }

    return true
  }

  async function handleSend() {
    if (toRecipients.length === 0) {
      addToast({
        type: 'error',
        message: $_('composer.noRecipients'),
      })
      return
    }

    const selectedIdentity = identities.find(i => i.id === selectedIdentityId)
    if (!selectedIdentity) {
      addToast({
        type: 'error',
        message: $_('composer.selectSenderIdentity'),
      })
      return
    }

    // Run validations that may show confirmation dialogs
    if (!validateBeforeSend()) {
      return
    }

    await doSend()
  }

  // Actually send the message (called directly or after confirmation)
  async function doSend() {
    // Cancel any pending draft save
    if (saveTimeoutId) {
      clearTimeout(saveTimeoutId)
      saveTimeoutId = null
    }

    // Wait for any in-flight draft save to complete before sending
    await savingComplete

    sending = true

    try {
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
    // Cancel any pending draft save
    if (saveTimeoutId) {
      clearTimeout(saveTimeoutId)
      saveTimeoutId = null
    }

    // Always show confirmation dialog (even for empty content, since a draft may have been saved)
    showCloseConfirm = true
  }

  // Discard: Delete draft from local DB and IMAP, then close
  async function handleDiscardAndClose() {
    discarding = true
    if (saveTimeoutId) {
      clearTimeout(saveTimeoutId)
      saveTimeoutId = null
    }
    await savingComplete
    closeLoading = 'discard'
    try {
      if (currentDraftId) {
        await api.deleteDraft(currentDraftId)
        currentDraftId = null
      }
    } catch (err) {
      console.error('Failed to delete draft:', err)
      // Still close even if delete fails
    }
    showCloseConfirm = false
    closeLoading = null
    onClose?.()
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
        await handleInlineImageFile(file)
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
        showFlatpakDndDialog ||
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
      onPasteImage: handleInlineImageFile,
      onDropImage: handleInlineImageFile,
      onDropFile: handleDroppedFile,
      onDropFilePaths: handleDroppedFilePaths,
      onShiftTab: () => document.getElementById('composer-subject')?.focus(),
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
  function handleKeyDown(e: KeyboardEvent) {
    if (e.defaultPrevented) return

    if (e.key === 'Tab') {
      handleComposerTabKeydown(e)
      return
    }
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault()
      handleSend()
    }
    // Alt+T to focus toolbar (hint mode)
    if (e.key === 't' && e.altKey) {
      e.preventDefault()
      toolbarRef?.focus()
    }
    // Alt+A to attach files
    if (e.key === 'a' && e.altKey) {
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

  // Handle an inline image file (from paste or drop)
  async function handleInlineImageFile(file: File) {
    if (file.size > MAX_INLINE_IMAGE_SIZE) {
      addToast({
        type: 'error',
        message: $_('composer.imageTooLarge'),
      })
      return
    }

    try {
      const dataUrl = await fileToDataUrl(file)

      // Dedup by content (dataUrl): same image pasted twice produces a
      // single inlineImage entry, so the sent MIME has one inline
      // attachment instead of leaving the second cid orphaned. The editor
      // still gets a second <img> with the same dataUrl src — the viewer
      // side is what handles same-cid resolution (see EmailBody.svelte).
      const existing = inlineImages.find(i => i.dataUrl === dataUrl)
      if (existing) {
        editor?.chain().focus().setImage({ src: existing.dataUrl, alt: existing.filename }).run()
        scheduleDraftSave()
        return
      }

      const cid = generateCID()

      // Extract base64 data and content type from data URL
      const matches = dataUrl.match(/^data:([^;]+);base64,(.+)$/)
      if (!matches) {
        console.error('Invalid data URL format')
        return
      }

      const contentType = matches[1]
      const base64Data = matches[2]

      // Store the inline image
      const inlineImage: InlineImage = {
        cid,
        dataUrl,
        contentType,
        data: base64Data,
        filename: file.name || `image${inlineImageCounter}.${contentType.split('/')[1] || 'png'}`,
      }
      inlineImages = [...inlineImages, inlineImage]

      // Insert the image into the editor with the data URL (for display)
      // When sending, we'll convert data URLs to cid: references
      editor?.chain().focus().setImage({ src: dataUrl, alt: inlineImage.filename }).run()

      scheduleDraftSave()
    } catch (err) {
      console.error('Failed to process inline image:', err)
      addToast({
        type: 'error',
        message: $_('composer.failedToInsertImage'),
      })
    }
  }

  // Handle a non-image File dropped on the editor (add as attachment)
  async function handleDroppedFile(file: File) {
    try {
      attachments = [...attachments, await fileToComposerAttachment(file)]
      scheduleDraftSave()
    } catch (err) {
      console.error('Failed to read dropped file:', err)
    }
  }

  // Handle file paths dropped on the editor (from text/uri-list parsing)
  // Images are inserted inline, other files are added as attachments
  async function handleDroppedFilePaths(paths: string[]) {
    for (const filePath of paths) {
      try {
        const att = await api.readFileAsAttachment(filePath)
        if (!att) continue

        if (att.contentType.startsWith('image/')) {
          // Check size before inserting inline
          const imageBytes = estimateBase64DecodedSize(att.data)
          if (imageBytes > MAX_INLINE_IMAGE_SIZE) {
            addToast({
              type: 'error',
              message: $_('composer.imageTooLarge'),
            })
            continue
          }
          // Insert as inline image
          const dataUrl = `data:${att.contentType};base64,${att.data}`

          // Dedup by content; mirrors handleInlineImageFile. Same image
          // dropped twice ⇒ one inline attachment (no orphan cid in MIME).
          const existing = inlineImages.find(i => i.dataUrl === dataUrl)
          if (existing) {
            editor?.chain().focus().setImage({ src: existing.dataUrl, alt: existing.filename }).run()
            continue
          }

          const cid = generateCID()
          inlineImages = [...inlineImages, {
            cid,
            dataUrl,
            contentType: att.contentType,
            data: att.data,
            filename: att.filename,
          }]
          editor?.chain().focus().setImage({ src: dataUrl, alt: att.filename }).run()
          continue
        }
        // Add as regular attachment
        attachments = [...attachments, backendAttachmentToComposerAttachment(att)]
      } catch {
        // Direct read failed — if Flatpak, show permission info dialog
        if (await api.isFlatpak()) {
          showFlatpakDndDialog = true
        }
        return
      }
    }
    scheduleDraftSave()
  }

  // Attachment handling — uses HTML file input so WebKitGTK routes through
  // the FileChooser portal (required for Flatpak sandbox file access)
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
          // If Flatpak, show permission info dialog
          if (await api.isFlatpak()) {
            showFlatpakDndDialog = true
          }
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
        class="h-full {isPlainTextMode ? 'hidden' : ''}"
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

<!-- Empty Subject Confirmation Dialog -->
<AlertDialog.Root bind:open={showEmptySubjectDialog}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{$_('composer.emptySubjectTitle')}</AlertDialog.Title>
      <AlertDialog.Description>
        {$_('composer.emptySubjectDescription')}
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>{$_('common.cancel')}</AlertDialog.Cancel>
      <AlertDialog.Action onclick={handleConfirmEmptySubject}>{$_('composer.sendAnywayGeneric')}</AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>

<!-- Missing Attachment Confirmation Dialog -->
<AlertDialog.Root bind:open={showMissingAttachmentDialog}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{$_('composer.missingAttachmentTitle')}</AlertDialog.Title>
      <AlertDialog.Description>
        {$_('composer.missingAttachmentDescription')}
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>{$_('common.cancel')}</AlertDialog.Cancel>
      <AlertDialog.Action onclick={handleConfirmMissingAttachment}>{$_('composer.sendAnywayGeneric')}</AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>

<!-- Flatpak Drag-and-Drop Info Dialog -->
<AlertDialog.Root bind:open={showFlatpakDndDialog}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{$_('composer.flatpakDndTitle')}</AlertDialog.Title>
      <AlertDialog.Description>
        <p class="mb-3">{$_('composer.flatpakDndDescription')}</p>
        <p class="mb-2">{$_('composer.flatpakDndGrantExample')}</p>
        <code class="block bg-muted px-3 py-2 rounded text-sm font-mono mb-3 select-all overflow-x-auto">flatpak override --user --filesystem=home com.aulyc.aulycmail</code>
        <p class="mb-3 text-sm text-destructive">{$_('composer.flatpakDndSecurityWarning')}</p>
        <p class="text-sm text-muted-foreground">{$_('composer.flatpakDndAlternative')}</p>
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Action onclick={() => showFlatpakDndDialog = false}>{$_('common.ok')}</AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>

<!-- Close Confirmation Dialog -->
<ThreeOptionDialog
  bind:open={showCloseConfirm}
  title={$_('composer.closeTitle')}
  description={$_('composer.closeDescription')}
  option1Label={$_('composer.discardDraft')}
  option2Label={$_('composer.saveAndClose')}
  option3Label={$_('composer.keepEditing')}
  option1Variant="destructive"
  option2Variant="default"
  loading={closeLoading === 'discard' ? 'option1' : closeLoading === 'save' ? 'option2' : null}
  onOption1={handleDiscardAndClose}
  onOption2={handleSaveAndClose}
  onOption3={handleKeepEditing}
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
</style>
