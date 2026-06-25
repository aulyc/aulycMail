<script lang="ts">
  import Icon from '@iconify/svelte'

  // Checkbox — same prop API as Switch (checked / onCheckedChange / id /
  // disabled / class) so the two are drop-in interchangeable. Rendered as a
  // square check toggle.
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
    id,
    class: className = '',
  }: Props = $props()

  function toggle() {
    if (disabled) return
    checked = !checked
    onCheckedChange?.(checked)
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (disabled) return
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      toggle()
    }
  }
</script>

<button
  type="button"
  role="checkbox"
  aria-checked={checked}
  aria-disabled={disabled}
  {id}
  class="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded border transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50 {checked ? 'bg-primary border-primary text-primary-foreground' : 'bg-background border-input'} {className}"
  onclick={toggle}
  onkeydown={handleKeyDown}
  {disabled}
>
  {#if checked}
    <Icon icon="mdi:check" class="w-4 h-4" />
  {/if}
</button>
