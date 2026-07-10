<script lang="ts">
  import * as Select from '$lib/components/ui/select'
  import { Label } from '$lib/components/ui/label'
  import BoolSelect from '$lib/components/ui/bool-select/BoolSelect.svelte'
  import { _, setLocale } from '$lib/i18n'
  import { supportedLocales } from '$lib/i18n'
  import { getIsDarkActive } from '$lib/stores/theme.svelte'

  interface Props {
    messageListDensity: string
    themeMode: string
    runBackground: boolean
    autostart: boolean
    language: string
    onDensityChange: (value: string) => void
    onThemeChange: (value: string) => void
    onRunBackgroundChange: (value: boolean) => void
    onAutostartChange: (value: boolean) => void
    onLanguageChange: (value: string) => void
    accentBarUnread: boolean
    menuBarIcon: boolean
    developerMode: boolean
    darkMailContent: boolean
    composerFormat: string
    readReceiptResponsePolicy: string
    onFormatChange: (value: string) => void
    onPolicyChange: (value: string) => void
  }

  let {
    messageListDensity = $bindable(),
    themeMode = $bindable(),
    runBackground = $bindable(),
    autostart = $bindable(),
    language = $bindable(),
    onDensityChange,
    onThemeChange,
    onRunBackgroundChange,
    onAutostartChange,
    onLanguageChange,
    accentBarUnread = $bindable(),
    menuBarIcon = $bindable(),
    developerMode = $bindable(),
    darkMailContent = $bindable(),
    composerFormat = $bindable(),
    readReceiptResponsePolicy = $bindable(),
    onFormatChange,
    onPolicyChange,
  }: Props = $props()

  // Default composer format options
  const formatOptions = $derived([
    { value: 'rich', label: $_('settings.composerFormatRich') },
    { value: 'plain', label: $_('settings.composerFormatPlain') },
  ])

  // Read-receipt response policy options
  const readReceiptResponseOptions = $derived([
    { value: 'never', label: $_('settingsGeneral.neverSendReceipts') },
    { value: 'ask', label: $_('settingsGeneral.askEachTime') },
    { value: 'always', label: $_('settingsGeneral.alwaysSendReceipts') },
  ])

  function getFormatLabel(value: string): string {
    return formatOptions.find(o => o.value === value)?.label ?? value
  }

  function getPolicyLabel(value: string): string {
    return readReceiptResponseOptions.find(o => o.value === value)?.label ?? value
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

  // 小 (compact) / 中 (standard) / 大 (large) — the old 极小 (micro) is removed
  const densityOptions = $derived([
    { value: 'compact', label: $_('settingsGeneral.densityCompact') },
    { value: 'standard', label: $_('settingsGeneral.densityStandard') },
    { value: 'large', label: $_('settingsGeneral.densityLarge') },
  ])

  // Theme mode options — just dark (Pop!) and light.
  const themeModeOptions = $derived([
    { value: 'pop-dark', label: $_('settingsGeneral.themeDark') },
    { value: 'light-blue', label: $_('settingsGeneral.themeLight') },
  ])

  function getDensityLabel(value: string): string {
    return densityOptions.find(opt => opt.value === value)?.label || value
  }

  function getThemeModeLabel(value: string): string {
    return themeModeOptions.find(opt => opt.value === value)?.label || value
  }

  function getLanguageLabel(code: string): string {
    return supportedLocales.find(l => l.code === code)?.name || code || 'English'
  }

  function handleDensityChange(value: string) {
    messageListDensity = value
    onDensityChange?.(value)
  }

  function handleThemeChange(value: string) {
    themeMode = value
    onThemeChange?.(value)
  }


  function handleRunBackgroundChange(value: boolean) {
    runBackground = value
    if (!value && menuBarIcon) {
      menuBarIcon = false
    }
    onRunBackgroundChange?.(value)
  }

  function handleMenuBarIconChange(value: boolean) {
    menuBarIcon = value
    if (value && !runBackground) {
      runBackground = true
      onRunBackgroundChange?.(true)
    }
  }

  function handleAutostartChange(value: boolean) {
    autostart = value
    onAutostartChange?.(value)
  }

  function handleLanguageChange(value: string) {
    language = value
    setLocale(value)
    onLanguageChange?.(value)
  }
</script>

<div class="space-y-4">
  <!-- Language -->
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label>{$_('settingsGeneral.language')}</Label>
    </div>
    <Select.Root value={language || 'en'} onValueChange={handleLanguageChange}>
      <Select.Trigger class="w-36 shrink-0">
        <Select.Value>{getLanguageLabel(language || 'en')}</Select.Value>
      </Select.Trigger>
      <Select.Content>
        {#each supportedLocales as loc (loc.code)}
          <Select.Item value={loc.code} label={loc.name} />
        {/each}
      </Select.Content>
    </Select.Root>
  </div>

  <!-- Theme -->
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label>{$_('settingsGeneral.theme')}</Label>
    </div>
    <Select.Root value={themeMode} onValueChange={handleThemeChange}>
      <Select.Trigger class="w-36 shrink-0">
        <Select.Value placeholder={$_('settingsGeneral.selectTheme')}>
          {getThemeModeLabel(themeMode)}
        </Select.Value>
      </Select.Trigger>
      <Select.Content>
        {#each themeModeOptions as opt (opt.value)}
          <Select.Item value={opt.value} label={opt.label} />
        {/each}
      </Select.Content>
    </Select.Root>
  </div>

  <!-- Dark mail content — only relevant when a dark theme is active -->
  {#if getIsDarkActive()}
    <div class="flex items-center justify-between gap-3">
      <div>
        <Label for="dark-mail-content">{$_('settingsGeneral.darkMailContent')}</Label>
      </div>
      <BoolSelect id="dark-mail-content" bind:checked={darkMailContent} class="w-36" />
    </div>
  {/if}

  <!-- Accent bar for unread messages -->
  <div class="flex items-center justify-between gap-3">
    <div>
      <Label for="accent-bar-unread">{$_('settingsGeneral.accentBarUnread')}</Label>
    </div>
    <BoolSelect id="accent-bar-unread" bind:checked={accentBarUnread} class="w-36" />
  </div>

  <!-- Message list density -->
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label>{$_('settingsGeneral.messageListDensity')}</Label>
    </div>
    <Select.Root value={messageListDensity} onValueChange={handleDensityChange}>
      <Select.Trigger class="w-36 shrink-0">
        <Select.Value placeholder={$_('settingsGeneral.selectDensity')}>
          {getDensityLabel(messageListDensity)}
        </Select.Value>
      </Select.Trigger>
      <Select.Content>
        {#each densityOptions as opt (opt.value)}
          <Select.Item value={opt.value} label={opt.label} />
        {/each}
      </Select.Content>
    </Select.Root>
  </div>

  <!-- Run in background -->
  <div class="flex items-center justify-between gap-3">
    <div class="space-y-0.5">
      <Label for="run-background">{$_('settingsGeneral.runInBackground')}</Label>
    </div>
    <BoolSelect id="run-background" bind:checked={runBackground} onCheckedChange={handleRunBackgroundChange} class="w-36" />
  </div>

  <!-- Menu bar icon -->
  <div class="flex items-center justify-between gap-3">
    <div class="space-y-0.5">
      <Label for="menu-bar-icon">{$_('settingsGeneral.menuBarIcon')}</Label>
    </div>
    <BoolSelect id="menu-bar-icon" bind:checked={menuBarIcon} onCheckedChange={handleMenuBarIconChange} class="w-36" />
  </div>

  <!-- Autostart on login -->
  <div class="flex items-center justify-between gap-3">
    <div class="space-y-0.5">
      <Label for="autostart">{$_('settingsGeneral.autostartOnLogin')}</Label>
    </div>
    <BoolSelect id="autostart" bind:checked={autostart} onCheckedChange={handleAutostartChange} class="w-36" />
  </div>

  <!-- Developer mode -->
  <div class="flex items-center justify-between gap-3">
    <div class="space-y-0.5">
      <Label for="developer-mode">{$_('settingsGeneral.developerMode')}</Label>
    </div>
    <BoolSelect id="developer-mode" bind:checked={developerMode} class="w-36" />
  </div>

  <!-- Default composer format (moved here from the removed "Composer" tab) -->
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label>{$_('settings.composerFormat')}</Label>
    </div>
    <Select.Root value={composerFormat} onValueChange={handleFormatChange}>
      <Select.Trigger class="w-36 shrink-0">
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

  <!-- Read-receipt response policy (moved here from the removed "Composer" tab) -->
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label>{$_('settingsGeneral.readReceiptPolicy')}</Label>
    </div>
    <Select.Root value={readReceiptResponsePolicy} onValueChange={handlePolicyChange}>
      <Select.Trigger class="w-36 shrink-0">
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
