<script lang="ts" module>
  import type { FieldConstraints, SourceTypeID } from './types'

  // Per-source slot rule table. Kept (and exported) so the dialogs can still
  // call slotConstraintsFor() for their save-time validation, even though the
  // minimal form only renders the email constraint.
  export function slotConstraintsFor(_sourceType: SourceTypeID): FieldConstraints {
    return {
      emails: { kind: 'none' },
      phones: { kind: 'none' },
      addresses: { kind: 'none' },
      urls: { kind: 'none' },
      impps: { kind: 'none' },
    }
  }
</script>

<script lang="ts">
  // ContactFieldsForm — the minimal contact form: display name, email(s), and
  // note. Everything else (nickname, phones, addresses, URLs, IMPPs, org,
  // title, categories, birthday, photo) was intentionally removed for an
  // extremely lean address book. Shared by AddContactDialog and
  // ContactEditDialog so the two stay in lockstep.

  import { _ } from 'svelte-i18n'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import EmailsField from './EmailsField.svelte'
  import type { EmailRow } from './types'

  interface Props {
    nameInput: string
    noteInput: string
    emails: EmailRow[]
    errors?: Record<string, string>
    saving?: boolean
  }

  let {
    nameInput = $bindable(''),
    noteInput = $bindable(''),
    emails = $bindable([]),
    errors = {},
    saving = false,
  }: Props = $props()
</script>

<div class="space-y-4">
  <!-- Display name -->
  <div class="flex items-start gap-4">
    <Label for="cf-name" class="w-20 shrink-0 pt-3">{$_('contacts.edit.name')}</Label>
    <div class="min-w-0 flex-1">
      <Input
        id="cf-name"
        type="text"
        bind:value={nameInput}
        disabled={saving}
        aria-invalid={errors.name ? 'true' : undefined}
      />
      {#if errors.name}
        <p class="text-xs text-destructive mt-1">{errors.name}</p>
      {/if}
    </div>
  </div>

  <!-- Emails -->
  <EmailsField bind:emails errors={errors} disabled={saving} />

  <!-- Note -->
  <div class="flex items-start gap-4">
    <Label for="cf-note" class="w-20 shrink-0 pt-3">{$_('contacts.edit.note')}</Label>
    <div class="min-w-0 flex-1">
      <textarea
        id="cf-note"
        bind:value={noteInput}
        disabled={saving}
        rows={3}
        class="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring resize-y"
      ></textarea>
    </div>
  </div>
</div>
