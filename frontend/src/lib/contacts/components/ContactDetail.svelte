<script lang="ts">
  import { _ } from 'svelte-i18n'
  import DetailPane from '$lib/components/kit/DetailPane.svelte'
  import { Button } from '$lib/components/ui/button'
  import ConfirmDialog from '$lib/components/kit/ConfirmDialog.svelte'
  import Icon from '@iconify/svelte'
  import { contactsView, deleteLocalContact } from '$contacts/stores/contactsView.svelte'
  import { toasts } from '$lib/stores/toast'
  import { formatLocalDateTime, formatRelativeDate } from '$lib/utils/date'
  // @ts-ignore - wailsjs bindings
  import type { contactdto } from '$wailsjs/go/models'
  // @ts-ignore - wailsjs bindings
  import { GetContactMessages } from '$wailsjs/go/app/App'
  // @ts-ignore - wailsjs bindings
  import { EventsEmit } from '$wailsjs/runtime/runtime'

  // Compact mail summary returned by GetContactMessages (mirrors the Go
  // message.ContactMessage struct).
  interface ContactMessage {
    id: string
    threadId: string
    accountId: string
    accountName?: string
    accountEmail?: string
    folderId: string
    subject: string
    fromName: string
    fromEmail: string
    date: string
    isRead: boolean
    incoming: boolean
  }

  let relatedMessages = $state<ContactMessage[]>([])
  let loadingMessages = $state(false)

  // Load the contact's related mail whenever the selected contact changes.
  $effect(() => {
    const email = primaryEmail
    if (!email) {
      relatedMessages = []
      return
    }
    loadingMessages = true
    let cancelled = false
    GetContactMessages(email, 50)
      .then((msgs: ContactMessage[]) => {
        if (!cancelled) relatedMessages = msgs || []
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          console.error('Failed to load contact messages:', err)
          relatedMessages = []
        }
      })
      .finally(() => {
        if (!cancelled) loadingMessages = false
      })
    return () => { cancelled = true }
  })

  // Jump to the mail view and open this conversation. App.svelte listens for
  // 'mail:openConversation' and handles the rail switch + navigation.
  function openConversation(m: ContactMessage) {
    EventsEmit('mail:openConversation', {
      accountId: m.accountId,
      folderId: m.folderId,
      threadId: m.threadId,
    })
  }

  // Edit-dialog state lives in ContactsPane (hoisted so the 'e' keyboard
  // shortcut can open it from anywhere within the pane). The button below
  // calls onEdit; ContactsPane owns the dialog itself.
  interface Props {
    onEdit?: (contact: contactdto.Contact) => void
  }
  let { onEdit }: Props = $props()

  let contact = $derived(contactsView.detail)
  let primaryEmail = $derived(contact && contact.emails && contact.emails.length > 0 ? contact.emails[0] : '')
  let associatedAccounts = $derived(contact?.associatedAccounts ?? [])

  let showDeleteConfirm = $state(false)
  let deleting = $state(false)

  async function copyEmail(email: string) {
    try {
      await navigator.clipboard.writeText(email)
      toasts.success($_('contacts.toast.emailCopied', { values: { email } }))
    } catch {
      toasts.error($_('contacts.toast.emailCopyFailed'))
    }
  }

  function handleKeydown(e: KeyboardEvent, email: string) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      e.stopPropagation()
      copyEmail(email)
    }
  }

  async function confirmDelete() {
    if (!contact) return
    deleting = true
    try {
      await deleteLocalContact(contact.id)
      toasts.success($_('contacts.toast.deleted'))
    } catch (err) {
      console.error('Failed to delete contact:', err)
      toasts.error($_('contacts.toast.failedDelete'))
    } finally {
      deleting = false
    }
  }
</script>

<DetailPane
  empty={!contact}
  emptyIcon="mdi:account-multiple-outline"
  emptyText={$_('contacts.detail.emptyState')}
