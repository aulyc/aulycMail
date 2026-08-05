<script lang="ts">
  import Icon from '@iconify/svelte'
  import type { app } from '../../../../wailsjs/go/models'
  import { BrowserOpenURL } from '../../../../wailsjs/runtime/runtime'
  import { _ } from '$lib/i18n'
  import appLogo from '$/assets/images/logo-universal.png'
  import AboutInfoDialog from './AboutInfoDialog.svelte'
  import UpdateAction from './UpdateAction.svelte'
  import { getAboutInfoContent, type InfoKind } from './aboutInfoContent'

  interface Props { appInfo: app.AppInfo | null; loading?: boolean }
  let { appInfo, loading = false }: Props = $props()

  let infoOpen = $state(false)
  let infoKind = $state<InfoKind>('product')

  function openWebsite() {
    if (appInfo?.website) {
      BrowserOpenURL(appInfo.website)
    }
  }

  function openInfo(kind: InfoKind) {
    infoKind = kind
    infoOpen = true
  }

  const infoContent = $derived(getAboutInfoContent(infoKind, $_))
</script>

<div class="flex h-full flex-col items-center justify-center space-y-6 py-6">
  {#if loading}
    <Icon icon="mdi:loading" class="w-8 h-8 animate-spin text-muted-foreground" />
  {:else if appInfo}
    <!-- Logo + App Name & Version -->
    <div class="flex flex-col items-center space-y-2">
      <img
        src={appLogo}
        alt={`${appInfo.name} Logo`}
        class="h-24 w-24 rounded-[22%] shadow-sm ring-1 ring-border/60"
        draggable="false"
      />
      <div class="text-center space-y-1">
        <h2 class="text-2xl font-bold text-foreground">{appInfo.name}</h2>
        <p class="text-sm text-muted-foreground">{$_('settingsAbout.version', { values: { version: appInfo.displayVersion } })}</p>
      </div>
    </div>

    <!-- Links -->
    <div class="flex w-max flex-col items-stretch gap-2">
      <UpdateAction />
      <button
        type="button"
        onclick={() => openInfo('product')}
        class="flex w-full items-center justify-start gap-2 text-left text-sm text-primary transition-colors hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
      >
        <Icon icon="lucide:book-open-text" class="h-5 w-5 shrink-0" />
        <span>{$_('settingsAbout.productDescription')}</span>
      </button>
      <button
        type="button"
        onclick={openWebsite}
        class="flex w-full items-center justify-start gap-2 text-left text-sm text-primary transition-colors hover:underline"
      >
        <Icon icon="mdi:web" class="h-5 w-5 shrink-0" />
        <span>{$_('settingsAbout.website')}</span>
      </button>
      <button
        type="button"
        onclick={() => openInfo('privacy')}
        class="flex w-full items-center justify-start gap-2 text-left text-sm text-primary transition-colors hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
      >
        <Icon icon="mdi:shield-account" class="h-5 w-5 shrink-0" />
        <span>{$_('settingsAbout.privacyPolicy')}</span>
      </button>
      <button
        type="button"
        onclick={() => openInfo('terms')}
        class="flex w-full items-center justify-start gap-2 text-left text-sm text-primary transition-colors hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
      >
        <Icon icon="mdi:file-document" class="h-5 w-5 shrink-0" />
        <span>{$_('settingsAbout.termsOfUse')}</span>
      </button>
      <button
        type="button"
        onclick={() => openInfo('acknowledgements')}
        class="flex w-full items-center justify-start gap-2 text-left text-sm text-primary transition-colors hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
      >
        <Icon icon="lucide:heart-handshake" class="h-5 w-5 shrink-0" />
        <span>{$_('settingsAbout.acknowledgementsLabel')}</span>
      </button>
    </div>

  {:else}
    <p class="text-muted-foreground">{$_('settingsAbout.failedToLoad')}</p>
  {/if}
</div>

<AboutInfoDialog bind:open={infoOpen} title={infoContent.title} intro={infoContent.intro} sections={infoContent.sections} />
