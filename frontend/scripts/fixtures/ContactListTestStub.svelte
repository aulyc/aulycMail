<script lang="ts">
  let { onAdd }: { onAdd: () => void } = $props()
</script>

<button type="button" data-contact-add onclick={onAdd}>Add contact</button>
