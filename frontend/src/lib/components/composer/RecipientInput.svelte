<script module lang="ts">
  // Module-scope counter — gives each RecipientInput instance a unique ID so
  // drag-and-drop can tell intra-field reorder from cross-field move without
  // any shared store.
  let nextInstanceId = 0
</script>

<script lang="ts">
  import { getContext, tick } from 'svelte'
  import Icon from '@iconify/svelte'
  // @ts-ignore - Wails generated imports
  import { smtp, contact } from '../../../../wailsjs/go/models'
  import { type ComposerApi, COMPOSER_API_KEY, createMainWindowApi } from '$lib/composerApi'

  interface Props {
    recipients: smtp.Address[]
    placeholder?: string
    /** Optional: search contacts function override */
    searchContactsFn?: (query: string, limit: number) => Promise<contact.Contact[]>
  }

  let { recipients = $bindable([]), placeholder = 'Add recipients...', searchContactsFn }: Props = $props()

  nextInstanceId += 1
  const instanceId = nextInstanceId

  // Get API from context or create default
  const contextApi = getContext<ComposerApi | undefined>(COMPOSER_API_KEY)
  const api: ComposerApi = contextApi || createMainWindowApi()
  const SUGGESTION_LIMIT = 100
  const SUGGESTION_ROW_HEIGHT = 52
  const SUGGESTION_VISIBLE_ROWS = 4
  const SUGGESTION_MENU_BORDER_WIDTH = 2
  const SUGGESTION_MENU_HEIGHT = SUGGESTION_ROW_HEIGHT * SUGGESTION_VISIBLE_ROWS + SUGGESTION_MENU_BORDER_WIDTH

  // Use the prop function or fall back to API (evaluated each call to handle prop changes)
  function doSearchContacts(query: string, limit: number) {
    return searchContactsFn ? searchContactsFn(query, limit) : api.searchContacts(query, limit)
  }

  // State
  let inputValue = $state('')
  let suggestions = $state<contact.Contact[]>([])
  let showSuggestions = $state(false)
  let selectedIndex = $state(-1)
  let suggestionKeyboardMode = $state(false)
  let suggestionWindowStart = $state(0)
  let suggestionsLeft = $state(0)
  let inputIndex = $state(recipients.length)
  let inputElement = $state<HTMLInputElement | null>(null)
  let containerElement = $state<HTMLDivElement | null>(null)
  let debounceTimer: ReturnType<typeof setTimeout> | null = null
  let lastPointerX = -1
  let lastPointerY = -1
  let previousRecipientCount = recipients.length
  let inputComposing = $state(false)
  const visibleSuggestions = $derived(suggestions.slice(suggestionWindowStart, suggestionWindowStart + SUGGESTION_VISIBLE_ROWS))

  $effect(() => {
    const count = recipients.length
    if (inputIndex > recipients.length) {
      inputIndex = recipients.length
    } else if (inputIndex === previousRecipientCount && count > previousRecipientCount) {
      inputIndex = count
    }
    previousRecipientCount = count
  })

  $effect(() => {
    const maxStart = Math.max(0, suggestions.length - SUGGESTION_VISIBLE_ROWS)
    if (suggestionWindowStart > maxStart) {
      suggestionWindowStart = maxStart
    }
  })

  function updateSuggestionsPosition() {
    if (!inputElement || !containerElement) return
    suggestionsLeft = inputElement.offsetLeft
  }

  // Search contacts as user types
  async function searchContacts(query: string) {
    if (query.length < 1) {
      suggestions = []
      showSuggestions = false
      return
    }

    try {
      const results = await doSearchContacts(query, SUGGESTION_LIMIT)
      suggestions = results || []
      showSuggestions = suggestions.length > 0
      selectedIndex = -1
      suggestionKeyboardMode = false
      suggestionWindowStart = 0
      updateSuggestionsPosition()
    } catch (err) {
      console.error('Failed to search contacts:', err)
      suggestions = []
      suggestionWindowStart = 0
    }
  }

  function handleInput() {
    if (inputComposing) return
    updateSuggestionsPosition()
    // Debounce the search
    if (debounceTimer) {
      clearTimeout(debounceTimer)
    }
    debounceTimer = setTimeout(() => {
      searchContacts(inputValue.trim())
    }, 200)
  }

  function focusInputAt(index: number) {
    inputIndex = Math.min(Math.max(index, 0), recipients.length)
    void tick().then(() => {
      inputElement?.focus()
      inputElement?.setSelectionRange(inputValue.length, inputValue.length)
      updateSuggestionsPosition()
    })
  }

  function setSelectedIndex(index: number, inputMode: 'keyboard' | 'mouse' | 'program' = 'program') {
    if (suggestions.length === 0) {
      selectedIndex = -1
      suggestionWindowStart = 0
      return
    }
    if (inputMode === 'keyboard') suggestionKeyboardMode = true
    if (inputMode === 'mouse') suggestionKeyboardMode = false
    const nextIndex = Math.min(Math.max(index, 0), suggestions.length - 1)
    let nextWindowStart = suggestionWindowStart
    if (nextIndex < suggestionWindowStart) {
      nextWindowStart = nextIndex
    } else if (nextIndex >= suggestionWindowStart + SUGGESTION_VISIBLE_ROWS) {
      nextWindowStart = nextIndex - SUGGESTION_VISIBLE_ROWS + 1
    }
    suggestionWindowStart = nextWindowStart
    selectedIndex = nextIndex
  }

  function handleSuggestionPointerMove(e: PointerEvent, index: number) {
    if (e.clientX === lastPointerX && e.clientY === lastPointerY) return
    lastPointerX = e.clientX
    lastPointerY = e.clientY
    setSelectedIndex(index, 'mouse')
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.isComposing || inputComposing) return

    if (e.key === 'ArrowDown') {
      e.preventDefault()
      if (showSuggestions && suggestions.length > 0) {
        setSelectedIndex(selectedIndex + 1, 'keyboard')
      }
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      if (showSuggestions && suggestions.length > 0) {
        setSelectedIndex(selectedIndex <= 0 ? 0 : selectedIndex - 1, 'keyboard')
      }
    } else if (
      e.key === 'ArrowLeft' &&
      inputValue === '' &&
      (inputElement?.selectionStart ?? 0) === 0 &&
      inputIndex > 0
    ) {
      e.preventDefault()
      focusInputAt(inputIndex - 1)
    } else if (
      e.key === 'ArrowRight' &&
      inputValue === '' &&
      (inputElement?.selectionStart ?? 0) === 0 &&
      inputIndex < recipients.length
    ) {
      e.preventDefault()
      focusInputAt(inputIndex + 1)
    } else if (e.key === 'Home' && inputValue === '' && inputIndex > 0) {
      e.preventDefault()
      focusInputAt(0)
    } else if (e.key === 'End' && inputValue === '' && inputIndex < recipients.length) {
      e.preventDefault()
      focusInputAt(recipients.length)
    } else if (e.key === 'Backspace' && inputValue === '' && inputIndex > 0) {
      // Remove the recipient immediately before the caret.
      e.preventDefault()
      removeRecipient(inputIndex - 1, inputIndex - 1)
    } else if (e.key === 'Delete' && inputValue === '' && inputIndex < recipients.length) {
      // Remove the recipient immediately after the caret.
      e.preventDefault()
      removeRecipient(inputIndex, inputIndex)
    } else if (e.key === 'Delete' && inputValue === '' && inputIndex === recipients.length && recipients.length > 0) {
      e.preventDefault()
      removeRecipient(recipients.length - 1, recipients.length - 1)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (showSuggestions && selectedIndex >= 0) {
        selectSuggestion(suggestions[selectedIndex])
      } else if (inputValue.trim()) {
        addRecipient(inputValue.trim())
      }
    } else if (e.key === 'Escape') {
      showSuggestions = false
      selectedIndex = -1
    } else if (e.key === ',' || e.key === ';' || e.key === 'Tab') {
      if (inputValue.trim()) {
        e.preventDefault()
        addRecipient(inputValue.trim())
      }
    }
  }

  function selectSuggestion(contact: contact.Contact) {
    const address = new smtp.Address({
      name: contact.display_name || '',
      address: contact.email,
    })
    insertRecipient(address)
    inputValue = ''
    suggestions = []
    showSuggestions = false
    selectedIndex = -1
    suggestionKeyboardMode = false
    suggestionWindowStart = 0
    focusInputAt(inputIndex)
  }

  function insertRecipient(address: smtp.Address) {
    const email = (address.address || (address as any).email || '').trim().toLowerCase()
    if (email && recipients.some(r => (r.address || (r as any).email || '').toLowerCase() === email)) {
      return
    }
    const next = [...recipients]
    const insertAt = Math.min(Math.max(inputIndex, 0), next.length)
    next.splice(insertAt, 0, address)
    recipients = next
    inputIndex = insertAt + 1
  }

  function addRecipient(value: string) {
    // Parse email address (handle "Name <email@example.com>" format)
    const emailRegex = /^(?:(.+?)\s*<)?([^\s<>]+@[^\s<>]+)>?$/
    const match = value.match(emailRegex)

    if (match) {
      const name = match[1]?.trim() || ''
      const email = match[2].toLowerCase()

      // Check if already added (handle both 'address' and 'email' field names)
      if (recipients.some(r => (r.address || (r as any).email || '').toLowerCase() === email)) {
        inputValue = ''
        return
      }

      const address = new smtp.Address({
        name: name,
        address: email,
      })
      insertRecipient(address)
      inputValue = ''
      suggestions = []
      showSuggestions = false
      suggestionWindowStart = 0
      focusInputAt(inputIndex)
    }
  }

  function removeRecipient(index: number, nextInputIndex = Math.min(index, recipients.length - 1)) {
    recipients = recipients.filter((_, i) => i !== index)
    inputIndex = Math.min(Math.max(nextInputIndex, 0), recipients.length)
    focusInputAt(inputIndex)
  }

  // ─── Drag-and-drop: reorder within field, move between To/Cc/Bcc fields ───

  const DND_MIME = 'application/x-aulycmail-recipient'

  let draggingIndex = $state<number | null>(null)
  let dropTargetIndex = $state<number | null>(null)

  function handleChipDragStart(e: DragEvent, index: number) {
    if (!e.dataTransfer) return
    e.dataTransfer.setData(DND_MIME, JSON.stringify({
      sourceId: instanceId,
      recipient: recipients[index],
    }))
    e.dataTransfer.effectAllowed = 'move'
    draggingIndex = index
  }

  function handleChipDragEnd(e: DragEvent) {
    // Source removes its chip only if a cross-field move actually happened.
    // Intra-field reorder clears draggingIndex inside handleDrop so this skips.
    // dropEffect is 'none' for cancelled drops.
    if (e.dataTransfer?.dropEffect === 'move' && draggingIndex !== null) {
      removeRecipient(draggingIndex)
    }
    draggingIndex = null
    dropTargetIndex = null
  }

  function hasDndPayload(e: DragEvent): boolean {
    return !!e.dataTransfer?.types.includes(DND_MIME)
  }

  function handleDragEnter(e: DragEvent, targetIndex: number) {
    if (!hasDndPayload(e)) return
    e.preventDefault()
    dropTargetIndex = targetIndex
  }

  function handleDragOver(e: DragEvent, targetIndex: number) {
    if (!hasDndPayload(e)) return
    e.preventDefault()  // required to allow drop
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
    dropTargetIndex = targetIndex
  }

  function handleDragLeave() {
    dropTargetIndex = null
  }

  function handleDrop(e: DragEvent, targetIndex: number) {
    const raw = e.dataTransfer?.getData(DND_MIME)
    if (!raw) {
      dropTargetIndex = null
      return
    }

    let payload: { sourceId: number; recipient: smtp.Address }
    try {
      payload = JSON.parse(raw)
    } catch {
      dropTargetIndex = null
      return
    }

    if (payload.sourceId === instanceId) {
      // Intra-field reorder: splice from source index to target index.
      e.preventDefault()
      const from = draggingIndex
      if (from === null || from === targetIndex || from + 1 === targetIndex) {
        // No move (dropping on self or immediately after self)
        draggingIndex = null
        dropTargetIndex = null
        return
      }
      const next = [...recipients]
      const [moved] = next.splice(from, 1)
      const adjusted = from < targetIndex ? targetIndex - 1 : targetIndex
      next.splice(adjusted, 0, moved)
      recipients = next
      // Clear draggingIndex so handleChipDragEnd skips removal (move already done).
      draggingIndex = null
      dropTargetIndex = null
      return
    }

    // Cross-field move: append to destination via the existing addRecipient
    // pipeline (parse, dedup, clear). Source's handleChipDragEnd will remove
    // from source on dragend.
    e.preventDefault()
    const r = payload.recipient
    const name = r?.name?.trim()
    const address = (r?.address || '').trim()
    if (!address) {
      dropTargetIndex = null
      return
    }
    addRecipient(name ? `${name} <${address}>` : address)
    dropTargetIndex = null
  }

  function handleBlur() {
    // Delay hiding to allow click on suggestion (mousedown-based selection
    // runs before blur and clears inputValue, so the auto-commit below becomes
    // a no-op for that path).
    setTimeout(() => {
      showSuggestions = false
      suggestionWindowStart = 0
      // Auto-commit typed text on blur so the user doesn't have to press
      // Tab/Enter to turn a typed address into a chip. Invalid input is left
      // in the field for the user to fix (addRecipient is a no-op on regex
      // miss).
      if (inputValue.trim()) {
        addRecipient(inputValue.trim())
      } else if (inputValue) {
        inputValue = ''
      }
    }, 200)
  }

  // Allow parent to focus the input programmatically
  export function focus() {
    focusInputAt(recipients.length)
  }

  function handleFocus() {
    updateSuggestionsPosition()
    if (inputValue.trim().length >= 1 && suggestions.length > 0) {
      showSuggestions = true
    }
  }

  function handlePaste(e: ClipboardEvent) {
    const text = e.clipboardData?.getData('text')
    if (text) {
      // Handle pasted email addresses (comma or semicolon separated)
      const addresses = text.split(/[,;]/).map(a => a.trim()).filter(Boolean)
      if (addresses.length > 1) {
        e.preventDefault()
        addresses.forEach(addRecipient)
      }
    }
  }

  function handleSlotMouseDown(e: MouseEvent, index: number) {
    e.preventDefault()
    focusInputAt(index)
  }

  function handleCompositionStart() {
    inputComposing = true
    showSuggestions = false
    selectedIndex = -1
    suggestionWindowStart = 0
  }

  function handleCompositionEnd() {
    inputComposing = false
    void tick().then(() => {
      handleInput()
    })
  }
