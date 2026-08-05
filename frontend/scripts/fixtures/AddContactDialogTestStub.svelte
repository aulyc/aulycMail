<script lang="ts">
  let {
    open = $bindable(false),
    onCreated,
  }: {
    open?: boolean
    onCreated: (id: string) => void | Promise<void>
  } = $props()

  async function create() {
    await onCreated('created-contact')
    open = false
  }
</script>

{#if open}
  <button type="button" data-add-contact-create onclick={create}>Create contact</button>
{/if}
