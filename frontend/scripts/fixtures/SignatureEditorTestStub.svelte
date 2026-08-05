<script lang="ts">
  let {
    value = '',
    onchange,
  }: {
    value?: string
    placeholder?: string
    onchange?: (value: string) => void
  } = $props()
</script>

<textarea
  data-signature-editor-stub
  {value}
  oninput={(event) => onchange?.((event.currentTarget as HTMLTextAreaElement).value)}
></textarea>
