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
    nativeTitleBar: boolean
    showTitleBar: boolean
    runBackground: boolean
    autostart: boolean
    language: string
    onDensityChange: (value: string) => void
    onThemeChange: (value: string) => void
    onTitleBarChange: (nativeTitleBar: boolean, showTitleBar: boolean) => void
    onRunBackgroundChange: (value: boolean) => void
    onAutostartChange: (value: boolean) => void
    onLanguageChange: (value: string) => void
    accentBarUnread: boolean
    darkMailContent: boolean
  }

  let {
    messageListDensity = $bindable(),
    themeMode = $bindable(),
    nativeTitleBar = $bindable(),
    showTitleBar = $bindable(),
    runBackground = $bindable(),
    autostart = $bindable(),
    language = $bindable(),
    onDensityChange,
    onThemeChange,
    onTitleBarChange,
    onRunBackgroundChange,
    onAutostartChange,
    onLanguageChange,
    accentBarUnread = $bindable(),
    darkMailContent = $bindable(),
  }: Props = $props()

  // Message list density options
  const densityOptions = $derived([
    { value: 'micro', label: $_('settingsGeneral.densityMicro') },
    { value: 'compact', label: $_('settingsGeneral.densityCompact') },
    { value: 'standard', label: $_('settingsGeneral.densityStandard') },
    { value: 'large', label: $_('settingsGeneral.densityLarge') },
  ])

  // Title bar options
  const titleBarOptions = $derived([
    { value: 'aulycmail', label: $_('settingsGeneral.titleBaraulycmail'), description: $_('settingsGeneral.titleBaraulycmailDesc') },
    { value: 'native', label: $_('settingsGeneral.titleBarNative'), description: $_('settingsGeneral.titleBarNativeDesc') },
    { value: 'disable', label: $_('settingsGeneral.titleBarDisable'), description: $_('settingsGeneral.titleBarDisableDesc') },
  ])

  const titleBarValue = $derived(
    nativeTitleBar ? 'native' : showTitleBar ? 'aulycmail' : 'disable'
  )

  // Theme mode options
  const themeModeOptions = $derived([
    { value: 'system', label: $_('settingsGeneral.themeSystem') },
    { value: 'yaru-dark', label: 'Yaru (Dark)' },
    { value: 'pop-dark', label: 'Pop! (Dark)' },
    { value: 'light-blue', label: $_('settingsGeneral.themeLightBlue') },
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

  function handleTitleBarChange(value: string) {
    switch (value) {
      case 'aulycmail':
        nativeTitleBar = false
        showTitleBar = true
        break
      case 'native':
        nativeTitleBar = true
        showTitleBar = false
        break
      case 'disable':
        nativeTitleBar = false
        showTitleBar = false
        break
    }
    onTitleBarChange?.(nativeTitleBar, showTitleBar)
  }

  function getTitleBarLabel(value: string): string {
    return titleBarOptions.find(opt => opt.value === value)?.label || value
  }

  function handleRunBackgroundChange(value: boolean) {
    runBackground = value
    onRunBackgroundChange?.(value)
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
  <!-- Title bar -->
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label>{$_('settingsGeneral.titleBar')}</Label>
      <p class="text-xs text-muted-foreground">{$_('settingsGeneral.titleBarHelp')}</p>
    </div>
    <Select.Root value={titleBarValue} onValueChange={handleTitleBarChange}>
      <Select.Trigger class="w-48 shrink-0">
        <Select.Value>{getTitleBarLabel(titleBarValue)}</Select.Value>
      </Select.Trigger>
      <Select.Content>
        {#each titleBarOptions as opt (opt.value)}
          <Select.Item value={opt.value} label={opt.label} />
        {/each}
      </Select.Content>
    </Select.Root>
  </div>

  <!-- Language -->
  <div class="flex items-center justify-between gap-4">
    <Label>{$_('settingsGeneral.language')}</Label>
    <Select.Root value={language || 'en'} onValueChange={handleLanguageChange}>
      <Select.Trigger class="w-48 shrink-0">
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
      <p class="text-xs text-muted-foreground">{$_('settingsGeneral.themeHelp')}</p>
    </div>
    <Select.Root value={themeMode} onValueChange={handleThemeChange}>
      <Select.Trigger class="w-48 shrink-0">
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
        <p class="text-xs text-muted-foreground">
          {$_('settingsGeneral.darkMailContentHelp')}
        </p>
      </div>
      <BoolSelect id="dark-mail-content" bind:checked={darkMailContent} />
    </div>
  {/if}

  <!-- Accent bar for unread messages -->
  <div class="flex items-center justify-between gap-3">
    <div>
      <Label for="accent-bar-unread">{$_('settingsGeneral.accentBarUnread')}</Label>
      <p class="text-xs text-muted-foreground">
        {$_('settingsGeneral.accentBarUnreadHelp')}
      </p>
    </div>
    <BoolSelect id="accent-bar-unread" bind:checked={accentBarUnread} />
  </div>

  <!-- Message list density -->
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <Label>{$_('settingsGeneral.messageListDensity')}</Label>
      <p class="text-xs text-muted-foreground">{$_('settingsGeneral.messageListDensityHelp')}</p>
    </div>
    <Select.Root value={messageListDensity} onValueChange={handleDensityChange}>
      <Select.Trigger class="w-48 shrink-0">
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
      <p class="text-xs text-muted-foreground">
        {$_('settingsGeneral.runInBackgroundHelp')}
      </p>
    </div>
    <BoolSelect id="run-background" bind:checked={runBackground} onCheckedChange={handleRunBackgroundChange} />
  </div>

  <!-- Autostart on login -->
  <div class="flex items-center justify-between gap-3">
    <div class="space-y-0.5">
      <Label for="autostart">{$_('settingsGeneral.autostartOnLogin')}</Label>
      <p class="text-xs text-muted-foreground">
        {$_('settingsGeneral.autostartHelp')}
      </p>
    </div>
    <BoolSelect id="autostart" bind:checked={autostart} onCheckedChange={handleAutostartChange} />
  </div>
</div>
