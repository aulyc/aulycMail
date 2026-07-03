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
    /** Live-linked name for the account's default identity */
    linkedName?: string
    /** Callback when dialog should close */
    onClose?: () => void
    /** Callback when the display name changes */
    onNameChange?: (value: string) => void
    /** Callback when identity is saved */
    onSave?: (config: account.IdentityConfig) => Promise<void>
  }

  type SignatureMode = 'none' | 'html' | 'plain'
  type SignatureSeparatorOption = 'none' | 'dash' | 'asterisk'

  let {
    open = $bindable(false),
    identity = null,
    accountId: _accountId,
    linkedName,
    onClose,
    onNameChange,
    onSave,
  }: Props = $props()

  // Form state
  let email = $state('')
  let name = $state('')
  let signatureHtml = $state('')
  let signatureText = $state('')
  let signatureMode = $state<SignatureMode>('html')
  let signatureForNew = $state(true)
  let signatureForReply = $state(true)
  let signatureForForward = $state(true)
  let signatureSeparator = $state<SignatureSeparatorOption>('none')

  let saving = $state(false)
  let errors = $state<Record<string, string>>({})

  // Initialize form when identity changes
  $effect(() => {
    if (open) {
      if (identity) {
        // Editing existing identity
        email = identity.email || ''
        name = identity.name || ''
        signatureHtml = identity.signatureHtml || ''
        signatureText = identity.signatureText || ''
        signatureMode = identity.signatureEnabled === false
          ? 'none'
          : (signatureHtml || !signatureText ? 'html' : 'plain')
        signatureForNew = identity.signatureForNew ?? true
        signatureForReply = identity.signatureForReply ?? true
        signatureForForward = identity.signatureForForward ?? true
        signatureSeparator = getSeparatorOption(identity.signatureSeparatorStyle, identity.signatureSeparator)
      } else {
        // New identity - reset to defaults
        email = ''
        name = ''
        signatureHtml = ''
        signatureText = ''
        signatureMode = 'html'
        signatureForNew = true
        signatureForReply = true
        signatureForForward = true
        signatureSeparator = 'none'
      }
      errors = {}
    }
  })

  $effect(() => {
    if (open && identity?.isDefault && linkedName !== undefined && name !== linkedName) {
      name = linkedName
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
        signatureHtml: signatureMode === 'html' ? signatureHtml : '',
        signatureText: signatureMode === 'plain' ? signatureText : '',
        signatureEnabled: signatureMode !== 'none',
        signatureForNew,
        signatureForReply,
        signatureForForward,
        signaturePlacement: 'above',
        signatureSeparator: signatureSeparator !== 'none',
        signatureSeparatorStyle: getSeparatorStyle(signatureSeparator),
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

  function handleNameInput(value: string) {
    name = value
    if (identity?.isDefault) {
      onNameChange?.(value)
    }
  }

  function getSignatureStatusLabel(mode: SignatureMode): string {
    if (mode === 'none') return $_('identity.signatureStatusDisabled')
    if (mode === 'plain') return $_('identity.signatureStatusPlain')
    return $_('identity.signatureStatusHtml')
  }

  function getSeparatorOption(style: string | undefined, legacySeparator: boolean | undefined): SignatureSeparatorOption {
    if (style === '*****') return 'asterisk'
    if (style === '-----' || legacySeparator) return 'dash'
    return 'none'
  }

  function getSeparatorStyle(option: SignatureSeparatorOption): string {
    if (option === 'dash') return '-----'
    if (option === 'asterisk') return '*****'
    return ''
  }

  function getSeparatorLabel(option: SignatureSeparatorOption): string {
    if (option === 'dash') return '-----'
    if (option === 'asterisk') return '*****'
    return $_('identity.signatureSeparatorDisabled')
  }

  function getDialogTitle(): string {
    if (!identity) return $_('identity.addEmailTitle')
    return `${email || identity.email} ${$_('identity.editEmailTitle')}`
  }

</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
  <Dialog.Content class="max-w-2xl max-h-[90vh] overflow-hidden flex flex-col">
    <Dialog.Header>
      <Dialog.Title>
        {getDialogTitle()}
      </Dialog.Title>
    </Dialog.Header>

    <form onsubmit={(e) => { e.preventDefault(); handleSave() }} class="flex-1 min-h-0 flex flex-col overflow-hidden">
      <!-- Fixed-height scroll area: keeps the dialog height constant whether or
           not the signature section is shown. pr-3/pl-1/pt-1.5 keep the
           scrollbar + focus rings off the inputs. -->
      <div class="h-[460px] max-h-[calc(90vh-200px)] overflow-y-auto space-y-5 pt-1.5 pl-1 pr-3 pb-2">
      <!-- Email & Name (label + input on one row) -->
      <div class="space-y-4">
        {#if !identity}
          <div>
            <div class="flex items-center justify-between gap-4">
              <div class="min-w-0">
                <Label for="email">{$_('identity.emailAddressLabel')}</Label>
              </div>
              <Input
                id="email"
                type="email"
                placeholder="you@example.com"
                bind:value={email}
                class="w-64 shrink-0 {errors.email ? 'border-destructive' : ''}"
              />
            </div>
            {#if errors.email}
              <p class="text-sm text-destructive mt-1">{errors.email}</p>
            {/if}
          </div>
        {/if}

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
              oninput={(e) => handleNameInput((e.target as HTMLInputElement).value)}
              class="w-64 shrink-0 {errors.name ? 'border-destructive' : ''}"
            />
          </div>
          {#if errors.name}
            <p class="text-sm text-destructive mt-1">{errors.name}</p>
          {/if}
        </div>
      </div>

      <!-- Signature Section -->
      <div class="space-y-5">
        <div class="flex items-center justify-between gap-4">
          <Label class="font-medium">{$_('identity.signatureStatus')}</Label>
          <Select.Root
            value={signatureMode}
            onValueChange={(v) => {
              if (v === 'none' || v === 'html' || v === 'plain') {
                signatureMode = v
              }
            }}
          >
            <Select.Trigger class="w-64 shrink-0">
              <Select.Value>
                {getSignatureStatusLabel(signatureMode)}
              </Select.Value>
            </Select.Trigger>
            <Select.Content>
              <Select.Item value="none" label={$_('identity.signatureStatusDisabled')} />
              <Select.Item value="html" label={$_('identity.signatureStatusHtml')} />
              <Select.Item value="plain" label={$_('identity.signatureStatusPlain')} />
            </Select.Content>
          </Select.Root>
        </div>

        {#if signatureMode !== 'none'}
          {#if signatureMode === 'html'}
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
          {:else}
            <!-- Plain Text Signature -->
            <div class="space-y-2">
              <Label for="signatureText">{$_('identity.plainTextSignature')}</Label>
              <textarea
                id="signatureText"
                bind:value={signatureText}
                rows="4"
                placeholder="Plain text version for text-only emails..."
                class="w-full p-3 text-sm bg-background border border-input rounded-md resize-none overflow-y-auto focus:outline-none focus:ring-2 focus:ring-ring font-mono"
              ></textarea>
            </div>
          {/if}

          <!-- Signature behavior and separator share one grid so row spacing stays consistent. -->
          <div class="grid grid-cols-[minmax(0,1fr)_16rem] items-center gap-x-4 gap-y-2">
            <Label class="font-medium">{$_('identity.appendSignatureTo')}</Label>
            <div class="contents">
              <label class="flex h-10 items-center gap-2 cursor-pointer rounded-md border border-input bg-background px-3 hover:bg-muted/50 transition-colors">
                <input
                  type="checkbox"
                  bind:checked={signatureForNew}
                  class="w-4 h-4 rounded border-input accent-primary"
                />
                <span class="text-sm">{$_('identity.newMessages')}</span>
              </label>
              <div></div>
              <label class="flex h-10 items-center gap-2 cursor-pointer rounded-md border border-input bg-background px-3 hover:bg-muted/50 transition-colors">
                <input
                  type="checkbox"
                  bind:checked={signatureForReply}
                  class="w-4 h-4 rounded border-input accent-primary"
                />
                <span class="text-sm">{$_('identity.replies')}</span>
              </label>
              <div></div>
              <label class="flex h-10 items-center gap-2 cursor-pointer rounded-md border border-input bg-background px-3 hover:bg-muted/50 transition-colors">
                <input
                  type="checkbox"
                  bind:checked={signatureForForward}
                  class="w-4 h-4 rounded border-input accent-primary"
                />
                <span class="text-sm">{$_('identity.forwards')}</span>
              </label>
            </div>
            <Label class="font-medium">{$_('identity.signatureSeparatorLabel')}</Label>
            <Select.Root
              value={signatureSeparator}
              onValueChange={(v) => {
                if (v === 'none' || v === 'dash' || v === 'asterisk') {
                  signatureSeparator = v
                }
              }}
            >
              <Select.Trigger class="w-64 h-10 shrink-0">
                <Select.Value>
                  {getSeparatorLabel(signatureSeparator)}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                <Select.Item value="none" label={$_('identity.signatureSeparatorDisabled')} />
                <Select.Item value="dash" label="-----" />
                <Select.Item value="asterisk" label="*****" />
              </Select.Content>
            </Select.Root>
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
      <div class="flex items-center justify-end gap-2 pt-4">
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
