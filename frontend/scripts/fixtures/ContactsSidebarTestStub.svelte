<script lang="ts">
  let { onSelect }: { onSelect: () => void } = $props()
</script>

<button type="button" data-contacts-sidebar-select onclick={onSelect}>Select source</button>
