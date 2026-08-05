<script lang="ts">
  let {
    open = $bindable(false),
    certificate,
    onAcceptOnce,
    onAcceptPermanently,
    onDecline,
  } = $props()
</script>

{#if open}
  <section data-certificate-dialog data-host={certificate?.host ?? ''}>
    <button type="button" data-cert-action="once" onclick={onAcceptOnce}>once</button>
    <button type="button" data-cert-action="permanent" onclick={onAcceptPermanently}>permanent</button>
    <button type="button" data-cert-action="decline" onclick={onDecline}>decline</button>
  </section>
{/if}
