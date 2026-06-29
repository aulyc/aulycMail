<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import * as Select from '$lib/components/ui/select'
  import { Button } from '$lib/components/ui/button'
  import {
    syncPeriodOptions,
  } from '$lib/config/providers'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { account } from '../../../../../wailsjs/go/models'

  interface Props {
    /** The account being edited */
    editAccount: account.Account
    /** Bound form values */
    displayName: string
    email: string
    username: string
    password: string
    syncPeriodDays: string
    /** Auth type from account */
    authType: string
    /** Whether the editing account uses the generic provider (controls
     *  whether to hint about the separate SMTP credentials UI on the
     *  Server tab). */
    isGenericProvider: boolean
    /** Validation errors */
    errors: Record<string, string>
    /** Whether re-authorization is in progress */
    reauthorizing?: boolean
    /** Whether re-authorization succeeded */
    reauthorizeSuccess?: boolean
    /** Callbacks */
    onDisplayNameChange: (value: string) => void
    onUsernameChange: (value: string) => void
    onPasswordChange: (value: string) => void
    onSyncPeriodChange: (value: string) => void
    onReauthorize?: () => void
  }

  let {
    editAccount: _editAccount,
    displayName = $bindable(),
    email = $bindable(),
    username = $bindable(),
    password = $bindable(),
    syncPeriodDays = $bindable(),
    authType,
    isGenericProvider,
    errors,
    reauthorizing = false,
    reauthorizeSuccess = false,
    onDisplayNameChange,
    onUsernameChange,
    onPasswordChange,
    onSyncPeriodChange,
    onReauthorize,
  }: Props = $props()

  function getSyncPeriodLabel(value: string): string {
    const numValue = Number(value)
    const option = syncPeriodOptions.find(opt => opt.value === numValue)
    return option ? $_(option.labelKey) : `${value} days`
  }
</script>

<div class="space-y-4">
  <!-- Default display name -->
  <div>
    <div class="flex items-center justify-between gap-4">
      <div class="min-w-0">
        <Label for="displayName">{$_('account.displayName')}</Label>
        <p class="text-xs text-muted-foreground">{$_('account.displayNameHelp')}</p>
      </div>
      <Input
        id="displayName"
        type="text"
        placeholder={$_('account.displayNamePlaceholder')}
        bind:value={displayName}
        oninput={(e) => onDisplayNameChange((e.target as HTMLInputElement).value)}
        class="w-64 shrink-0 {errors.displayName ? 'border-destructive' : ''}"
      />
    </div>
    {#if errors.displayName}
      <p class="text-sm text-destructive mt-1">{errors.displayName}</p>
    {/if}
  </div>

  <!-- Email address (read-only) -->
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label for="email">{$_('account.emailAddress')}</Label>
      <p class="text-xs text-muted-foreground">{$_('account.emailReadOnly')}</p>
    </div>
    <Input id="email" type="email" value={email} disabled class="w-64 shrink-0 bg-muted" />
  </div>

  <!-- Username -->
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label for="username">{$_('account.username')}</Label>
      <p class="text-xs text-muted-foreground">{$_('account.usernameHelp')}</p>
    </div>
    <Input
      id="username"
      type="text"
      placeholder={$_('account.usernamePlaceholder')}
      bind:value={username}
      oninput={(e) => onUsernameChange((e.target as HTMLInputElement).value)}
      class="w-64 shrink-0"
    />
  </div>

  {#if authType === 'oauth2'}
    <!-- OAuth account -->
    <div class="flex items-center justify-between gap-4">
      <div class="min-w-0">
        <Label>{$_('account.authentication')}</Label>
        <p class="text-xs text-muted-foreground">
          {reauthorizeSuccess ? $_('account.oauthFreshToken') : $_('account.reauthorizeHelp')}
        </p>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        {#if reauthorizeSuccess}
          <span class="text-xs text-green-500 flex items-center gap-1">
            <Icon icon="mdi:check-circle" class="w-4 h-4" />
            {$_('account.oauthReauthorized')}
          </span>
        {:else}
          <span class="text-xs text-muted-foreground flex items-center gap-1">
            <Icon icon="mdi:shield-check" class="w-4 h-4 text-primary" />
            {$_('account.oauthConnected')}
          </span>
          <Button variant="outline" size="sm" onclick={onReauthorize} disabled={reauthorizing}>
            {#if reauthorizing}
              <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
              {$_('account.authorizing')}
            {:else}
              <Icon icon="mdi:refresh" class="w-4 h-4 mr-2" />
              {$_('account.reauthorize')}
            {/if}
          </Button>
        {/if}
      </div>
    </div>
  {:else}
    <!-- Password account -->
    <div>
      <div class="flex items-center justify-between gap-4">
        <div class="min-w-0">
          <Label for="password">{$_('account.password')}</Label>
          {#if isGenericProvider}
            <p class="text-xs text-muted-foreground">{$_('account.smtpCredsNote')}</p>
          {/if}
        </div>
        <Input
          id="password"
          type="password"
          placeholder={$_('account.leaveEmptyToKeep')}
          bind:value={password}
          oninput={(e) => onPasswordChange((e.target as HTMLInputElement).value)}
          class="w-64 shrink-0 {errors.password ? 'border-destructive' : ''}"
        />
      </div>
      {#if errors.password}
        <p class="text-sm text-destructive mt-1">{errors.password}</p>
      {/if}
    </div>
  {/if}

  <!-- Sync period -->
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label>{$_('account.syncPeriod')}</Label>
      <p class="text-xs text-muted-foreground">{$_('account.syncPeriodHelp')}</p>
    </div>
    <Select.Root
      value={syncPeriodDays}
      onValueChange={(v) => { syncPeriodDays = v; onSyncPeriodChange(v) }}
    >
      <Select.Trigger class="w-48 shrink-0">
        <Select.Value placeholder="Select">
          {getSyncPeriodLabel(syncPeriodDays)}
        </Select.Value>
      </Select.Trigger>
      <Select.Content>
        {#each syncPeriodOptions as opt (opt.value)}
          <Select.Item value={String(opt.value)} label={$_(opt.labelKey)} />
        {/each}
      </Select.Content>
    </Select.Root>
  </div>
</div>
