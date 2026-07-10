<script lang="ts">
  import Icon from '@iconify/svelte'
  import { formatRelativeDate } from '$lib/utils/date'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { message } from '../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import { Star, Unstar } from '../../../../wailsjs/go/app/App'
  import { toasts } from '$lib/stores/toast'
  import { getAccentBarUnread } from '$lib/stores/settings.svelte'

  interface Props {
    conversation: message.Conversation
    density?: 'micro' | 'compact' | 'standard' | 'large'
    selected: boolean
    checked: boolean
    accountId: string
    folderId: string
    selectedMessageIds: string[]  // All message IDs from checked conversations (for multi-select)
    rowIndex?: number
    showAccountIndicator?: boolean  // Show account color dot in unified inbox view
    accountColor?: string           // Account color for the indicator
    accountName?: string            // Account name for tooltip
    highlightedSubject?: string     // Subject with <mark> tags for search highlighting
    highlightedSnippet?: string     // Snippet with <mark> tags for search highlighting
    highlightedFromName?: string    // From name with <mark> tags for search highlighting
    searchFolderName?: string       // Folder name to display in search results
    searchFolderType?: string       // Folder type for icon in search results
    isNonLocal?: boolean            // Show cloud icon for non-local server search results
    onSelect: (e?: MouseEvent) => void
    onContextMenu?: (e: MouseEvent) => void
    onActionComplete?: (autoSelectNext?: boolean) => void
    onOpenDraft?: () => void  // Double-click in the Drafts folder → open the draft in the composer
  }

  let {
    conversation,
    density = 'standard',
    selected,
    checked,
    accountId,
    folderId: _folderId,
    selectedMessageIds,
    rowIndex,
    showAccountIndicator = false,
    accountColor = '',
    accountName = '',
    highlightedSubject = '',
    highlightedSnippet = '',
    highlightedFromName = '',
    searchFolderName = '',
    searchFolderType: _searchFolderType = '',
    isNonLocal = false,
    onSelect,
    onContextMenu,
    onActionComplete,
    onOpenDraft,
  }: Props = $props()

  // Check if we're in search mode (have highlighted content)
  const isSearchResult = $derived(!!highlightedSubject || !!highlightedSnippet)

  // Density-based class mappings
  // micro = smallest (power users), compact = small, standard = default, large = accessibility
	  const densityClasses = {
	    row: {
	      micro: 'px-3 py-2 gap-2',
	      compact: 'px-4 py-3 gap-3',
	      standard: 'px-5 py-3.5 gap-3.5',
	      large: 'px-6 py-5 gap-5',
	    },
	    avatar: {
	      micro: 'w-8 h-8 text-xs',
	      compact: 'w-10 h-10 text-sm',
	      standard: 'w-11 h-11 text-sm',
	      large: 'w-14 h-14 text-lg',
	    },
	    senderText: {
	      micro: 'text-xs',
	      compact: 'text-sm',
	      standard: 'text-[15px]',
	      large: 'text-lg',
	    },
	    text: {
	      micro: 'text-[10px]',
	      compact: 'text-xs',
	      standard: 'text-sm',
	      large: 'text-base',
	    },
	    dateText: {
	      micro: 'text-[10px]',
	      compact: 'text-xs',
	      standard: 'text-sm',
	      large: 'text-base',
	    },
	    icon: {
	      micro: 'w-3 h-3',
	      compact: 'w-3.5 h-3.5',
	      standard: 'w-4 h-4',
	      large: 'w-5 h-5',
	    },
	    starIcon: {
	      micro: 'w-3.5 h-3.5',
	      compact: 'w-4 h-4',
	      standard: 'w-5 h-5',
	      large: 'w-6 h-6',
	    },
	    badge: {
	      micro: 'px-1 py-0 text-[10px]',
	      compact: 'px-1.5 py-0.5 text-xs',
	      standard: 'px-2 py-1 text-xs',
	      large: 'px-2.5 py-1 text-sm',
	    },
	  }

	  const densityRowHeight = {
	    micro: 66,
	    compact: 80,
	    standard: 94,
	    large: 120,
	  }

  // Get display name for participants
  function getParticipantNames(): string {
    if (!conversation.participants || conversation.participants.length === 0) {
      return $_('viewer.unknown')
    }

    const names = conversation.participants.map((p) => p.name || p.email.split('@')[0])

    if (names.length === 1) {
      return names[0]
    } else if (names.length === 2) {
      return names.join(', ')
    } else {
      return `${names[0]}, ${names[1]} +${names.length - 2}`
    }
  }

  async function handleStarClick(e: MouseEvent) {
    e.stopPropagation()
    const starring = !conversation.isStarred
    try {
      if (starring) {
        await Star(ownMessageIds)
        toasts.success($_('toast.starred'))
      }
      if (!starring) {
        await Unstar(ownMessageIds)
        toasts.success($_('toast.starRemoved'))
      }
      onActionComplete?.()
    } catch (err) {
      console.error('Star toggle failed:', err)
      toasts.error($_('toast.failedToUpdateStar'))
    }
  }

  const hasUnread = $derived((conversation.unreadCount || 0) > 0)

  // Get message IDs from the conversation for context menu
  // Use messageIds field (populated by ListConversationsByFolder), fallback to messages array
  const ownMessageIds = $derived(
    conversation.messageIds || conversation.messages?.map((m) => m.id) || []
  )
  const composeStatus = $derived((conversation as any).composeStatus || '')
  const composeAction = $derived((conversation as any).composeAction || '')

  // Drag start handler: stash messageIds + sourceAccountId in dataTransfer so
  // the folder drop target can move them via MoveToFolder(). If this row is
  // part of the multi-select, drag the whole checked set; otherwise drag just
  // this row's messages (matches the context-menu selection rule).
  function handleDragStart(e: DragEvent) {
    if (!e.dataTransfer) return
    const messageIds = checked ? selectedMessageIds : ownMessageIds
    if (messageIds.length === 0) return
    const payload = JSON.stringify({ messageIds, sourceAccountId: accountId })
    e.dataTransfer.setData('application/x-aulycmail-messages', payload)
    e.dataTransfer.effectAllowed = 'move'
  }

  function getComposeStatusIcon(status: string, action: string): string {
    if (status === 'draft') return 'mdi:file-document-edit-outline'
    if (action === 'reply-all') return 'mdi:reply-all'
    if (action === 'reply') return 'mdi:reply'
    if (action === 'forward') return 'mdi:share'
    return ''
  }

  function getComposeStatusTitle(status: string, action: string): string {
    const suffix = action === 'reply-all' ? 'ReplyAll' : action === 'forward' ? 'Forward' : 'Reply'
    return $_(`messageList.composeStatus${status === 'draft' ? 'Draft' : 'Sent'}${suffix}`)
  }
