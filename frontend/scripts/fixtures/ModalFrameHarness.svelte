<script lang="ts">
  import ModalFrame from '../../src/lib/components/ui/ModalFrame.svelte'

  let { open = true, mode = 'buttons', onClose } = $props<{
    open?: boolean
    mode?: 'buttons' | 'none'
    onClose?: () => void
  }>()
</script>

<ModalFrame {open} {onClose} panelClass="test-panel" labelledBy="modal-title">
  <h2 id="modal-title">Synthetic modal</h2>
  {#if mode === 'buttons'}
    <button type="button" data-first>First</button>
    <button type="button" disabled>Disabled</button>
    <a href="#hidden" aria-hidden="true">Hidden</a>
    <button type="button" data-last>Last</button>
  {/if}
</ModalFrame>
