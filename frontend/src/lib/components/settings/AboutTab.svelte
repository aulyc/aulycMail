<script lang="ts">
  import Icon from '@iconify/svelte'
  import type { app } from '../../../../wailsjs/go/models'
  import { BrowserOpenURL } from '../../../../wailsjs/runtime/runtime'
  import { _ } from '$lib/i18n'
  import AboutInfoDialog from './AboutInfoDialog.svelte'

  interface Props { appInfo: app.AppInfo | null; loading?: boolean }
  let { appInfo, loading = false }: Props = $props()

  type InfoKind = 'product' | 'privacy' | 'terms' | 'acknowledgements'
  interface InfoSection { title: string; body?: string; items?: string[] }
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

  function openInfoOnPointerDown(event: PointerEvent, kind: InfoKind) {
    if (event.button !== 0) return
    event.preventDefault()
    event.stopPropagation()
    openInfo(kind)
  }

  function openInfoOnKeyboardClick(event: MouseEvent, kind: InfoKind) {
    if (event.detail === 0) openInfo(kind)
  }

  const infoContent = $derived.by((): { title: string; intro: string; sections: InfoSection[] } => {
    if (infoKind === 'product') return {
      title: $_('settingsAbout.product.title'),
      intro: $_('settingsAbout.product.intro'),
      sections: [
        { title: $_('settingsAbout.product.positionTitle'), body: $_('settingsAbout.product.positionBody') },
        { title: $_('settingsAbout.product.featuresTitle'), items: [$_('settingsAbout.product.featureAccounts'), $_('settingsAbout.product.featureMail'), $_('settingsAbout.product.featureSearch'), $_('settingsAbout.product.featureContacts'), $_('settingsAbout.product.featurePrivacy'), $_('settingsAbout.product.featureBackup')] },
        { title: $_('settingsAbout.product.dataTitle'), body: $_('settingsAbout.product.dataBody') },
      ],
    }
    if (infoKind === 'privacy') return {
      title: $_('settingsAbout.privacy.title'),
      intro: $_('settingsAbout.privacy.intro'),
      sections: [
        { title: $_('settingsAbout.privacy.noCollectionTitle'), items: [$_('settingsAbout.privacy.noCollectionPersonal'), $_('settingsAbout.privacy.noCollectionMail'), $_('settingsAbout.privacy.noCollectionTracking'), $_('settingsAbout.privacy.noCollectionAds'), $_('settingsAbout.privacy.noCollectionSale')] },
        { title: $_('settingsAbout.privacy.localTitle'), items: [$_('settingsAbout.privacy.localMail'), $_('settingsAbout.privacy.localAccount'), $_('settingsAbout.privacy.localContacts'), $_('settingsAbout.privacy.localSettings'), $_('settingsAbout.privacy.localLogs'), $_('settingsAbout.privacy.localBackups')] },
        { title: $_('settingsAbout.privacy.securityTitle'), body: $_('settingsAbout.privacy.securityBody') },
        { title: $_('settingsAbout.privacy.retentionTitle'), body: $_('settingsAbout.privacy.retentionBody') },
        { title: $_('settingsAbout.privacy.contactTitle'), body: $_('settingsAbout.privacy.contactBody') },
      ],
    }
    if (infoKind === 'terms') return {
      title: $_('settingsAbout.terms.title'),
      intro: $_('settingsAbout.terms.intro'),
      sections: [
        { title: $_('settingsAbout.terms.descriptionTitle'), body: $_('settingsAbout.terms.descriptionBody') },
        { title: $_('settingsAbout.terms.responsibilitiesTitle'), items: [$_('settingsAbout.terms.responsibilityCredentials'), $_('settingsAbout.terms.responsibilityDevice'), $_('settingsAbout.terms.responsibilityLaw'), $_('settingsAbout.terms.responsibilityProvider'), $_('settingsAbout.terms.responsibilityBackup')] },
        { title: $_('settingsAbout.terms.useTitle'), body: $_('settingsAbout.terms.useBody') },
        { title: $_('settingsAbout.terms.disclaimerTitle'), body: $_('settingsAbout.terms.disclaimerBody') },
        { title: $_('settingsAbout.terms.thirdPartyTitle'), body: $_('settingsAbout.terms.thirdPartyBody') },
        { title: $_('settingsAbout.terms.contactTitle'), body: $_('settingsAbout.terms.contactBody') },
      ],
    }
    return {
      title: $_('settingsAbout.acknowledgements.title'),
      intro: $_('settingsAbout.acknowledgements.intro'),
      sections: [
        { title: $_('settingsAbout.acknowledgements.technologyTitle'), items: [$_('settingsAbout.acknowledgements.technologyDesktop'), $_('settingsAbout.acknowledgements.technologyEditor'), $_('settingsAbout.acknowledgements.technologyInterface'), $_('settingsAbout.acknowledgements.technologyData')] },
        { title: $_('settingsAbout.acknowledgements.communityTitle'), body: $_('settingsAbout.acknowledgements.communityBody') },
        { title: $_('settingsAbout.acknowledgements.licenseTitle'), body: $_('settingsAbout.acknowledgements.licenseBody') },
      ],
    }
  })
