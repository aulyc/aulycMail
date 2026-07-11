<script lang="ts">
  interface Props { title: string; description?: string }
  let { title, description = '' }: Props = $props()
</script>

<header class="flex min-w-0 items-baseline gap-3">
  <h2 class="shrink-0 text-xl font-semibold tracking-tight text-foreground">{title}</h2>
  {#if description}<p class="min-w-0 truncate text-sm text-muted-foreground">{description}</p>{/if}
</header>
