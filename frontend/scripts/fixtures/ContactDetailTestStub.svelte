<script lang="ts">
  let { onEdit }: { onEdit: (contact: Record<string, unknown>) => void } = $props()
</script>

<button type="button" data-contact-edit onclick={() => onEdit({ id: 'contact-from-detail', name: 'Detail Contact' })}>Edit contact</button>