</script>

<div bind:this={containerElement} class="relative">
  <div class="flex min-h-6 w-full flex-wrap items-center gap-x-0.5 gap-y-1 py-1" role="list">
    {#each Array(recipients.length + 1) as _, slotIndex (slotIndex)}
      {#if dropTargetIndex === slotIndex}
        <span aria-hidden="true" class="h-6 w-0.5 shrink-0 rounded-full bg-primary"></span>
      {/if}

      {#if slotIndex === inputIndex}
        <!-- Input — also a drop target for inserting at the caret position -->
        <input
          bind:this={inputElement}
          bind:value={inputValue}
          oninput={handleInput}
          onkeydown={handleKeyDown}
          onblur={handleBlur}
          onfocus={handleFocus}
          onpaste={handlePaste}
          oncompositionstart={handleCompositionStart}
          oncompositionend={handleCompositionEnd}
          ondragenter={(e) => handleDragEnter(e, inputIndex)}
          ondragover={(e) => handleDragOver(e, inputIndex)}
          ondragleave={handleDragLeave}
          ondrop={(e) => handleDrop(e, inputIndex)}
          type="text"
          inputmode="email"
          autocomplete="email"
          spellcheck="false"
          placeholder={recipients.length === 0 || inputValue ? placeholder : ''}
          class="{inputIndex === recipients.length || inputValue ? 'flex-1 min-w-[150px]' : 'w-[6px] min-w-[6px] shrink-0'} h-6 bg-transparent p-0 text-sm leading-6 focus:outline-none"
        />
      {:else if slotIndex !== 0}
        <button
          type="button"
          aria-label="Set recipient cursor"
          tabindex="-1"
          onmousedown={(e) => handleSlotMouseDown(e, slotIndex)}
          ondragenter={(e) => handleDragEnter(e, slotIndex)}
          ondragover={(e) => handleDragOver(e, slotIndex)}
          ondragleave={handleDragLeave}
          ondrop={(e) => handleDrop(e, slotIndex)}
          class="h-6 w-[6px] shrink-0 cursor-text bg-transparent p-0"
        ></button>
      {/if}

      {#if slotIndex < recipients.length}
        {@const recipient = recipients[slotIndex]}
        <div
          role="listitem"
          draggable="true"
          ondragstart={(e) => handleChipDragStart(e, slotIndex)}
          ondragend={handleChipDragEnd}
          ondragenter={(e) => handleDragEnter(e, slotIndex)}
          ondragover={(e) => handleDragOver(e, slotIndex)}
          ondragleave={handleDragLeave}
          ondrop={(e) => handleDrop(e, slotIndex)}
          class="flex h-6 items-center gap-1 px-2 py-0.5 bg-muted rounded-md text-sm transition-opacity cursor-grab {draggingIndex === slotIndex ? 'opacity-50' : ''}"
        >
          <span>
            {#if recipient.name}
              {recipient.name}
            {:else}
              {recipient.address || (recipient as any).email || ''}
            {/if}
          </span>
          <button
            onclick={() => removeRecipient(slotIndex)}
            class="text-muted-foreground hover:text-foreground"
            type="button"
          >
            <Icon icon="mdi:close" class="w-3.5 h-3.5" />
          </button>
        </div>
      {/if}
    {/each}

    {#if inputIndex !== recipients.length}
      <button
        type="button"
        aria-label="Set recipient cursor at end"
        tabindex="-1"
        onmousedown={(e) => handleSlotMouseDown(e, recipients.length)}
        ondragenter={(e) => handleDragEnter(e, recipients.length)}
        ondragover={(e) => handleDragOver(e, recipients.length)}
        ondragleave={handleDragLeave}
        ondrop={(e) => handleDrop(e, recipients.length)}
        class="h-6 min-w-[150px] flex-1 cursor-text bg-transparent p-0"
      ></button>
    {/if}
  </div>

  <!-- Suggestions dropdown -->
  {#if showSuggestions}
    <div
      class="absolute top-full mt-1 w-72 overflow-hidden rounded-md border border-border bg-popover shadow-lg z-50"
      style="left: {suggestionsLeft}px; max-height: {SUGGESTION_MENU_HEIGHT}px;"
    >
      {#each visibleSuggestions as suggestion, visibleIndex (visibleIndex)}
        {@const index = suggestionWindowStart + visibleIndex}
        <button
          onmousedown={() => selectSuggestion(suggestion)}
          onpointermove={(e) => handleSuggestionPointerMove(e, index)}
          data-suggestion-index={index}
          class="flex h-[52px] w-full items-center gap-3 px-3 py-2 text-left transition-colors {suggestionKeyboardMode ? '' : 'hover:bg-muted'} {index === selectedIndex ? 'bg-muted' : ''}"
          type="button"
        >
          <!-- Avatar placeholder -->
          <div class="w-8 h-8 rounded-full bg-primary/10 flex items-center justify-center text-xs font-medium text-primary">
            {(suggestion.display_name || suggestion.email)[0].toUpperCase()}
          </div>
          <div class="flex-1 min-w-0">
            <div class="text-sm font-medium truncate">
              {suggestion.display_name || suggestion.email}
            </div>
            {#if suggestion.display_name}
              <div class="text-xs text-muted-foreground truncate">
                {suggestion.email}
              </div>
            {/if}
          </div>
        </button>
      {/each}
    </div>
  {/if}
</div>
