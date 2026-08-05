<script lang="ts">
  let { open = $bindable(false), testing = false, result } = $props()
</script>

{#if open}
  <section data-connection-dialog data-testing={testing} data-success={result?.success ?? ''}>
    {result?.message ?? ''}
  </section>
{/if}
