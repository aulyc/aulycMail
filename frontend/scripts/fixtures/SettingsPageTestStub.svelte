<script lang="ts">
  let {
    draft,
    appInfo,
    loading = false,
    initialType = '',
    onOpenActivityLog,
    onAccountOrderChanged,
    onLoadMoreFinished,
  } = $props()

  let selectOpen = $state(false)
</script>

<section data-settings-page-stub data-app-version={appInfo?.version ?? ''} data-loading={loading} data-initial-type={initialType}>
  <button
    type="button"
    data-settings-initial-selection="true"
    data-settings-control-id="toggle-native-title"
    onclick={() => { if (draft) draft.nativeTitleBar = !draft.nativeTitleBar }}
  >toggle native title</button>
  <input aria-label="test setting" value="draft value" />
  <button type="button" data-settings-control-id="before-horizontal">before horizontal</button>
  <div
    data-settings-horizontal-group
    data-settings-horizontal-context="synthetic-account"
    data-settings-arrow-down-target="activity-load-more"
  >
    <button type="button" data-settings-horizontal-action="move-up">move up</button>
    <button type="button" data-settings-horizontal-action="move-down">move down</button>
  </div>
  <button type="button" data-settings-control-id="activity-load-more" onclick={() => onLoadMoreFinished?.(true)}>load more</button>
  <button type="button" data-test-action="load-finished" onclick={() => onLoadMoreFinished?.(false)}>load finished</button>
  <div
    data-settings-horizontal-group
    data-settings-horizontal-arrows-only
    data-settings-arrow-up-target="activity-load-more"
    data-settings-arrow-down-target="missing-control"
    data-settings-arrow-down-fallback="settings-close"
  >
    <button type="button" data-settings-horizontal-action="first">first explicit</button>
    <button type="button" data-settings-horizontal-action="second">second explicit</button>
  </div>
  <div data-settings-horizontal-group data-settings-horizontal-arrows-only>
    <button type="button" data-settings-horizontal-action="outside">outside group</button>
  </div>
  <div data-settings-keyboard-order-group>
    <button type="button" data-settings-control-id="ordered-last" data-settings-keyboard-order="2">ordered last</button>
    <button type="button" data-settings-control-id="ordered-first" data-settings-keyboard-order="1">ordered first</button>
  </div>
  <button
    type="button"
    data-keyboard-select-trigger="true"
    data-settings-control-id="select-trigger"
    aria-expanded={selectOpen}
    onkeydown={(event) => {
      if (event.key === 'Enter' || event.key === ' ') selectOpen = true
    }}
  >select trigger</button>
  <button type="button" data-test-action="close-select" onclick={() => { selectOpen = false }}>close select</button>
  <button type="button" data-test-action="open-activity" onclick={() => onOpenActivityLog?.()}>open activity</button>
  <button type="button" data-test-action="order-changed" onclick={() => onAccountOrderChanged?.('synthetic-account', 'move-down')}>order changed</button>
</section>
