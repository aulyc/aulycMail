<script lang="ts">
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import * as Select from '$lib/components/ui/select'
  import {
    syncPeriodOptions,
    syncIntervalOptions,
  } from '$lib/config/providers'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { account, app } from '../../../../../wailsjs/go/models'

  interface Props {
    /** The account being edited */
    editAccount: account.Account
    /** Bound form values */
    displayName: string
    email: string
    username: string
    password: string
    noOutgoingServer: boolean
    replyForwardIdentityID: string
    availableIdentityGroups: app.AccountIdentityGroup[]
    syncPeriodDays: string
    syncInterval: string
    readReceiptRequestPolicy: string
    /** Validation errors */
    errors: Record<string, string>
    /** Callbacks */
    onDisplayNameChange: (value: string) => void
    onUsernameChange: (value: string) => void
    onPasswordChange: (value: string) => void
    onNoOutgoingServerChange: (value: boolean) => void
    onReplyForwardIdentityIDChange: (value: string) => void
    onSyncPeriodChange: (value: string) => void
    onSyncIntervalChange: (value: string) => void
    onReadReceiptPolicyChange: (value: string) => void
  }

  let {
    editAccount: _editAccount,
    displayName = $bindable(),
    email = $bindable(),
    username = $bindable(),
    password = $bindable(),
    noOutgoingServer = $bindable(false),
    replyForwardIdentityID = $bindable(''),
    availableIdentityGroups = [],
    syncPeriodDays = $bindable(),
    syncInterval = $bindable(),
    readReceiptRequestPolicy = $bindable(),
    errors,
    onDisplayNameChange,
    onUsernameChange,
    onPasswordChange,
    onNoOutgoingServerChange,
    onReplyForwardIdentityIDChange,
    onSyncPeriodChange,
    onSyncIntervalChange,
    onReadReceiptPolicyChange,
  }: Props = $props()

  const selectTriggerClass = 'w-64 shrink-0'
  const readReceiptRequestOptions = [
    { value: 'never', labelKey: 'account.neverRequest' },
    { value: 'ask', labelKey: 'account.askEachTime' },
    { value: 'always', labelKey: 'account.alwaysRequest' },
  ]

  function getSyncPeriodLabel(value: string): string {
    const numValue = Number(value)
    const option = syncPeriodOptions.find(opt => opt.value === numValue)
    return option ? $_(option.labelKey) : `${value} days`
  }

  function getSyncIntervalLabel(value: string): string {
    const numValue = Number(value)
    const option = syncIntervalOptions.find(opt => opt.value === numValue)
    return option ? $_(option.labelKey) : `${value} min`
  }

  function getReadReceiptLabel(value: string): string {
    switch (value) {
      case 'never': return $_('account.neverRequest')
      case 'ask': return $_('account.askEachTime')
      case 'always': return $_('account.alwaysRequest')
      default: return value
    }
  }

  function getSendingLabel(value: string): string {
    return value === 'disabled'
      ? $_('account.mailSendingDisabled')
      : $_('account.mailSendingEnabled')
  }
</script>

