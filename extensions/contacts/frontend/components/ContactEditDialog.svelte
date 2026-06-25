<!--
  ContactEditDialog — minimal Edit dialog: display name, email(s), and note.
  The field section is rendered by the shared <ContactFieldsForm> component
  that AddContactDialog also uses. The patch sends only the edited fields, so
  any other data a contact might carry is left untouched.
-->
<script lang="ts">
  import { _ } from 'svelte-i18n'
  import * as Dialog from '$lib/components/ui/dialog'
  import { Button } from '$lib/components/ui/button'
  import Icon from '@iconify/svelte'
  import { updateContact } from '$extensions/contacts/frontend/stores/contactsView.svelte'
  import { toasts } from '$lib/stores/toast'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import ContactFieldsForm from './fields/ContactFieldsForm.svelte'
  import type { EmailRow } from './fields/types'
  // @ts-ignore - wailsjs bindings
  import type { v1 } from '$wailsjs/go/models'

  interface Props {
    open: boolean
    contact: v1.Contact | null
    onClose?: () => void
  }

  let { open = $bindable(false), contact, onClose }: Props = $props()

  let nameInput = $state('')
  let noteInput = $state('')
  let emails = $state<EmailRow[]>([])
  let saving = $state(false)
  let errors = $state<Record<string, string>>({})

  // Hydrate state from `contact` each time the dialog opens.
  $effect(() => {
    if (open && contact) {
      nameInput = contact.name ?? ''
      noteInput = contact.note ?? ''
      if (contact.emailItems && contact.emailItems.length > 0) {
        emails = contact.emailItems.map((e) => ({
          email: e.email,
          type: e.type ?? '',
          isPrimary: e.isPrimary ?? false,
        }))
      } else if (contact.emails && contact.emails.length > 0) {
        emails = contact.emails.map((e, i) => ({ email: e, type: '', isPrimary: i === 0 }))
      } else {
        emails = []
      }
      errors = {}
    }
  })

  $effect(() => {
    if (open) {
      dialogGuardOpen()
      return () => dialogGuardClose()
    }
  })

  const recordID = $derived(contact?.id ?? '')

  function isValidEmail(s: string): boolean {
    const t = s.trim().toLowerCase()
    if (t === '') return false
    if (!t.includes('@') || t.indexOf('@') === t.length - 1 || t.startsWith('@')) return false
    return true
  }

  function validate(): boolean {
    const next: Record<string, string> = {}
    if (nameInput.trim() === '') {
      next.name = $_('contacts.edit.nameRequired')
    }
    emails.forEach((e, i) => {
      if (e.email.trim() !== '' && !isValidEmail(e.email)) {
        next[`email-${i}`] = $_('contacts.edit.emailInvalid')
      }
    })
    errors = next
    return Object.keys(next).length === 0
  }

  async function save() {
    if (!recordID) return
    if (!validate()) return
    saving = true
    try {
      // Only the edited fields are sent; absent fields are left untouched by
      // the backend patch. Cast through unknown — the runtime accepts plain
      // objects (JSON marshaling) without instantiating the Wails class.
      const patch = ({
        name: nameInput.trim(),
        note: noteInput.trim(),
        emails: emails
          .filter((e) => e.email.trim() !== '')
          .map((e) => ({ email: e.email.trim().toLowerCase(), type: e.type, isPrimary: e.isPrimary })),
      }) as unknown as v1.ContactPatch
      await updateContact(recordID, patch)
      toasts.success($_('contacts.toast.updated'))
      close()
    } catch (err) {
      console.error('Failed to update contact:', err)
      const msg = (err as Error)?.message ?? String(err)
      toasts.error(`${$_('contacts.toast.failedUpdate')}: ${msg}`)
    } finally {
      saving = false
    }
  }

  function close() {
    open = false
    onClose?.()
  }
</script>

<Dialog.Root bind:open onOpenChange={(v) => { if (!v) close() }}>
  <Dialog.Content class="max-w-xl max-h-[85vh] overflow-y-auto">
    <Dialog.Header>
      <Dialog.Title>{$_('contacts.edit.title')}</Dialog.Title>
    </Dialog.Header>

    <div class="mt-2">
      <ContactFieldsForm
        bind:nameInput
        bind:noteInput
        bind:emails
        errors={errors}
        saving={saving}
      />
    </div>

    <div class="flex items-center justify-end gap-2 pt-4 border-t border-border mt-4 sticky bottom-0 bg-background">
      <Button variant="ghost" onclick={close} disabled={saving}>{$_('contacts.common.cancel')}</Button>
      <Button onclick={save} disabled={saving || !recordID}>
        {#if saving}
          <Icon icon="mdi:loading" class="w-4 h-4 mr-1 animate-spin" />
        {/if}
        {$_('contacts.common.save')}
      </Button>
    </div>
  </Dialog.Content>
</Dialog.Root>
