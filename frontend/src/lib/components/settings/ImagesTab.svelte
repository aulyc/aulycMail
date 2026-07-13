<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import BoolSelect from '$lib/components/ui/bool-select/BoolSelect.svelte'
  import ConfirmDialog from '$lib/components/ui/confirm-dialog/ConfirmDialog.svelte'
  import { addToast } from '$lib/stores/toast'
  import { _ } from '$lib/i18n'
  import { refreshImageAllowlist } from '$lib/stores/imageAllowlist.svelte'
  import SettingsRow from './shared/SettingsRow.svelte'
  import { SETTINGS_SELECT_WIDTH_CLASS } from './shared/settingsControlStyles'
  // @ts-ignore - wailsjs path
  import { settings } from '../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import {
    GetImageAllowlist,
    RemoveImageAllowlist,
  } from '../../../../wailsjs/go/app/App'

  interface Props {
    alwaysLoadImages: boolean
    onAlwaysLoadImagesChange: (value: boolean) => void
  }

  let {
    alwaysLoadImages = $bindable(),
    onAlwaysLoadImagesChange,
  }: Props = $props()

  // State
  let entries = $state<settings.AllowlistEntry[]>([])
  let loading = $state(true)
  let addressesCollapsed = $state(false)
  let domainsCollapsed = $state(false)
  let showAlwaysLoadImagesConfirm = $state(false)

  // Derived
  let addresses = $derived(entries.filter(e => e.type === 'sender'))
  let domains = $derived(entries.filter(e => e.type === 'domain'))

  function handleAlwaysLoadImagesChange(value: boolean) {
    if (value) {
      showAlwaysLoadImagesConfirm = true
      return
    }
    alwaysLoadImages = false
    onAlwaysLoadImagesChange?.(false)
  }

  async function loadData() {
    try {
      entries = await GetImageAllowlist() ?? []
    } catch (err) {
      console.error('Failed to load image allowlist:', err)
    } finally {
      loading = false
    }
  }

  async function handleRemove(id: number) {
    try {
      await RemoveImageAllowlist(id)
      await loadData()
      refreshImageAllowlist()
      addToast({
        type: 'success',
        message: $_('images.removed'),
      })
    } catch (err) {
      console.error('Failed to remove allowlist entry:', err)
    }
  }

  onMount(() => {
    loadData()
  })
</script>

{#if loading}
  <div class="flex items-center justify-center py-8">
    <Icon icon="mdi:loading" class="w-6 h-6 animate-spin text-muted-foreground" />
  </div>
{:else}
  <div>
    <SettingsRow label={$_('settingsGeneral.alwaysLoadImages')}>
      <BoolSelect
        id="always-load-images"
        bind:checked={alwaysLoadImages}
        onCheckedChange={handleAlwaysLoadImagesChange}
        class={SETTINGS_SELECT_WIDTH_CLASS}
      />
    </SettingsRow>

    {#if !alwaysLoadImages}
      <div class="space-y-6 py-6">
        <!-- Addresses Section -->
        <div class="space-y-3">
          <button
            class="w-full flex items-center gap-2 text-sm font-semibold text-foreground hover:text-primary transition-colors text-left"
            onclick={() => addressesCollapsed = !addressesCollapsed}
          >
            {$_('images.addresses')}
            <span class="text-[10px] px-1.5 py-0.5 rounded bg-muted text-muted-foreground font-medium">{addresses.length}</span>
            <Icon icon={addressesCollapsed ? 'mdi:chevron-right' : 'mdi:chevron-down'} class="w-4 h-4 flex-shrink-0" />
            {#if addresses.length === 0}
              <span class="min-w-0 truncate text-sm font-normal text-muted-foreground">{$_('images.noAddresses')}</span>
            {/if}
          </button>

          {#if !addressesCollapsed && addresses.length > 0}
            <div class="space-y-1.5 max-h-48 overflow-y-auto">
              {#each addresses as entry (entry.id)}
                <div class="flex items-center gap-3 p-2 rounded-md border border-border">
                  <Icon icon="mdi:email-outline" class="w-4 h-4 text-muted-foreground flex-shrink-0" />
                  <span class="text-sm flex-1 truncate">{entry.value}</span>
                  <Button variant="ghost" size="sm" onclick={() => handleRemove(entry.id)} title={$_('images.removeButton')}>
                    <Icon icon="mdi:close" class="w-3.5 h-3.5" />
                  </Button>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Domains Section -->
        <div class="space-y-3">
          <button
            class="w-full flex items-center gap-2 text-sm font-semibold text-foreground hover:text-primary transition-colors text-left"
            onclick={() => domainsCollapsed = !domainsCollapsed}
          >
            {$_('images.domains')}
            <span class="text-[10px] px-1.5 py-0.5 rounded bg-muted text-muted-foreground font-medium">{domains.length}</span>
            <Icon icon={domainsCollapsed ? 'mdi:chevron-right' : 'mdi:chevron-down'} class="w-4 h-4 flex-shrink-0" />
            {#if domains.length === 0}
              <span class="min-w-0 truncate text-sm font-normal text-muted-foreground">{$_('images.noDomains')}</span>
            {/if}
          </button>

          {#if !domainsCollapsed && domains.length > 0}
            <div class="space-y-1.5 max-h-48 overflow-y-auto">
              {#each domains as entry (entry.id)}
                <div class="flex items-center gap-3 p-2 rounded-md border border-border">
                  <Icon icon="mdi:web" class="w-4 h-4 text-muted-foreground flex-shrink-0" />
                  <span class="text-sm flex-1 truncate">{entry.value}</span>
                  <Button variant="ghost" size="sm" onclick={() => handleRemove(entry.id)} title={$_('images.removeButton')}>
                    <Icon icon="mdi:close" class="w-3.5 h-3.5" />
                  </Button>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      </div>
    {/if}
  </div>
{/if}

<ConfirmDialog
  bind:open={showAlwaysLoadImagesConfirm}
  title={$_('settingsGeneral.alwaysLoadImagesWarningTitle')}
  description={$_('settingsGeneral.alwaysLoadImagesWarningDescription')}
  confirmLabel={$_('settingsGeneral.disable')}
  cancelLabel={$_('common.cancel')}
  variant="destructive"
  onConfirm={() => { onAlwaysLoadImagesChange?.(true) }}
  onCancel={() => { alwaysLoadImages = false }}
/>
