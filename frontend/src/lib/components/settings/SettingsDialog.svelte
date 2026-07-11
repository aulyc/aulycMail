<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '@iconify/svelte'
  import * as Dialog from '$lib/components/ui/dialog'
  import * as Tabs from '$lib/components/ui/tabs'
  import { Button } from '$lib/components/ui/button'
  // @ts-ignore - wailsjs path
  import { GetReadReceiptResponsePolicy, SetReadReceiptResponsePolicy, GetMarkAsReadDelay, SetMarkAsReadDelay, GetMessageListDensity, SetMessageListDensity, GetThemeMode, SetThemeMode, GetShowTitleBar, SetShowTitleBar, GetRunBackground, SetRunBackground, GetStartHidden, SetStartHidden, GetAutostart, SetAutostart, GetLanguage, SetLanguage, GetComposerFormat, SetComposerFormat, GetNativeTitleBar, SetNativeTitleBar, GetAlwaysLoadImages, SetAlwaysLoadImages, GetDarkMailContent, SetDarkMailContent, GetAccentBarUnread, SetAccentBarUnread, GetMenuBarIcon, SetMenuBarIcon, GetDeveloperMode, SetDeveloperMode, QuitApp } from '../../../../wailsjs/go/app/App.js'
  import { addToast } from '$lib/stores/toast'
  import { setMessageListDensity as updateDensityStore, setThemeMode as updateThemeStore, setLanguage as updateLanguageStore, setComposerFormat as updateComposerFormatStore, setAlwaysLoadImages as updateAlwaysLoadImagesStore, setDarkMailContent as updateDarkMailContentStore, setAccentBarUnread as updateAccentBarUnreadStore, setDeveloperMode as updateDeveloperModeStore, type MessageListDensity, type ThemeMode, type ComposerFormat } from '$lib/stores/settings.svelte'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import { _ } from '$lib/i18n'
  import ConfirmDialog from '$lib/components/ui/confirm-dialog/ConfirmDialog.svelte'
  import GeneralTab from './GeneralTab.svelte'
  import ImagesTab from './ImagesTab.svelte'
  import AccountsTab from './AccountsTab.svelte'

  interface Props {
    /** Whether the dialog is open */
    open?: boolean
    /** Callback when dialog should close */
    onClose?: () => void
  }

  let {
    open = $bindable(false),
    onClose,
  }: Props = $props()

  // Settings state
  let readReceiptResponsePolicy = $state<string>('ask')
  let markAsReadDelaySeconds = $state<number>(1) // Display in seconds, store in ms
  let messageListDensity = $state<string>('standard')
  let themeMode = $state<string>('system')
  let showTitleBar = $state<boolean>(true)
  let runBackground = $state<boolean>(false)
  let startHidden = $state<boolean>(false)
  let autostart = $state<boolean>(false)
  let language = $state<string>('')
  let composerFormat = $state<string>('rich')
  let nativeTitleBar = $state<boolean>(false)
  let alwaysLoadImages = $state<boolean>(false)
  let darkMailContent = $state<boolean>(false)
  let accentBarUnread = $state<boolean>(false)
  let menuBarIcon = $state<boolean>(false)
  let developerMode = $state<boolean>(false)
  let originalNativeTitleBar = false
  let showRestartDialog = $state(false)
  let loading = $state(true)
  let saving = $state(false)
  let activeTab = $state('general')
  const settingsTabListClass = 'grid w-full grid-cols-3 bg-muted-foreground/10 border border-muted-foreground/15 dark:bg-muted'
  const settingsTabTriggerClass = 'flex items-center gap-2 data-[state=inactive]:hover:bg-background/70 data-[state=inactive]:hover:text-foreground data-[state=active]:bg-primary/10 data-[state=active]:text-primary data-[state=active]:shadow-none data-[state=active]:ring-1 data-[state=active]:ring-primary/25 dark:data-[state=active]:bg-primary/15'

  // Load settings on mount
  onMount(async () => {
    await loadSettings()
  })

  // Also load when dialog opens
  $effect(() => {
    if (open) {
      loadSettings()
    }
  })

  // Activate the dialog guard while open: suppresses background refreshes
  // and routes global keyboard shortcuts (e.g. Ctrl+A) to the dialog inputs
  // instead of the message list / viewer behind it.
  $effect(() => {
    if (open) {
      dialogGuardOpen()
      return () => dialogGuardClose()
    }
  })

  async function loadSettings() {
    loading = true
    try {
      const [policy, delayMs, density, theme, titleBar, runBg, startHid, autoSt, lang, compFmt, nativeTB, alwaysImages, darkMail, accentBar, menuBar, devMode] = await Promise.all([
        GetReadReceiptResponsePolicy(),
        GetMarkAsReadDelay(),
        GetMessageListDensity(),
        GetThemeMode(),
        GetShowTitleBar(),
        GetRunBackground(),
        GetStartHidden(),
        GetAutostart(),
        GetLanguage(),
        GetComposerFormat(),
        GetNativeTitleBar(),
        GetAlwaysLoadImages(),
        GetDarkMailContent(),
        GetAccentBarUnread(),
        GetMenuBarIcon(),
        GetDeveloperMode(),
      ])
      readReceiptResponsePolicy = policy
      // Convert ms to seconds for display
      markAsReadDelaySeconds = delayMs < 0 ? -1 : delayMs / 1000
      messageListDensity = density
      // Only Dark (pop-dark) and Light (light-blue) remain; coerce any legacy
      // value (system / yaru-dark) to Dark so the dropdown shows a valid option.
      themeMode = (theme === 'pop-dark' || theme === 'light-blue') ? theme : 'pop-dark'
      showTitleBar = titleBar
      runBackground = runBg
      startHidden = startHid
      autostart = autoSt
      language = lang
      composerFormat = compFmt || 'rich'
      nativeTitleBar = nativeTB ?? false
      alwaysLoadImages = alwaysImages ?? false
      darkMailContent = darkMail ?? false
      accentBarUnread = accentBar ?? false
      menuBarIcon = menuBar ?? false
      developerMode = devMode ?? false
      originalNativeTitleBar = nativeTitleBar
    } catch (err) {
      console.error('Failed to load settings:', err)
    } finally {
      loading = false
    }
  }

  async function handleSave() {
    saving = true
    try {
      // Convert seconds to ms for storage
      const delayMs = markAsReadDelaySeconds < 0 ? -1 : Math.round(markAsReadDelaySeconds * 1000)
      const effectiveRunBackground = menuBarIcon ? true : runBackground

      // Save settings sequentially to avoid SQLite lock conflicts
      await SetReadReceiptResponsePolicy(readReceiptResponsePolicy)
      await SetMarkAsReadDelay(delayMs)
      await SetMessageListDensity(messageListDensity)
      await SetThemeMode(themeMode)
      await SetShowTitleBar(showTitleBar)
      await SetRunBackground(effectiveRunBackground)
      await SetStartHidden(startHidden)
      await SetAutostart(autostart)
      if (language) {
        await SetLanguage(language)
      }
      await SetComposerFormat(composerFormat)
      await SetNativeTitleBar(nativeTitleBar)
      await SetAlwaysLoadImages(alwaysLoadImages)
      await SetDarkMailContent(darkMailContent)
      await SetAccentBarUnread(accentBarUnread)
      await SetMenuBarIcon(menuBarIcon)
      await SetDeveloperMode(developerMode)
      // Update the reactive stores so UI updates immediately
      updateDensityStore(messageListDensity as MessageListDensity)
      updateThemeStore(themeMode as ThemeMode)
      runBackground = effectiveRunBackground
      if (language) {
        updateLanguageStore(language)
      }
      updateComposerFormatStore(composerFormat as ComposerFormat)
      updateAlwaysLoadImagesStore(alwaysLoadImages)
      updateDarkMailContentStore(darkMailContent)
      updateAccentBarUnreadStore(accentBarUnread)
      updateDeveloperModeStore(developerMode)
      addToast({
        type: 'success',
        message: $_('toast.settingsSaved'),
      })
      // Show restart dialog if native title bar setting changed
      if (nativeTitleBar !== originalNativeTitleBar) {
        originalNativeTitleBar = nativeTitleBar
        showRestartDialog = true
        return
      }
      open = false
      onClose?.()
    } catch (err) {
      console.error('Failed to save settings:', err)
      addToast({
        type: 'error',
        message: $_('toast.failedToSaveSettings'),
      })
    } finally {
      saving = false
    }
  }

  function handleCancel() {
    open = false
    onClose?.()
  }

  function handleOpenChange(isOpen: boolean) {
    open = isOpen
    if (!isOpen) {
      onClose?.()
    }
  }
