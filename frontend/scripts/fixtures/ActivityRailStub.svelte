<script>
  let { onOpenSettings } = $props()
  let settingsButton

  export function selectSettingsEntry() {
    settingsButton?.setAttribute('data-selected', 'true')
  }

  export function focusSettings() {
    settingsButton?.focus()
  }
</script>

<nav data-stub="activity-rail">
  <button bind:this={settingsButton} data-app-action="open-settings" onclick={onOpenSettings}>Settings</button>
</nav>
