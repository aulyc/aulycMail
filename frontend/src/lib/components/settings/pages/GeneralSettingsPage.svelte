<script lang="ts">
  import * as Select from '$lib/components/ui/select'
  import BoolSelect from '$lib/components/ui/bool-select/BoolSelect.svelte'
  import { _, supportedLocales } from '$lib/i18n'
  import type { SettingsDraft } from '../settingsDraft.svelte'
  import SettingsPageHeader from '../shared/SettingsPageHeader.svelte'
  import SettingsSection from '../shared/SettingsSection.svelte'
  import SettingsRow from '../shared/SettingsRow.svelte'
  import { SETTINGS_SELECT_WIDTH_CLASS } from '../shared/settingsControlStyles'

  interface Props { draft: SettingsDraft }
  let { draft }: Props = $props()

  function languageLabel(code: string) {
    return supportedLocales.find(item => item.code === code)?.name ?? code ?? 'English'
  }

  function setRunBackground(value: boolean) {
    draft.runBackground = value
    if (!value) draft.menuBarIcon = false
  }

  function setMenuBarIcon(value: boolean) {
    draft.menuBarIcon = value
    if (value) draft.runBackground = true
  }
</script>

<div class="space-y-6">
  <SettingsPageHeader description={$_('settingsDescriptions.general')} />
  <SettingsSection framed={false}>
    <SettingsRow label={$_('settingsGeneral.language')}>
      <Select.Root value={draft.language || 'en'} onValueChange={(value) => value && (draft.language = value)}>
        <Select.Trigger class={SETTINGS_SELECT_WIDTH_CLASS}><Select.Value>{languageLabel(draft.language || 'en')}</Select.Value></Select.Trigger>
        <Select.Content>{#each supportedLocales as locale (locale.code)}<Select.Item value={locale.code} label={locale.name} />{/each}</Select.Content>
      </Select.Root>
    </SettingsRow>
    <SettingsRow label={$_('settingsGeneral.runInBackground')}>
      <BoolSelect bind:checked={draft.runBackground} onCheckedChange={setRunBackground} class={SETTINGS_SELECT_WIDTH_CLASS} />
    </SettingsRow>
    <SettingsRow label={$_('settingsGeneral.menuBarIcon')}>
      <BoolSelect bind:checked={draft.menuBarIcon} onCheckedChange={setMenuBarIcon} class={SETTINGS_SELECT_WIDTH_CLASS} />
    </SettingsRow>
    <SettingsRow label={$_('settingsGeneral.autostartOnLogin')}>
      <BoolSelect bind:checked={draft.autostart} class={SETTINGS_SELECT_WIDTH_CLASS} />
    </SettingsRow>
    <SettingsRow label={$_('settingsGeneral.developerMode')}>
      <BoolSelect bind:checked={draft.developerMode} class={SETTINGS_SELECT_WIDTH_CLASS} />
    </SettingsRow>
  </SettingsSection>
</div>