</script>

<div class="flex h-full flex-col items-center justify-center space-y-6 py-6">
  {#if loading}
    <Icon icon="mdi:loading" class="w-8 h-8 animate-spin text-muted-foreground" />
  {:else if appInfo}
    <!-- Logo + App Name & Version -->
    <div class="flex flex-col items-center space-y-2">
      <Icon icon="lucide:mail" class="h-24 w-24 text-muted-foreground" aria-label={`${appInfo.name} Logo`} />
      <div class="text-center space-y-1">
        <h2 class="text-2xl font-bold text-foreground">{appInfo.name}</h2>
        <p class="text-sm text-muted-foreground">{$_('settingsAbout.version', { values: { version: appInfo.version } })}</p>
      </div>
    </div>

    <!-- Description -->
    <p class="text-center text-sm text-muted-foreground max-w-xs">
      {appInfo.description}
    </p>

    <!-- Links -->
    <div class="flex flex-col items-center gap-2">
      <button
        type="button"
        onpointerdown={(event) => openInfoOnPointerDown(event, 'product')}
        onclick={(event) => openInfoOnKeyboardClick(event, 'product')}
        class="flex items-center gap-2 text-sm text-primary hover:underline transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
      >
        <Icon icon="lucide:book-open-text" class="w-5 h-5" />
        <span>{$_('settingsAbout.productDescription')}</span>
      </button>
      <button
        type="button"
        onclick={openWebsite}
        class="flex items-center gap-2 text-sm text-primary hover:underline transition-colors"
      >
        <Icon icon="mdi:web" class="w-5 h-5" />
        <span>{$_('settingsAbout.website')}</span>
      </button>
      <button
        type="button"
        onpointerdown={(event) => openInfoOnPointerDown(event, 'privacy')}
        onclick={(event) => openInfoOnKeyboardClick(event, 'privacy')}
        class="flex items-center gap-2 text-sm text-primary hover:underline transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
      >
        <Icon icon="mdi:shield-account" class="w-5 h-5" />
        <span>{$_('settingsAbout.privacyPolicy')}</span>
      </button>
      <button
        type="button"
        onpointerdown={(event) => openInfoOnPointerDown(event, 'terms')}
        onclick={(event) => openInfoOnKeyboardClick(event, 'terms')}
        class="flex items-center gap-2 text-sm text-primary hover:underline transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
      >
        <Icon icon="mdi:file-document" class="w-5 h-5" />
        <span>{$_('settingsAbout.termsOfUse')}</span>
      </button>
      <button
        type="button"
        onpointerdown={(event) => openInfoOnPointerDown(event, 'acknowledgements')}
        onclick={(event) => openInfoOnKeyboardClick(event, 'acknowledgements')}
        class="flex items-center gap-2 text-sm text-primary hover:underline transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
      >
        <Icon icon="lucide:heart-handshake" class="w-5 h-5" />
        <span>{$_('settingsAbout.acknowledgementsLabel')}</span>
      </button>
    </div>

  {:else}
    <p class="text-muted-foreground">{$_('settingsAbout.failedToLoad')}</p>
  {/if}
</div>

<AboutInfoDialog bind:open={infoOpen} title={infoContent.title} intro={infoContent.intro} sections={infoContent.sections} />
