<script lang="ts">
  // AddContactDialog — creates a new contact in the single local address book.
  // Hosts the shared ContactFieldsForm so users can add a contact with phones,
  // addresses, URLs, IMPPs, photo, and the rest of the rich fields in one step.
  // The contact is always created as a local manual entry ('local:manual');
  // the 'collected' kind is reserved for mail auto-collection.

  import { untrack } from 'svelte'
  import { _ } from 'svelte-i18n'
  import * as Dialog from '$lib/components/ui/dialog'
  import { Button } from '$lib/components/ui/button'
  import Icon from '@iconify/svelte'
  import { createContact } from '$extensions/contacts/frontend/stores/contactsView.svelte'
  import { toasts } from '$lib/stores/toast'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import ContactFieldsForm, { slotConstraintsFor } from './fields/ContactFieldsForm.svelte'
  import type {
    EmailRow,
    PhoneRow,
    AddressRow,
    URLRow,
    IMPPRow,
    PhotoState,
  } from './fields/types'
  // @ts-ignore - wailsjs bindings
  import { v1 } from '$wailsjs/go/models'

  interface Props {
    open: boolean
    onClose?: () => void
    onCreated?: (id: string) => void
  }

  let { open = $bindable(false), onClose, onCreated }: Props = $props()

  // Local manual entry is the only write target. 'local:manual' is the only
  // writable local kind; 'local:collected' is reserved for mail collection.
  const LOCAL_VALUE = 'local:manual'

  let saving = $state(false)
  let errors = $state<Record<string, string>>({})

  // Field-form state — scalar + repeating, mirrors ContactEditDialog.
  let nameInput = $state('')
  let nicknameInput = $state('')
  let orgInput = $state('')
  let titleInput = $state('')
  let noteInput = $state('')
  let bdayInput = $state('')
  let categoriesInput = $state('')
  let emails = $state<EmailRow[]>([{ email: '', type: '', isPrimary: true }])
  let phones = $state<PhoneRow[]>([])
  let addresses = $state<AddressRow[]>([])
  let urls = $state<URLRow[]>([])
  let impps = $state<IMPPRow[]>([])
  let photo = $state<PhotoState>({ data: '', mediaType: '', url: '' })

  // Reset state each time the dialog opens.
  $effect(() => {
    if (!open) return
    untrack(() => {
      nameInput = ''
      nicknameInput = ''
      orgInput = ''
      titleInput = ''
      noteInput = ''
      bdayInput = ''
      categoriesInput = ''
      emails = [{ email: '', type: '', isPrimary: true }]
      phones = []
      addresses = []
      urls = []
      impps = []
      photo = { data: '', mediaType: '', url: '' }
      errors = {}
      saving = false
    })
  })

  $effect(() => {
    if (open) {
      dialogGuardOpen()
      return () => dialogGuardClose()
    }
  })

  function isValidEmail(s: string): boolean {
    const t = s.trim().toLowerCase()
    if (t === '') return false
    if (!t.includes('@') || t.indexOf('@') === t.length - 1 || t.startsWith('@')) return false
    return true
  }

  function validate(): boolean {
    const next: Record<string, string> = {}

    // Need at least one valid email — that's the contact's identity.
    const nonEmpty = emails.filter(e => e.email.trim() !== '')
    if (nonEmpty.length === 0) {
      next.email = $_('contacts.add.errorEmailRequired')
    } else {
      emails.forEach((e, i) => {
        if (e.email.trim() !== '' && !isValidEmail(e.email)) {
          next[`email-${i}`] = $_('contacts.add.errorEmailInvalid')
        }
      })
    }

    // Slot guards (e.g. phone type caps) surfaced from the shared constraints.
    const constraints = slotConstraintsFor('local')
    if (constraints.phones.kind === 'maxByType') {
      const c = constraints.phones
      const target = c.type.toLowerCase()
      const count = phones.filter(p => p.type.toLowerCase() === target).length
      if (count > c.max) {
        toasts.error(c.reason)
        return false
      }
    }

    errors = next
    return Object.keys(next).length === 0
  }

  function close() {
    open = false
    onClose?.()
  }

  function handleSaveError(err: unknown) {
    const msg = (err as Error)?.message ?? String(err)
    if (/already exists/i.test(msg) || /UNIQUE constraint/i.test(msg)) {
      errors = { ...errors, email: $_('contacts.add.errorEmailExists') }
      return
    }
    console.error('Failed to create contact:', err)
    toasts.error(`${$_('contacts.toast.failedAdd')}: ${msg}`)
  }

  // Build the rich ContactCreateInput from form state. Empty repeater rows
  // are filtered before send. The first non-empty email is treated as the
  // legacy primary if no row carries IsPrimary explicitly.
  function buildCreateInput(): v1.ContactCreateInput {
    const filteredEmails = emails
      .filter(e => e.email.trim() !== '')
      .map(e => ({ email: e.email.trim().toLowerCase(), type: e.type, isPrimary: e.isPrimary }))
    if (!filteredEmails.some(e => e.isPrimary) && filteredEmails.length > 0) {
      filteredEmails[0].isPrimary = true
    }
    const primaryEmail = filteredEmails.find(e => e.isPrimary)?.email ?? filteredEmails[0]?.email ?? ''
    const photoForApi = photo.data ? { data: photo.data, mediaType: photo.mediaType, url: '' } : undefined
    const categories = categoriesInput
      .split(',')
      .map(c => c.trim())
      .filter(c => c.length > 0)

    return v1.ContactCreateInput.createFrom({
      sourceId: LOCAL_VALUE,
      addressbookId: '',
      email: primaryEmail,
      name: nameInput.trim(),
      nickname: nicknameInput.trim(),
      org: orgInput.trim(),
      title: titleInput.trim(),
      note: noteInput.trim(),
      bday: bdayInput.trim(),
      categories: categories.length > 0 ? categories : undefined,
      emails: filteredEmails.length > 0 ? filteredEmails : undefined,
      phones: phones
        .filter(p => p.number.trim() !== '')
        .map(p => ({ number: p.number.trim(), type: p.type, isPrimary: p.isPrimary })),
      addresses: addresses
        .filter(a => a.street || a.city || a.region || a.postcode || a.country)
        .map(a => ({
          type: a.type,
          street: a.street.trim(),
          city: a.city.trim(),
          region: a.region.trim(),
          postcode: a.postcode.trim(),
          country: a.country.trim(),
        })),
      urls: urls
        .filter(u => u.url.trim() !== '')
        .map(u => ({ url: u.url.trim(), type: u.type })),
      impps: impps
        .filter(i => i.handle.trim() !== '')
        .map(i => ({ handle: i.handle.trim(), type: i.type })),
      photo: photoForApi,
    })
  }

  async function save() {
    if (!validate()) return
    saving = true
    try {
      const input = buildCreateInput()
      const id = await createContact(input)
      toasts.success($_('contacts.toast.added'))
      onCreated?.(id)
      close()
    } catch (err) {
      handleSaveError(err)
    } finally {
      saving = false
    }
  }
