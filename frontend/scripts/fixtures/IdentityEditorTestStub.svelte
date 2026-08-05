<script lang="ts">
  let {
    open = $bindable(false),
    identity,
    linkedName,
    onClose,
    onNameChange,
    onSave,
  }: {
    open?: boolean
    accountId: string
    identity?: { id?: string; isDefault?: boolean } | null
    linkedName?: string
    onClose?: () => void
    onNameChange?: (value: string) => void
    onSave?: (config: Record<string, unknown>) => void | Promise<void>
  } = $props()

  async function save() {
    await onSave?.({ email: 'saved@example.test', name: 'Saved Name' })
    open = false
    onClose?.()
  }

  function close() {
    open = false
    onClose?.()
  }
</script>

{#if open}
  <div data-identity-editor data-identity-id={identity?.id ?? ''} data-linked-name={linkedName ?? ''}>
    <button type="button" data-identity-name onclick={() => onNameChange?.('Draft Name')}>Change name</button>
    <button type="button" data-identity-save onclick={save}>Save identity</button>
    <button type="button" data-identity-close onclick={close}>Close identity</button>
  </div>
{/if}
