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
    darkMailContent: boolean
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
    darkMailContent = $bindable(),
  }: Props = $props()

  // Message list density options
  const densityOptions = $derived([
    { value: 'micro', label: $_('settingsGeneral.densityMicro') },
    { value: 'compact', label: $_('settingsGeneral.densityCompact') },
    { value: 'standard', label: $_('settingsGeneral.densityStandard') },
    { value: 'large', label: $_('settingsGeneral.densityLarge') },
  ])

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
