<script lang="ts">
  import Icon from '@iconify/svelte'
  import * as Dialog from '$lib/components/ui/dialog'
  import { Button } from '$lib/components/ui/button'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import * as Select from '$lib/components/ui/select'
  import { _ } from '$lib/i18n'
  import SignatureEditor from './SignatureEditor.svelte'
  // @ts-ignore - wailsjs path
  import { account } from '../../../../../wailsjs/go/models'

  interface Props {
    /** Whether the dialog is open */
    open?: boolean
    /** Identity to edit (null for new identity) */
    identity?: account.Identity | null
    /** Account ID (required for creating new identity) */
    accountId: string
    /** Callback when dialog should close */
    onClose?: () => void
    /** Callback when identity is saved */
    onSave?: (config: account.IdentityConfig) => Promise<void>
  }

  let {
    open = $bindable(false),
    identity = null,
    accountId: _accountId,
    onClose,
    onSave,
  }: Props = $props()

  // Form state
  let email = $state('')
  let name = $state('')
  let signatureHtml = $state('')
  let signatureText = $state('')
  let signatureEnabled = $state(true)
  let signatureForNew = $state(true)
  let signatureForReply = $state(true)
  let signatureForForward = $state(true)
  let signaturePlacement = $state<'above' | 'below'>('above')
  let signatureSeparator = $state(false)

  let saving = $state(false)
  let errors = $state<Record<string, string>>({})

  // The default identity's email must stay in sync with the account's login
  // email (providers like 163 reject a From/envelope-sender that doesn't match
  // the authenticated account), so it's read-only — only aliases are editable.
  const isDefaultIdentity = $derived(identity?.isDefault ?? false)

  // Initialize form when identity changes
  $effect(() => {
    if (open) {
      if (identity) {
        // Editing existing identity
        email = identity.email || ''
        name = identity.name || ''
        signatureHtml = identity.signatureHtml || ''
        signatureText = identity.signatureText || ''
        signatureEnabled = identity.signatureEnabled ?? true
        signatureForNew = identity.signatureForNew ?? true
        signatureForReply = identity.signatureForReply ?? true
        signatureForForward = identity.signatureForForward ?? true
        signaturePlacement = (identity.signaturePlacement as 'above' | 'below') || 'above'
        signatureSeparator = identity.signatureSeparator ?? false
      } else {
        // New identity - reset to defaults
        email = ''
        name = ''
        signatureHtml = ''
        signatureText = ''
        signatureEnabled = true
        signatureForNew = true
        signatureForReply = true
        signatureForForward = true
        signaturePlacement = 'above'
        signatureSeparator = false
      }
      errors = {}
    }
  })

  function validate(): boolean {
    errors = {}

    if (!email.trim()) {
      errors.email = $_('identity.emailRequired')
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      errors.email = $_('identity.invalidEmailFormat')
    }

    if (!name.trim()) {
      errors.name = $_('identity.displayNameRequired')
    }

    return Object.keys(errors).length === 0
  }

  async function handleSave() {
    if (!validate()) return

    saving = true
    try {
      const config = new account.IdentityConfig({
        email: email.trim(),
        name: name.trim(),
        signatureHtml,
        signatureText,
        signatureEnabled,
        signatureForNew,
        signatureForReply,
        signatureForForward,
        signaturePlacement,
        signatureSeparator,
      })

      await onSave?.(config)
      open = false
      onClose?.()
    } catch (err) {
      console.error('Failed to save identity:', err)
      errors.general = $_('identity.saveFailed')
    } finally {
      saving = false
    }
  }

  function handleCancel() {
    open = false
    onClose?.()
  }

  function handleOpenChange(isOpen: boolean) {
    open = isOpen
    if (!isOpen) {
      onClose?.()
    }
  }

  // Convert HTML to plain text for the "Generate from HTML" button
  function generatePlainTextFromHtml() {
    if (!signatureHtml) return

    const temp = document.createElement('div')
    temp.innerHTML = signatureHtml

    // Replace <br> and block elements with newlines
    const blockElements = temp.querySelectorAll('p, div, br, li')
    blockElements.forEach(el => {
      if (el.tagName === 'BR') {
        el.replaceWith('\n')
      } else if (el.tagName === 'LI') {
        el.prepend(document.createTextNode('- '))
        el.append(document.createTextNode('\n'))
      } else {
        el.append(document.createTextNode('\n'))
      }
    })

    let text = temp.textContent || ''
    text = text.replace(/\n{3,}/g, '\n\n')
    signatureText = text.trim()
  }
