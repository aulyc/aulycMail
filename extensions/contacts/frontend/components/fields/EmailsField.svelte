<script lang="ts">
  // EmailsField — minimal repeating email rows: address input + remove + Add.
  // Type and primary selection were dropped for a leaner contact form; the
  // first email is treated as primary by the dialogs' build step.

  import { _ } from 'svelte-i18n'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import { Button } from '$lib/components/ui/button'
  import Icon from '@iconify/svelte'
  import type { EmailRow, SlotConstraint } from './types'

  interface Props {
    emails: EmailRow[]
    errors?: Record<string, string>
    disabled?: boolean
    constraint?: SlotConstraint
  }

  let { emails = $bindable([]), errors = {}, disabled = false, constraint = { kind: 'none' } }: Props = $props()

  const atMax = $derived(
    constraint.kind === 'max' && emails.length >= constraint.max,
  )
  const maxReason = $derived(constraint.kind === 'max' ? constraint.reason : '')

  function add() {
    emails = [...emails, { email: '', type: '', isPrimary: emails.length === 0 }]
  }
  function remove(i: number) {
    emails = emails.filter((_, idx) => idx !== i)
  }
</script>

<div>
  <Label>{$_('contacts.edit.emails')}</Label>
  <div class="space-y-2">
    {#each emails as e, i (i)}
      <div class="flex gap-2 items-start">
        <div class="flex-1">
          <Input
            type="email"
            bind:value={e.email}
            placeholder={$_('contacts.edit.emailPlaceholder')}
            disabled={disabled}
            aria-invalid={errors[`email-${i}`] ? 'true' : undefined}
          />
          {#if errors[`email-${i}`]}
            <p class="text-xs text-destructive mt-1">{errors[`email-${i}`]}</p>
          {/if}
        </div>
        <Button
          variant="ghost"
          size="icon"
          onclick={() => remove(i)}
          disabled={disabled}
          aria-label={$_('contacts.edit.removeEmail')}
        >
          <Icon icon="mdi:close" class="w-4 h-4" />
        </Button>
      </div>
    {/each}
  </div>
  <Button variant="outline" size="sm" onclick={add} disabled={disabled || atMax} class="mt-2" title={atMax ? maxReason : undefined}>
    <Icon icon="mdi:plus" class="w-4 h-4 mr-1" />
    {$_('contacts.edit.addEmail')}
  </Button>
</div>