>
  {#snippet header()}
    {#if contact}
      <h1 class="m-0 text-xl font-semibold text-foreground flex-1 min-w-0 truncate">
        {contact.name || $_('contacts.common.unnamed')}
      </h1>
      <div class="flex items-center gap-1 flex-shrink-0">
        <Button variant="outline" size="sm" onclick={() => { if (contact) onEdit?.(contact) }}>
          <Icon icon="mdi:pencil" class="w-4 h-4 mr-1" />
          {$_('contacts.detail.edit')}
        </Button>
        <Button
          variant="outline"
          size="sm"
          class="text-destructive hover:text-destructive"
          onclick={() => { showDeleteConfirm = true }}
        >
          <Icon icon="mdi:delete-outline" class="w-4 h-4 mr-1" />
          {$_('contacts.common.delete')}
        </Button>
      </div>
    {/if}
  {/snippet}

  {#snippet body()}
    {#if contact}
      <div class="h-full flex flex-col">
      <!-- Fixed contact info — stays put while the mail list scrolls -->
      <div class="flex-shrink-0">
      <dl class="grid grid-cols-[80px_1fr] gap-y-2 gap-x-3 items-baseline">
        <dt class="text-sm text-muted-foreground">{$_('contacts.detail.labels.displayName')}</dt>
        <dd class="m-0 break-words text-foreground">
          {contact.name || $_('contacts.common.unnamed')}
        </dd>

        <dt class="text-sm text-muted-foreground">{$_('contacts.detail.labels.email')}</dt>
        <dd class="m-0 break-words">
          {#if contact.emailItems && contact.emailItems.length > 0}
            {#each contact.emailItems as item (item.email)}
              <div class="flex items-baseline gap-2">
                <span
                  role="button"
                  tabindex="0"
                  class="text-primary hover:underline cursor-pointer"
                  title={$_('contacts.detail.copyTooltip')}
                  onclick={(e) => { e.stopPropagation(); copyEmail(item.email) }}
                  onkeydown={(e) => handleKeydown(e, item.email)}
                >{item.email}</span>
                {#if item.type}
                  <span class="text-xs text-muted-foreground uppercase">{item.type}</span>
                {/if}
              </div>
            {/each}
          {/if}
          {#if (!contact.emailItems || contact.emailItems.length === 0) && contact.emails && contact.emails.length > 0}
            {#each contact.emails as email (email)}
              <div>
                <span
                  role="button"
                  tabindex="0"
                  class="text-primary hover:underline cursor-pointer"
                  title={$_('contacts.detail.copyTooltip')}
                  onclick={(e) => { e.stopPropagation(); copyEmail(email) }}
                  onkeydown={(e) => handleKeydown(e, email)}
                >{email}</span>
              </div>
            {/each}
          {/if}
        </dd>

        {#if associatedAccounts.length > 0}
          <dt class="text-sm text-muted-foreground">{$_('contacts.detail.labels.associatedAccounts')}</dt>
          <dd class="m-0 break-words text-foreground space-y-1">
            {#each associatedAccounts as account (account.accountId)}
              <div class="break-all">{account.email || account.name}</div>
            {/each}
          </dd>
        {/if}

        {#if contact.phones && contact.phones.length > 0}
          <dt class="text-sm text-muted-foreground">{$_('contacts.detail.labels.phone')}</dt>
          <dd class="m-0 break-words text-foreground">
            {#each contact.phones as p (p.number + (p.type ?? ''))}
              <div class="flex items-baseline gap-2">
                <span>{p.number}</span>
                {#if p.type}
                  <span class="text-xs text-muted-foreground uppercase">{p.type}</span>
                {/if}
                {#if p.isPrimary}
                  <span class="text-xs text-primary">{$_('contacts.common.primary')}</span>
                {/if}
              </div>
            {/each}
          </dd>
        {/if}

        {#if contact.addresses && contact.addresses.length > 0}
          <dt class="text-sm text-muted-foreground">{$_('contacts.detail.labels.address')}</dt>
          <dd class="m-0 break-words text-foreground space-y-2">
            {#each contact.addresses as a, i (i)}
              <div>
                {#if a.type}
                  <span class="text-xs text-muted-foreground uppercase mr-2">{a.type}</span>
                {/if}
                <span>
                  {[a.street, a.city, a.region, a.postcode, a.country].filter(Boolean).join(', ')}
                </span>
              </div>
            {/each}
          </dd>
        {/if}

        {#if contact.org}
          <dt class="text-sm text-muted-foreground">{$_('contacts.detail.labels.org')}</dt>
          <dd class="m-0 break-words text-foreground">{contact.org}</dd>
        {/if}

        {#if contact.title}
          <dt class="text-sm text-muted-foreground">{$_('contacts.detail.labels.title')}</dt>
          <dd class="m-0 break-words text-foreground">{contact.title}</dd>
        {/if}

        {#if contact.bday}
          <dt class="text-sm text-muted-foreground">{$_('contacts.detail.labels.bday')}</dt>
          <dd class="m-0 break-words text-foreground">{contact.bday}</dd>
        {/if}

        {#if contact.nickname}
          <dt class="text-sm text-muted-foreground">{$_('contacts.detail.labels.nickname')}</dt>
          <dd class="m-0 break-words text-foreground">{contact.nickname}</dd>
        {/if}

        {#if contact.urls && contact.urls.length > 0}
          <dt class="text-sm text-muted-foreground">{$_('contacts.detail.labels.url')}</dt>
          <dd class="m-0 break-words text-foreground">
            {#each contact.urls as u (u.url + (u.type ?? ''))}
              <div class="flex items-baseline gap-2">
                <a href={u.url} target="_blank" rel="noopener noreferrer" class="text-primary hover:underline">{u.url}</a>
                {#if u.type}
                  <span class="text-xs text-muted-foreground uppercase">{u.type}</span>
                {/if}
              </div>
            {/each}
          </dd>
        {/if}

        {#if contact.impps && contact.impps.length > 0}
          <dt class="text-sm text-muted-foreground">{$_('contacts.detail.labels.im')}</dt>
          <dd class="m-0 break-words text-foreground">
            {#each contact.impps as i (i.handle + (i.type ?? ''))}
              <div class="flex items-baseline gap-2">
                <span>{i.handle}</span>
                {#if i.type}
                  <span class="text-xs text-muted-foreground uppercase">{i.type}</span>
                {/if}
              </div>
            {/each}
          </dd>
        {/if}

        {#if contact.categories && contact.categories.length > 0}
          <dt class="text-sm text-muted-foreground">{$_('contacts.detail.labels.categories')}</dt>
          <dd class="m-0 break-words text-foreground">
            <div class="flex flex-wrap gap-1">
              {#each contact.categories as cat (cat)}
                <span class="text-xs px-2 py-0.5 rounded-full bg-muted text-muted-foreground">{cat}</span>
              {/each}
            </div>
          </dd>
        {/if}

        {#if contact.note}
          <dt class="text-sm text-muted-foreground">{$_('contacts.detail.labels.note')}</dt>
          <dd class="m-0 break-words text-foreground whitespace-pre-wrap">{contact.note}</dd>
        {/if}

        <dt class="text-sm text-muted-foreground">{$_('contacts.detail.labels.lastUpdated')}</dt>
        <dd class="m-0 text-foreground">
          {contact.updatedAt ? formatLocalDateTime(contact.updatedAt) : '—'}
        </dd>
      </dl>

      <!-- Related mail heading — fixed with the contact info above -->
      <h2 class="text-sm text-muted-foreground mt-2 mb-2">{$_('contacts.detail.relatedMail')}</h2>
      </div>

      <!-- Scrolling related-mail list (pr-3 keeps the scrollbar off the date) -->
      <div data-keyboard-detail-scroll class="flex-1 min-h-0 overflow-y-auto scrollbar-thin border-t border-border pr-3">
        {#if loadingMessages}
          <div class="flex items-center gap-2 text-sm text-muted-foreground py-2">
            <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
            <span>{$_('contacts.detail.relatedMailLoading')}</span>
          </div>
        {:else if relatedMessages.length === 0}
          <p class="text-sm text-muted-foreground py-2">{$_('contacts.detail.relatedMailEmpty')}</p>
        {:else}
          <div class="divide-y divide-border border-b border-border">
            {#each relatedMessages as m (m.id)}
              <button
                type="button"
                class="w-full flex items-center gap-2 py-2 text-left cursor-pointer"
                onclick={() => openConversation(m)}
              >
                <Icon
                  icon={m.incoming ? 'mdi:email-arrow-left-outline' : 'mdi:email-arrow-right-outline'}
                  class="w-4 h-4 flex-shrink-0 text-muted-foreground"
                />
                <span class="flex-1 min-w-0 truncate {m.isRead ? 'text-foreground' : 'font-semibold text-foreground'}">
                  {m.subject || $_('contacts.common.unnamed')}
                </span>
                <span class="w-40 max-w-[40%] flex-shrink-0 truncate text-xs text-muted-foreground" title={m.accountEmail || m.accountName || ''}>
                  {m.accountEmail || m.accountName || '—'}
                </span>
                <span class="flex-shrink-0 text-xs text-muted-foreground whitespace-nowrap">
                  {formatRelativeDate(new Date(m.date))}
                </span>
              </button>
            {/each}
          </div>
        {/if}
      </div>
      </div>
    {/if}
  {/snippet}
</DetailPane>

<ConfirmDialog
  bind:open={showDeleteConfirm}
  title={$_('contacts.delete.title')}
  description={contact
    ? $_('contacts.delete.descriptionLocal', { values: { name: contact.name || primaryEmail || $_('contacts.common.unnamed') } })
    : ''}
  confirmLabel={$_('contacts.common.delete')}
  cancelLabel={$_('contacts.common.cancel')}
  variant="destructive"
  loading={deleting}
  onConfirm={confirmDelete}
/>
