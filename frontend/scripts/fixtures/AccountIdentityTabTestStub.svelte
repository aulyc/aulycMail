<script lang="ts">
  let { accountId, defaultDisplayName, onDefaultDisplayNameChange } = $props()
</script>

<section data-identity-tab data-account-id={accountId} data-display-name={defaultDisplayName}>
  <button type="button" data-identity-action="rename" onclick={() => onDefaultDisplayNameChange?.('Updated Identity')}>rename</button>
</section>
