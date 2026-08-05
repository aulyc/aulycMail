<script lang="ts">
  let {
    open = $bindable(false),
    contact,
  }: {
    open?: boolean
    contact?: { id?: string } | null
  } = $props()
</script>

{#if open}
  <div data-contact-edit-dialog data-contact-id={contact?.id ?? ''}>Edit dialog</div>
{/if}
