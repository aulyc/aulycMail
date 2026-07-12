<script lang="ts">
  import * as Select from '$lib/components/ui/select'
  import { _ } from '$lib/i18n'
  import type { SettingsDraft } from '../settingsDraft.svelte'
  import ImagesTab from '../ImagesTab.svelte'
  import SettingsPageHeader from '../shared/SettingsPageHeader.svelte'
  import SettingsSection from '../shared/SettingsSection.svelte'
  import SettingsRow from '../shared/SettingsRow.svelte'
  import { SETTINGS_SELECT_WIDTH_CLASS } from '../shared/settingsControlStyles'

  interface Props { draft: SettingsDraft }
  let { draft }: Props = $props()

  const formats = $derived([
    { value: 'rich', label: $_('settings.composerFormatRich') },
    { value: 'plain', label: $_('settings.composerFormatPlain') },
  ])
  const policies = $derived([
    { value: 'never', label: $_('settingsGeneral.neverSendReceipts') },
    { value: 'ask', label: $_('settingsGeneral.askEachTime') },
    { value: 'always', label: $_('settingsGeneral.alwaysSendReceipts') },
  ])
  const formatLabel = $derived(formats.find(item => item.value === draft.composerFormat)?.label ?? draft.composerFormat)
  const policyLabel = $derived(policies.find(item => item.value === draft.readReceiptResponsePolicy)?.label ?? draft.readReceiptResponsePolicy)
</script>

<div class="space-y-6">
  <SettingsPageHeader description={$_('settingsDescriptions.mail')} />
  <SettingsSection framed={false}>
    <SettingsRow label={$_('settings.composerFormat')}>
      <Select.Root value={draft.composerFormat} onValueChange={(value) => value && (draft.composerFormat = value)}>
        <Select.Trigger class={SETTINGS_SELECT_WIDTH_CLASS}><Select.Value>{formatLabel}</Select.Value></Select.Trigger>
        <Select.Content>{#each formats as option (option.value)}<Select.Item value={option.value} label={option.label} />{/each}</Select.Content>
      </Select.Root>
    </SettingsRow>
    <SettingsRow label={$_('settingsGeneral.readReceiptPolicy')}>
      <Select.Root value={draft.readReceiptResponsePolicy} onValueChange={(value) => value && (draft.readReceiptResponsePolicy = value)}>
        <Select.Trigger class={SETTINGS_SELECT_WIDTH_CLASS}><Select.Value>{policyLabel}</Select.Value></Select.Trigger>
        <Select.Content>{#each policies as option (option.value)}<Select.Item value={option.value} label={option.label} />{/each}</Select.Content>
      </Select.Root>
    </SettingsRow>
    <ImagesTab bind:alwaysLoadImages={draft.alwaysLoadImages} onAlwaysLoadImagesChange={(value) => draft.alwaysLoadImages = value} />
  </SettingsSection>
</div>