</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
  <Dialog.Content class="max-w-2xl max-h-[90vh] overflow-hidden flex flex-col">
    <Dialog.Header>
      <Dialog.Title>
        {identity
          ? (identity.isDefault ? $_('identity.editDefaultEmailTitle') : $_('identity.editEmailTitle'))
          : $_('identity.addEmailTitle')}
      </Dialog.Title>
      <Dialog.Description>
        {identity
          ? (identity.isDefault ? $_('identity.editDefaultEmailDescription') : $_('identity.editEmailDescription'))
          : $_('identity.addEmailDescription')}
      </Dialog.Description>
    </Dialog.Header>

    <form onsubmit={(e) => { e.preventDefault(); handleSave() }} class="flex-1 min-h-0 flex flex-col overflow-hidden">
      <!-- Fixed-height scroll area: keeps the dialog height constant whether or
           not the signature section is shown. pr-3/pl-1/pt-1.5 keep the
           scrollbar + focus rings off the inputs. -->
      <div class="h-[460px] max-h-[calc(90vh-200px)] overflow-y-auto space-y-6 pt-1.5 pl-1 pr-3 pb-2">
      <!-- Email & Name (label + input on one row) -->
      <div class="space-y-4">
        <div>
          <div class="flex items-center justify-between gap-4">
            <div class="min-w-0">
              <Label for="email">{$_('identity.emailAddressLabel')}</Label>
              {#if isDefaultIdentity}
                <p class="text-xs text-muted-foreground">{$_('identity.defaultEmailReadonlyHelp')}</p>
              {/if}
            </div>
            <Input
              id="email"
              type="email"
              placeholder="you@example.com"
              bind:value={email}
              readonly={isDefaultIdentity}
              tabindex={isDefaultIdentity ? -1 : undefined}
              class="w-64 shrink-0 {isDefaultIdentity ? 'cursor-not-allowed opacity-60 focus-visible:ring-0 focus-visible:ring-offset-0' : ''} {errors.email ? 'border-destructive' : ''}"
            />
          </div>
          {#if errors.email}
            <p class="text-sm text-destructive mt-1">{errors.email}</p>
          {/if}
        </div>

        <div>
          <div class="flex items-center justify-between gap-4">
            <div class="min-w-0">
              <Label for="name">{$_('identity.displayNameLabel')}</Label>
              <p class="text-xs text-muted-foreground">{$_('identity.displayNameHelp')}</p>
            </div>
            <Input
              id="name"
              type="text"
              placeholder="John Smith"
              bind:value={name}
              class="w-64 shrink-0 {errors.name ? 'border-destructive' : ''}"
            />
          </div>
          {#if errors.name}
            <p class="text-sm text-destructive mt-1">{errors.name}</p>
          {/if}
        </div>
      </div>

      <!-- Divider -->
      <div class="border-t border-border"></div>

      <!-- Signature Section -->
      <div class="space-y-4">
        <div class="flex items-center gap-2">
          <input
            type="checkbox"
            id="signatureEnabled"
            bind:checked={signatureEnabled}
            class="w-4 h-4 rounded border-input accent-primary"
          />
          <Label for="signatureEnabled" class="cursor-pointer font-medium">
            {$_('identity.useSignature')}
          </Label>
        </div>

        {#if signatureEnabled}
          <!-- HTML Signature Editor -->
          <div class="space-y-2">
            <Label>{$_('identity.htmlSignature')}</Label>
            <SignatureEditor
              value={signatureHtml}
              placeholder="Enter your signature..."
              onchange={(html) => signatureHtml = html}
            />
            <p class="text-xs text-muted-foreground">
              {$_('identity.signatureHelp')}
            </p>
          </div>

          <!-- Plain Text Signature -->
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <Label for="signatureText">{$_('identity.plainTextSignature')}</Label>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onclick={generatePlainTextFromHtml}
                disabled={!signatureHtml}
                class="text-xs h-7"
              >
                {$_('identity.generateFromHtml')}
              </Button>
            </div>
            <textarea
              id="signatureText"
              bind:value={signatureText}
              placeholder="Plain text version for text-only emails..."
              class="w-full min-h-[80px] p-3 text-sm bg-background border border-input rounded-md resize-y focus:outline-none focus:ring-2 focus:ring-ring font-mono"
            ></textarea>
            <p class="text-xs text-muted-foreground">
              {$_('identity.plainTextHelp')}
            </p>
          </div>

          <!-- Divider -->
          <div class="border-t border-border"></div>

          <!-- Signature Behavior (label + checkboxes on one row) -->
          <div class="flex items-center justify-between gap-4">
            <Label class="font-medium">{$_('identity.appendSignatureTo')}</Label>
            <div class="flex items-center gap-4 shrink-0">
              <label class="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  bind:checked={signatureForNew}
                  class="w-4 h-4 rounded border-input accent-primary"
                />
                <span class="text-sm">{$_('identity.newMessages')}</span>
              </label>
              <label class="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  bind:checked={signatureForReply}
                  class="w-4 h-4 rounded border-input accent-primary"
                />
                <span class="text-sm">{$_('identity.replies')}</span>
              </label>
              <label class="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  bind:checked={signatureForForward}
                  class="w-4 h-4 rounded border-input accent-primary"
                />
                <span class="text-sm">{$_('identity.forwards')}</span>
              </label>
            </div>
          </div>

          <!-- Signature Placement (label + dropdown on one row) -->
          <div class="flex items-center justify-between gap-4">
            <Label class="font-medium">{$_('identity.signaturePlacementLabel')}</Label>
            <Select.Root value={signaturePlacement} onValueChange={(v) => { if (v) signaturePlacement = v as 'above' | 'below' }}>
              <Select.Trigger class="w-48 shrink-0">
                <Select.Value>
                  {signaturePlacement === 'below' ? $_('identity.belowQuotedText') : $_('identity.aboveQuotedText')}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                <Select.Item value="above" label={$_('identity.aboveQuotedText')} />
                <Select.Item value="below" label={$_('identity.belowQuotedText')} />
              </Select.Content>
            </Select.Root>
          </div>

          <!-- Separator Option -->
          <div class="flex items-center gap-2">
            <input
              type="checkbox"
              id="signatureSeparator"
              bind:checked={signatureSeparator}
              class="w-4 h-4 rounded border-input accent-primary"
            />
            <Label for="signatureSeparator" class="cursor-pointer text-sm">
              {$_('identity.addSeparator')} (<code class="text-xs bg-muted px-1 rounded">-- </code>)
            </Label>
          </div>
        {/if}
      </div>

      <!-- Error message -->
      {#if errors.general}
        <div class="flex items-start gap-2 p-3 rounded-lg bg-destructive/10 border border-destructive/20">
          <Icon icon="mdi:alert-circle" class="w-5 h-5 text-destructive flex-shrink-0 mt-0.5" />
          <p class="text-sm text-destructive">{errors.general}</p>
        </div>
      {/if}

      </div>

      <!-- Actions (fixed at the bottom of the dialog) -->
      <div class="flex items-center justify-end gap-2 pt-4 border-t border-border">
        <Button type="button" variant="ghost" onclick={handleCancel} disabled={saving}>
          {$_('common.cancel')}
        </Button>
        <Button type="submit" disabled={saving}>
          {#if saving}
            <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
          {/if}
          {identity ? $_('identity.saveIdentityChanges') : $_('identity.addEmailAddressButton')}
        </Button>
      </div>
    </form>
  </Dialog.Content>
</Dialog.Root>
