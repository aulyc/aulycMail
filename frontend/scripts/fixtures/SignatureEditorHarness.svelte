<script lang="ts">
  import SignatureEditor from '../../src/lib/components/settings/account/SignatureEditor.svelte'

  let {
    initialValue = '',
    onchange,
  }: {
    initialValue?: string
    onchange?: (value: string) => void
  } = $props()

  let value = $state('')
  let initialized = $state(false)

  $effect(() => {
    if (!initialized) {
      value = initialValue
      initialized = true
    }
  })

  function handleChange(next: string) {
    value = next
    onchange?.(next)
  }
</script>

<SignatureEditor {value} onchange={handleChange} placeholder="Synthetic placeholder" />