</script>

<Dialog.Root bind:open onOpenChange={(v) => { if (!v) close() }}>
  <Dialog.Content class="max-w-xl max-h-[85vh] overflow-y-auto">
    <Dialog.Header>
      <Dialog.Title>{$_('contacts.add.title')}</Dialog.Title>
      <Dialog.Description>
        {$_('contacts.add.description')}
      </Dialog.Description>
    </Dialog.Header>

    <div class="space-y-5 mt-2">
      {#if errors.email}
        <p class="text-xs text-destructive">{errors.email}</p>
      {/if}

      <!-- Rich field form -->
      <ContactFieldsForm
        bind:nameInput
        bind:nicknameInput
        bind:orgInput
        bind:titleInput
        bind:noteInput
        bind:bdayInput
        bind:categoriesInput
        bind:emails
        bind:phones
        bind:addresses
        bind:urls
        bind:impps
        bind:photo
        errors={errors}
        saving={saving}
        sourceType={'local'}
      />
    </div>

    <div class="flex items-center justify-end gap-2 pt-4 border-t border-border mt-4 sticky bottom-0 bg-background">
      <Button variant="ghost" onclick={close} disabled={saving}>{$_('contacts.common.cancel')}</Button>
      <Button onclick={save} disabled={saving}>
        {#if saving}
          <Icon icon="mdi:loading" class="w-4 h-4 mr-1 animate-spin" />
        {/if}
        {$_('contacts.common.save')}
      </Button>
    </div>
  </Dialog.Content>
</Dialog.Root>
