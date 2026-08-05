<script lang="ts">
  import Icon from '@iconify/svelte'
  import type { app } from '../../../../wailsjs/go/models'
  import { BrowserOpenURL } from '../../../../wailsjs/runtime/runtime'
  import { _ } from '$lib/i18n'
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
  const versionValue = $derived(
    appInfo?.buildNumber && appInfo.buildNumber !== '0'
      ? `${appInfo.version} · build ${appInfo.buildNumber}`
      : (appInfo?.version ?? ''),
  )
</script>

<div class="mx-auto h-full w-full max-w-4xl">
  {#if loading}
    <div class="flex h-full min-h-72 items-center justify-center">
      <Icon icon="mdi:loading" class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>
  {:else if appInfo}
    <article class="flex h-full min-h-0 flex-col" data-about-document>
      <header data-about-fixed-header class="shrink-0 pb-5 pr-12">
        <h2 class="text-2xl font-bold tracking-tight text-foreground">
          {$_('settingsAbout.title', { values: { name: appInfo.name } })}
        </h2>
      </header>

      <div data-about-scroll-region class="min-h-0 flex-1 overflow-y-auto pr-2 scrollbar-thin">
        <div class="space-y-8 pb-6">
          <dl class="grid grid-cols-[7rem_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm" data-about-section="metadata">
            <dt class="font-semibold text-foreground">{$_('settingsAbout.versionTitle')}</dt>
            <dd class="text-muted-foreground">{versionValue}</dd>
            <dt class="font-semibold text-foreground">{$_('settingsAbout.compatibilityTitle')}</dt>
            <dd class="text-muted-foreground">{$_('settingsAbout.compatibilityValue')}</dd>
            <dt class="font-semibold text-foreground">{$_('settingsAbout.systemRequirementTitle')}</dt>
            <dd class="text-muted-foreground">{$_('settingsAbout.systemRequirementValue')}</dd>
          </dl>

          <section class="space-y-3" data-about-section="introduction">
            <h3 class="text-base font-semibold text-foreground">{$_('settingsAbout.applicationIntroduction')}</h3>
            <ol class="list-decimal space-y-2 pl-5 text-sm leading-6 text-muted-foreground">
              <li>{$_('settingsAbout.product.intro')}</li>
              <li>{$_('settingsAbout.product.positionBody')}</li>
              <li>{$_('settingsAbout.product.dataBody')}</li>
            </ol>
          </section>

          <section class="space-y-2" data-about-section="website">
            <h3 class="text-base font-semibold text-foreground">{$_('settingsAbout.website')}</h3>
            <button
              type="button"
              data-settings-focus-style="link"
              disabled={!appInfo.website}
              onclick={openWebsite}
              class="-mx-2 inline-flex min-h-7 w-fit items-center rounded-md px-2 text-left text-sm text-primary transition-colors hover:bg-primary/5 hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 disabled:cursor-default disabled:text-muted-foreground disabled:no-underline"
            >
              <span>{appInfo.website || '—'}</span>
            </button>
          </section>

          <section class="space-y-2" data-about-section="related-links">
            <h3 class="text-base font-semibold text-foreground">{$_('settingsAbout.relatedLinks')}</h3>
            <div class="flex flex-col items-start gap-0.5">
              <UpdateAction />
              <button
                type="button"
                data-settings-focus-style="link"
                onclick={() => openInfo('product')}
                class="-mx-2 inline-flex min-h-7 w-fit items-center rounded-md px-2 text-left text-sm text-primary transition-colors hover:bg-primary/5 hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
              >
                {$_('settingsAbout.productDescription')}
              </button>
              <button
                type="button"
                data-settings-focus-style="link"
                onclick={() => openInfo('privacy')}
                class="-mx-2 inline-flex min-h-7 w-fit items-center rounded-md px-2 text-left text-sm text-primary transition-colors hover:bg-primary/5 hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
              >
                {$_('settingsAbout.privacyPolicy')}
              </button>
              <button
                type="button"
                data-settings-focus-style="link"
                onclick={() => openInfo('terms')}
                class="-mx-2 inline-flex min-h-7 w-fit items-center rounded-md px-2 text-left text-sm text-primary transition-colors hover:bg-primary/5 hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
              >
                {$_('settingsAbout.termsOfUse')}
              </button>
              <button
                type="button"
                data-settings-focus-style="link"
                onclick={() => openInfo('acknowledgements')}
                class="-mx-2 inline-flex min-h-7 w-fit items-center rounded-md px-2 text-left text-sm text-primary transition-colors hover:bg-primary/5 hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
              >
                {$_('settingsAbout.acknowledgementsLabel')}
              </button>
            </div>
          </section>
        </div>
      </div>

      <footer class="shrink-0 border-t border-border pt-4 text-xs text-muted-foreground" data-about-section="copyright" data-about-fixed-footer>
        {$_('settingsAbout.copyright')}
      </footer>
    </article>
  {:else}
    <div class="flex h-full min-h-72 items-center justify-center">
      <p class="text-muted-foreground">{$_('settingsAbout.failedToLoad')}</p>
    </div>
  {/if}
</div>

<AboutInfoDialog bind:open={infoOpen} title={infoContent.title} intro={infoContent.intro} sections={infoContent.sections} />
