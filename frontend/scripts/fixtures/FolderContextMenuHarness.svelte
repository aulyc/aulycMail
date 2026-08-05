<script lang="ts">
  import FolderContextMenu from '../../src/lib/components/sidebar/FolderContextMenu.svelte'

  let { folderId = 'folder-1', disabled = false } = $props<{
    folderId?: string
    disabled?: boolean
  }>()
</script>

<FolderContextMenu {folderId} {disabled}>
  <button type="button" data-folder-child>folder child</button>
</FolderContextMenu>
