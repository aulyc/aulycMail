<script lang="ts">
  import Icon from '@iconify/svelte'
  import * as Select from '$lib/components/ui/select'
  import { Label } from '$lib/components/ui/label'
  import { _ } from '$lib/i18n'

  interface Props {
    composerFormat: string
    readReceiptResponsePolicy: string
    onFormatChange: (value: string) => void
    onPolicyChange: (value: string) => void
  }

  let {
    composerFormat = $bindable(),
    readReceiptResponsePolicy = $bindable(),
    onFormatChange,
    onPolicyChange,
  }: Props = $props()

  const formatOptions = $derived([
    { value: 'rich', label: $_('settings.composerFormatRich') },
    { value: 'plain', label: $_('settings.composerFormatPlain') },
  ])

  const readReceiptResponseOptions = $derived([
    { value: 'never', label: $_('settingsGeneral.neverSendReceipts') },
    { value: 'ask', label: $_('settingsGeneral.askEachTime') },
    { value: 'always', label: $_('settingsGeneral.alwaysSendReceipts') },
  ])

  function getPolicyLabel(value: string): string {
    return readReceiptResponseOptions.find(o => o.value === value)?.label ?? value
  }

  function getFormatLabel(value: string): string {
    return formatOptions.find(o => o.value === value)?.label ?? value
  }

  function handleFormatChange(value: string | undefined) {
    if (!value) return
    composerFormat = value
    onFormatChange?.(value)
  }

  function handlePolicyChange(value: string | undefined) {
    if (!value) return
    readReceiptResponsePolicy = value
    onPolicyChange?.(value)
  }
</script>

<div class="space-y-6 p-1">
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label>{$_('settings.composerFormat')}</Label>
      <p class="text-xs text-muted-foreground">{$_('settings.composerFormatDescription')}</p>
    </div>
    <Select.Root value={composerFormat} onValueChange={handleFormatChange}>
      <Select.Trigger class="w-48 shrink-0">
        <Select.Value placeholder={$_('settings.composerFormat')}>
          {getFormatLabel(composerFormat)}
        </Select.Value>
      </Select.Trigger>
      <Select.Content>
        {#each formatOptions as opt (opt.value)}
          <Select.Item value={opt.value} label={opt.label} />
        {/each}
      </Select.Content>
    </Select.Root>
  </div>

  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label>{$_('settingsGeneral.readReceiptPolicy')}</Label>
      <p class="text-xs text-muted-foreground">{$_('settingsGeneral.readReceiptPolicyHelp')}</p>
    </div>
    <Select.Root value={readReceiptResponsePolicy} onValueChange={handlePolicyChange}>
      <Select.Trigger class="w-48 shrink-0">
        <Select.Value placeholder={$_('settingsGeneral.selectPolicy')}>
          {getPolicyLabel(readReceiptResponsePolicy)}
        </Select.Value>
      </Select.Trigger>
      <Select.Content>
        {#each readReceiptResponseOptions as opt (opt.value)}
          <Select.Item value={opt.value} label={opt.label} />
        {/each}
      </Select.Content>
    </Select.Root>
  </div>
</div>
