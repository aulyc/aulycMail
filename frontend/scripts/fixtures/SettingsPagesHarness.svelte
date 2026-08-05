<script lang="ts">
  import { SettingsDraft } from '$lib/components/settings/settingsDraft.svelte'
  import AppearanceSettingsPage from '$lib/components/settings/pages/AppearanceSettingsPage.svelte'
  import GeneralSettingsPage from '$lib/components/settings/pages/GeneralSettingsPage.svelte'
  import MailSettingsPage from '$lib/components/settings/pages/MailSettingsPage.svelte'

  let { page }: { page: 'general' | 'appearance' | 'mail' } = $props()
  const draft = new SettingsDraft()
  draft.language = 'en'
  draft.runBackground = true
  draft.menuBarIcon = true
  draft.autostart = false
  draft.enhancedKeyboardNavigation = true
  draft.developerMode = false
  draft.themeMode = 'pop-dark'
  draft.darkMailContent = true
  draft.accentBarUnread = false
  draft.messageListDensity = 'standard'
  draft.composerFormat = 'rich'
  draft.readReceiptResponsePolicy = 'ask'
  draft.alwaysLoadImages = false
</script>

{#if page === 'general'}
  <GeneralSettingsPage {draft} />
{:else if page === 'appearance'}
  <AppearanceSettingsPage {draft} />
{:else}
  <MailSettingsPage {draft} />
{/if}

<output
  data-settings-draft
  data-language={draft.language}
  data-run-background={draft.runBackground}
  data-menu-bar-icon={draft.menuBarIcon}
  data-autostart={draft.autostart}
  data-enhanced-keyboard-navigation={draft.enhancedKeyboardNavigation}
  data-developer-mode={draft.developerMode}
  data-theme-mode={draft.themeMode}
  data-dark-mail-content={draft.darkMailContent}
  data-accent-bar-unread={draft.accentBarUnread}
  data-density={draft.messageListDensity}
  data-composer-format={draft.composerFormat}
  data-receipt-policy={draft.readReceiptResponsePolicy}
></output>
