<script lang="ts">
  import * as Select from '$lib/components/ui/select'
  import BoolSelect from '$lib/components/ui/bool-select/BoolSelect.svelte'
  import { _ } from '$lib/i18n'
  import type { SettingsDraft } from '../settingsDraft.svelte'
  import SettingsPageHeader from '../shared/SettingsPageHeader.svelte'
  import SettingsSection from '../shared/SettingsSection.svelte'
  import SettingsRow from '../shared/SettingsRow.svelte'
  import { SETTINGS_SELECT_WIDTH_CLASS } from '../shared/settingsControlStyles'

  interface Props { draft: SettingsDraft }
  let { draft }: Props = $props()

  const themes = $derived([
    { value: 'pop-dark', label: $_('settingsGeneral.themeDark') },
    { value: 'light-blue', label: $_('settingsGeneral.themeLight') },
  ])
  const densities = $derived([
    { value: 'compact', label: $_('settingsGeneral.densityCompact') },
    { value: 'standard', label: $_('settingsGeneral.densityStandard') },
    { value: 'large', label: $_('settingsGeneral.densityLarge') },
  ])
  const themeLabel = $derived(themes.find(item => item.value === draft.themeMode)?.label ?? draft.themeMode)
  const densityLabel = $derived(densities.find(item => item.value === draft.messageListDensity)?.label ?? draft.messageListDensity)
</script>

<div class="space-y-6">
  <SettingsPageHeader description={$_('settingsDescriptions.appearance')} />
  <SettingsSection framed={false}>
    <SettingsRow label={$_('settingsGeneral.theme')}>
      <Select.Root value={draft.themeMode} onValueChange={(value) => value && (draft.themeMode = value)}>
        <Select.Trigger class={SETTINGS_SELECT_WIDTH_CLASS}><Select.Value>{themeLabel}</Select.Value></Select.Trigger>
        <Select.Content>{#each themes as option (option.value)}<Select.Item value={option.value} label={option.label} />{/each}</Select.Content>
      </Select.Root>
    </SettingsRow>
    <SettingsRow label={$_('settingsGeneral.darkMailContent')}>
      <BoolSelect bind:checked={draft.darkMailContent} disabled={draft.themeMode !== 'pop-dark'} class={SETTINGS_SELECT_WIDTH_CLASS} />
    </SettingsRow>
    <SettingsRow label={$_('settingsGeneral.accentBarUnread')}>
      <BoolSelect bind:checked={draft.accentBarUnread} class={SETTINGS_SELECT_WIDTH_CLASS} />
    </SettingsRow>
    <SettingsRow label={$_('settingsGeneral.messageListDensity')}>
      <Select.Root value={draft.messageListDensity} onValueChange={(value) => value && (draft.messageListDensity = value)}>
        <Select.Trigger class={SETTINGS_SELECT_WIDTH_CLASS}><Select.Value>{densityLabel}</Select.Value></Select.Trigger>
        <Select.Content>{#each densities as option (option.value)}<Select.Item value={option.value} label={option.label} />{/each}</Select.Content>
      </Select.Root>
    </SettingsRow>
  </SettingsSection>
</div>
