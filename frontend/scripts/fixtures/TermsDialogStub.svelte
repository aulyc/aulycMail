<script>
  let { open = $bindable(false), onAccept } = $props()
</script>

{#if open}
  <div role="dialog" data-stub="terms-dialog">
    <button data-app-action="accept-terms" onclick={onAccept}>Accept terms</button>
  </div>
{/if}
