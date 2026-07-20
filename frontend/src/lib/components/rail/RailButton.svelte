<script lang="ts">
  import Icon from '@iconify/svelte'

  interface Props {
    icon: string
    label: string
    active: boolean
    selected?: boolean
    onclick: () => void
  }

  const { icon, label, active, selected = false, onclick }: Props = $props()
</script>

<button
  tabindex="-1"
  class="flex items-center justify-center w-12 h-12 border-l-[3px] transition-colors duration-150 cursor-pointer focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary focus-visible:-outline-offset-2 {selected && !active ? 'bg-accent/30 text-primary' : ''} {active
    ? 'border-l-primary text-primary bg-accent/40'
    : 'border-l-transparent text-muted-foreground hover:text-foreground hover:bg-accent/30'}"
  type="button"
  title={label}
  aria-label={label}
  aria-pressed={active}
  {onclick}
>
  <Icon {icon} width="22" height="22" />
</button>
