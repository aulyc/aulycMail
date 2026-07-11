<script lang="ts">
  interface Props { title: string; description?: string }
  let { title, description = '' }: Props = $props()
</script>

<header class="space-y-1">
  <h2 class="text-xl font-semibold tracking-tight text-foreground">{title}</h2>
  {#if description}<p class="text-sm text-muted-foreground">{description}</p>{/if}
</header>
