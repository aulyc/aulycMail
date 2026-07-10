<script lang="ts">
  // AddContactDialog — creates a new contact in the single local address book.
  // Minimal form: display name, email(s), and note. Always created as a local
  // manual entry ('local:manual'); 'collected' is reserved for mail collection.

  import { untrack } from 'svelte'
  import { _ } from 'svelte-i18n'
  import * as Dialog from '$lib/components/ui/dialog'
  import { Button } from '$lib/components/ui/button'
  import Icon from '@iconify/svelte'
  import { createContact } from '$contacts/stores/contactsView.svelte'
  import { toasts } from '$lib/stores/toast'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import ContactFieldsForm from './fields/ContactFieldsForm.svelte'
  import type { EmailRow } from './fields/types'
  // @ts-ignore - wailsjs bindings
  import { contactdto } from '$wailsjs/go/models'

  interface Props {
    open: boolean
    onClose?: () => void
    onCreated?: (id: string) => void
  }

  let { open = $bindable(false), onClose, onCreated }: Props = $props()

  // Local manual entry is the only write target.
  const LOCAL_VALUE = 'local:manual'

  let saving = $state(false)
  let errors = $state<Record<string, string>>({})

  let nameInput = $state('')
  let noteInput = $state('')
  let emails = $state<EmailRow[]>([{ email: '', type: '', isPrimary: true }])

  // Reset state each time the dialog opens.
  $effect(() => {
    if (!open) return
    untrack(() => {
      nameInput = ''
      noteInput = ''
      emails = [{ email: '', type: '', isPrimary: true }]
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

  function buildCreateInput(): contactdto.ContactCreateInput {
    const filteredEmails = emails
      .filter(e => e.email.trim() !== '')
      .map(e => ({ email: e.email.trim().toLowerCase(), type: e.type, isPrimary: e.isPrimary }))
    if (!filteredEmails.some(e => e.isPrimary) && filteredEmails.length > 0) {
      filteredEmails[0].isPrimary = true
    }
    const primaryEmail = filteredEmails.find(e => e.isPrimary)?.email ?? filteredEmails[0]?.email ?? ''

    return contactdto.ContactCreateInput.createFrom({
      sourceId: LOCAL_VALUE,
      email: primaryEmail,
      name: nameInput.trim(),
      note: noteInput.trim(),
      emails: filteredEmails.length > 0 ? filteredEmails : undefined,
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
      <Button onclick={save} disabled={saving}>
        {#if saving}
          <Icon icon="mdi:loading" class="w-4 h-4 mr-1 animate-spin" />
        {/if}
        {$_('contacts.common.save')}
      </Button>
    </div>
  </Dialog.Content>
</Dialog.Root>
