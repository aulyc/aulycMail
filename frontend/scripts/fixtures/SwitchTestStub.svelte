<script lang="ts">
  let {
    checked = $bindable(false),
    disabled = false,
    onCheckedChange,
  }: {
    checked?: boolean
    disabled?: boolean
    onCheckedChange?: (checked: boolean) => void
  } = $props()

  function change(event: Event) {
    checked = (event.target as HTMLInputElement).checked
    onCheckedChange?.(checked)
  }
</script>

<input type="checkbox" data-switch bind:checked {disabled} onchange={change} />
