<script lang="ts">
  import { _ } from '$lib/i18n'
  import type { SettingsDraft } from '../settingsDraft.svelte'
  import ImagesTab from '../ImagesTab.svelte'
  import SettingsPageHeader from '../shared/SettingsPageHeader.svelte'
  interface Props { draft: SettingsDraft }
  let { draft }: Props = $props()
</script>
<div class="space-y-6">
  <SettingsPageHeader description={$_('settingsDescriptions.imagePrivacy')} />
  <ImagesTab bind:alwaysLoadImages={draft.alwaysLoadImages} onAlwaysLoadImagesChange={(value) => draft.alwaysLoadImages = value} />
</div>
