<script lang="ts">
  interface Props { description: string }
  let { description }: Props = $props()
</script>

<header class="flex h-5 min-w-0 items-center">
  <p class="truncate text-sm text-muted-foreground">{description}</p>
</header>