</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
  <Dialog.Content class="max-w-2xl" preventCloseAutoFocus onInteractOutside={(e) => e.preventDefault()}>
    <Dialog.Header>
      <Dialog.Title>{$_('settings.title')}</Dialog.Title>
    </Dialog.Header>

    {#if loading}
      <div class="flex items-center justify-center py-8">
        <Icon icon="mdi:loading" class="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    {:else}
      <Tabs.Root bind:value={activeTab} class="w-full">
        <Tabs.List class={settingsTabListClass}>
          <Tabs.Trigger value="general" class={settingsTabTriggerClass}>
            <span class="inline-flex w-4 h-4 items-center justify-center shrink-0"><Icon icon="lucide:settings-2" width="16" height="16" /></span>
            {$_('settings.general')}
          </Tabs.Trigger>
          <Tabs.Trigger value="images" class={settingsTabTriggerClass}>
            <span class="inline-flex w-4 h-4 items-center justify-center shrink-0"><Icon icon="lucide:image" width="16" height="16" /></span>
            {$_('settings.images')}
          </Tabs.Trigger>
          <Tabs.Trigger value="accounts" class={settingsTabTriggerClass}>
            <span class="inline-flex w-4 h-4 items-center justify-center shrink-0"><Icon icon="lucide:mails" width="16" height="16" /></span>
            {$_('settings.accounts')}
          </Tabs.Trigger>
        </Tabs.List>

        <div class="mt-4 h-[390px] overflow-y-auto pl-1 pr-3">
          <Tabs.Content value="general" class="mt-0">
            <GeneralTab
              bind:messageListDensity
              bind:themeMode
              bind:runBackground
              bind:autostart
              bind:language
              onDensityChange={(v) => messageListDensity = v}
              onThemeChange={(v) => themeMode = v}
              onRunBackgroundChange={(v) => { runBackground = v }}
              onAutostartChange={(v) => autostart = v}
              onLanguageChange={(v) => language = v}
              bind:accentBarUnread
              bind:menuBarIcon
              bind:developerMode
              bind:darkMailContent
              bind:composerFormat
              bind:readReceiptResponsePolicy
              onFormatChange={(v) => composerFormat = v}
              onPolicyChange={(v) => readReceiptResponsePolicy = v}
            />
          </Tabs.Content>

          <Tabs.Content value="images" class="mt-0">
            <ImagesTab
              bind:alwaysLoadImages
              onAlwaysLoadImagesChange={(v) => alwaysLoadImages = v}
            />
          </Tabs.Content>

          <Tabs.Content value="accounts" class="mt-0">
            <AccountsTab />
          </Tabs.Content>
        </div>
      </Tabs.Root>

      <!-- Actions - SettingsDialog owns the footer so every tab uses the same chrome. -->
      <div class="flex items-center justify-end gap-2 border-t border-border pt-4">
        {#if activeTab === 'general' || activeTab === 'images'}
          <Button variant="ghost" onclick={handleCancel} disabled={saving}>
            {$_('common.cancel')}
          </Button>
          <Button onclick={handleSave} disabled={saving}>
            {#if saving}
              <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
            {/if}
            {$_('common.save')}
          </Button>
        {:else}
          <Button variant="ghost" onclick={handleCancel}>
            {$_('common.close')}
          </Button>
        {/if}
      </div>
    {/if}
  </Dialog.Content>
</Dialog.Root>

<ConfirmDialog
  bind:open={showRestartDialog}
  title={$_('settingsGeneral.restartRequired')}
  description={$_('settingsGeneral.restartRequiredDescription')}
  confirmLabel={$_('settingsGeneral.quitNow')}
  cancelLabel={$_('settingsGeneral.restartLater')}
  onConfirm={() => QuitApp()}
  onCancel={() => { showRestartDialog = false; open = false; onClose?.() }}
/>