</script>

	<div
	  data-conversation-row
	  data-row-index={rowIndex}
	  draggable="true"
	  style="height: {densityRowHeight[density]}px; min-height: {densityRowHeight[density]}px;"
	  class="group w-full flex items-start {densityClasses.row[density]} text-left border-b border-border transition-colors duration-300 cursor-pointer outline-none {selected
	    ? 'bg-primary/20'
	    : 'hover:bg-muted/50'} {getAccentBarUnread() && hasUnread ? 'border-l-[3px] border-l-primary' : ''}"
  onclick={(e) => onSelect(e)}
  ondblclick={() => onOpenDraft?.()}
  onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onSelect() }}}
  ondragstart={handleDragStart}
  oncontextmenu={onContextMenu}
  role="button"
  tabindex="0"
>
  <!-- Content -->
  <div class="flex-1 min-w-0">
    <div class="flex items-center gap-2 mb-0.5">
      <!-- Star (before the name; vertically centered with name + date).
             -my-1 cancels the button's vertical padding so it doesn't inflate
             the row height — keeps the click target, lifts the line up a touch. -->
      <button
        class="flex-shrink-0 p-1 -ml-1 -my-1 rounded hover:bg-muted transition-colors duration-200"
        onclick={handleStarClick}
      >
        <Icon
          icon={conversation.isStarred ? 'mdi:star' : 'mdi:star-outline'}
          class="{densityClasses.starIcon[density]} {conversation.isStarred ? 'text-yellow-500' : 'text-muted-foreground'}"
        />
      </button>

        <!-- Account Indicator (for unified inbox) -->
        {#if showAccountIndicator && accountColor}
          <span
            class="w-2 h-2 rounded-full flex-shrink-0"
            style="background-color: {accountColor}"
            title={accountName}
          ></span>
        {/if}

        <!-- Participant Names (with highlighting if in search mode) -->
        {#if highlightedFromName}
          <span class="{densityClasses.senderText[density]} truncate {hasUnread ? 'font-semibold text-foreground' : 'text-foreground'}">
            <!-- eslint-disable-next-line svelte/no-at-html-tags -- highlightMatches only inserts <mark> around already-escaped text -->
            {@html highlightedFromName}
          </span>
        {:else}
          <span class="{densityClasses.senderText[density]} truncate {hasUnread ? 'font-semibold text-foreground' : 'text-foreground'}">
            {getParticipantNames()}
          </span>
        {/if}

        <!-- Message Count Badge -->
        {#if conversation.messageCount > 1}
          <span
            class="flex-shrink-0 {densityClasses.badge[density]} rounded-full bg-muted text-muted-foreground"
          >
            {conversation.messageCount}
          </span>
        {/if}

        <!-- Folder Badge (for search results) -->
        {#if isSearchResult && searchFolderName}
          <span
            class="flex-shrink-0 {densityClasses.badge[density]} rounded bg-muted/50 text-muted-foreground flex items-center gap-1"
            title={$_('messageList.foundIn', { values: { folder: searchFolderName } })}
          >
            <Icon icon="mdi:folder-outline" class="w-3 h-3" />
            {searchFolderName}
          </span>
        {/if}

        <!-- Indicators -->
        <div class="flex items-center gap-1 flex-shrink-0">
          {#if isNonLocal}
            <span title={$_('search.notSyncedLocally')}>
              <Icon icon="mdi:cloud-outline" class="{densityClasses.icon[density]} text-muted-foreground" />
            </span>
          {/if}
          {#if composeStatus && getComposeStatusIcon(composeStatus, composeAction)}
            <span title={getComposeStatusTitle(composeStatus, composeAction)}>
              <Icon
                icon={getComposeStatusIcon(composeStatus, composeAction)}
                class="{densityClasses.icon[density]} {composeStatus === 'draft' ? 'text-muted-foreground' : 'text-primary'}"
              />
            </span>
          {/if}
          {#if conversation.hasAttachments}
            <Icon icon="mdi:paperclip" class="{densityClasses.icon[density]} text-muted-foreground" />
          {/if}
        </div>

        <!-- Date -->
        <span class="{densityClasses.dateText[density]} text-muted-foreground flex-shrink-0 ml-auto">
          {formatRelativeDate(new Date(conversation.latestDate))}
        </span>
      </div>

      <!-- Subject (with highlighting if in search mode) -->
      {#if highlightedSubject}
        <p
          class="truncate {densityClasses.text[density]} {hasUnread ? 'font-medium text-foreground' : 'text-muted-foreground'}"
        >
          <!-- eslint-disable-next-line svelte/no-at-html-tags -- highlightMatches only inserts <mark> around already-escaped text -->
          {@html highlightedSubject}
        </p>
      {:else}
        <p
          class="truncate {densityClasses.text[density]} {hasUnread ? 'font-medium text-foreground' : 'text-muted-foreground'}"
        >
          {conversation.subject || $_('viewer.noSubject')}
        </p>
      {/if}

      <!-- Snippet (with highlighting if in search mode) -->
      {#if highlightedSnippet}
        <p class="truncate {densityClasses.text[density]} text-muted-foreground">
          <!-- eslint-disable-next-line svelte/no-at-html-tags -- highlightMatches only inserts <mark> around already-escaped text -->
          {@html highlightedSnippet}
        </p>
      {:else if conversation.snippet}
        <p class="truncate {densityClasses.text[density]} text-muted-foreground">
          {conversation.snippet}
        </p>
      {:else if conversation.isEncrypted}
        <p class="truncate {densityClasses.text[density]} text-muted-foreground italic">
          {$_('messageList.encryptedContent')}
        </p>
      {:else}
        <p class="truncate {densityClasses.text[density]} text-muted-foreground italic">
          {$_('messageList.noContent')}
        </p>
      {/if}
    </div>
  </div>
