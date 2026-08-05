<script>
  let { open = $bindable(false), certificate, onAcceptOnce, onAcceptPermanently, onDecline } = $props()
</script>

{#if open}
  <div role="dialog" data-stub="certificate-dialog" data-host={certificate?.host || ''}>
    <button data-app-action="accept-cert-once" onclick={onAcceptOnce}>Accept once</button>
    <button data-app-action="accept-cert-always" onclick={onAcceptPermanently}>Accept always</button>
    <button data-app-action="decline-cert" onclick={onDecline}>Decline</button>
  </div>
{/if}
