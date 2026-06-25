<script lang="ts">
  // BoolSelect — a 是/否 (Yes/No) dropdown with the same prop API as Checkbox
  // (checked / onCheckedChange / id / disabled / class), so it's a drop-in
  // replacement. Used for boolean settings so they match the other one-line
  // dropdown rows.
  import * as Select from '$lib/components/ui/select'
  import { _ } from '$lib/i18n'

  interface Props {
    checked?: boolean
    disabled?: boolean
    onCheckedChange?: (checked: boolean) => void
    id?: string
    class?: string
  }

  let {
    checked = $bindable(false),
    disabled = false,
    onCheckedChange,
    class: className = 'w-24',
  }: Props = $props()

  const yesLabel = $derived($_('common.yes'))
  const noLabel = $derived($_('common.no'))

  function handle(v: string | undefined) {
    const b = v === 'yes'
    checked = b
    onCheckedChange?.(b)
  }
</script>

<Select.Root value={checked ? 'yes' : 'no'} onValueChange={handle} disabled={disabled}>
  <Select.Trigger class="{className} shrink-0">
    <Select.Value>{checked ? yesLabel : noLabel}</Select.Value>
  </Select.Trigger>
  <Select.Content>
    <Select.Item value="yes" label={yesLabel} />
    <Select.Item value="no" label={noLabel} />
  </Select.Content>
</Select.Root>