<div class="space-y-4">
  <!-- Default display name -->
  <div>
    <div class="flex items-center justify-between gap-4">
      <div class="min-w-0">
        <Label for="displayName">{$_('account.displayName')}</Label>
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
    </div>
    <Input id="email" type="email" value={email} disabled class="w-64 shrink-0 bg-muted" />
  </div>

  <!-- Username -->
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label for="username">{$_('account.username')}</Label>
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

  <!-- Password -->
  <div>
    <div class="flex items-center justify-between gap-4">
      <div class="min-w-0">
        <Label for="password">{$_('account.password')}</Label>
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

  <!-- Mail sending -->
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label>{$_('account.mailSending')}</Label>
    </div>
    <Select.Root
      value={noOutgoingServer ? 'disabled' : 'enabled'}
      onValueChange={(v) => {
        const disabled = v === 'disabled'
        noOutgoingServer = disabled
        onNoOutgoingServerChange(disabled)
      }}
    >
      <Select.Trigger class={selectTriggerClass}>
        <Select.Value placeholder="Select">
          {getSendingLabel(noOutgoingServer ? 'disabled' : 'enabled')}
        </Select.Value>
      </Select.Trigger>
      <Select.Content>
        <Select.Item value="enabled" label={$_('account.mailSendingEnabled')} />
        <Select.Item value="disabled" label={$_('account.mailSendingDisabled')} />
      </Select.Content>
    </Select.Root>
  </div>

  {#if noOutgoingServer}
    <!-- Reply/Forward-with identity picker. Default = empty value, which the
         composer resolves to the user's default sending identity at compose time. -->
    <div class="space-y-1">
      <Label>{$_('account.replyForwardWith')}</Label>
      <Select.Root
        value={replyForwardIdentityID}
        onValueChange={(v) => {
          replyForwardIdentityID = v
          onReplyForwardIdentityIDChange(v)
        }}
      >
        <Select.Trigger class="h-10">
          <Select.Value placeholder={$_('account.replyForwardWithDefault')}>
            {#if replyForwardIdentityID}
              {@const allIdentities = (availableIdentityGroups || []).flatMap(g => (g.identities || []).map(i => ({ identity: i, group: g })))}
              {@const found = allIdentities.find(x => x.identity.id === replyForwardIdentityID)}
              {#if found}
                {#if found.group.account?.color}
                  <span class="inline-block w-2 h-2 rounded-full mr-1.5 flex-shrink-0" style="background-color: {found.group.account.color}"></span>
                {/if}
                {found.identity.name} &lt;{found.identity.email}&gt;
              {:else}
                {$_('account.replyForwardWithDefault')}
              {/if}
            {:else}
              {$_('account.replyForwardWithDefault')}
            {/if}
          </Select.Value>
        </Select.Trigger>
        <Select.Content>
          <Select.Item value="" label={$_('account.replyForwardWithDefault')} />
          {#each availableIdentityGroups || [] as group (group.account?.id)}
            <Select.Group>
              <Select.GroupHeading class="flex items-center gap-1.5 px-2 py-1 text-xs font-medium text-muted-foreground">
                {#if group.account?.color}
                  <span class="inline-block w-2 h-2 rounded-full flex-shrink-0" style="background-color: {group.account.color}"></span>
                {/if}
                {group.account?.name || group.account?.email}
              </Select.GroupHeading>
              {#each group.identities || [] as identity (identity.id)}
                <Select.Item value={identity.id} label="{identity.name} <{identity.email}>" />
              {/each}
            </Select.Group>
          {/each}
        </Select.Content>
      </Select.Root>
    </div>
  {/if}

  <!-- Sync period -->
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label>{$_('account.syncPeriod')}</Label>
    </div>
    <Select.Root
      value={syncPeriodDays}
      onValueChange={(v) => { syncPeriodDays = v; onSyncPeriodChange(v) }}
    >
      <Select.Trigger class={selectTriggerClass}>
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

  <!-- Check new mail -->
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label>{$_('account.checkNewMail')}</Label>
    </div>
    <Select.Root
      value={syncInterval}
      onValueChange={(v) => { syncInterval = v; onSyncIntervalChange(v) }}
    >
      <Select.Trigger class={selectTriggerClass}>
        <Select.Value placeholder="Select">
          {getSyncIntervalLabel(syncInterval)}
        </Select.Value>
      </Select.Trigger>
      <Select.Content>
        {#each syncIntervalOptions as opt (opt.value)}
          <Select.Item value={String(opt.value)} label={$_(opt.labelKey)} />
        {/each}
      </Select.Content>
    </Select.Root>
  </div>

  <!-- Read receipt requests -->
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label>{$_('account.requestReadReceipts')}</Label>
    </div>
    <Select.Root
      value={readReceiptRequestPolicy}
      onValueChange={(v) => { readReceiptRequestPolicy = v; onReadReceiptPolicyChange(v) }}
    >
      <Select.Trigger class={selectTriggerClass}>
        <Select.Value placeholder="Select">
          {getReadReceiptLabel(readReceiptRequestPolicy)}
        </Select.Value>
      </Select.Trigger>
      <Select.Content>
        {#each readReceiptRequestOptions as opt (opt.value)}
          <Select.Item value={opt.value} label={$_(opt.labelKey)} />
        {/each}
      </Select.Content>
    </Select.Root>
  </div>
</div>
