<script>
  let { accountId, initialMessage, draftId, imagesLoaded, onClose, onSent } = $props()
</script>

<section data-stub="composer" data-account={accountId} data-draft={draftId || ''} data-images-loaded={String(imagesLoaded)}>
  <span data-composer-subject>{initialMessage?.subject || ''}</span>
  <span data-composer-to>{initialMessage?.to?.map((entry) => entry.address || entry.email).join(',') || ''}</span>
  <span data-composer-attachments>{initialMessage?.attachments?.length || 0}</span>
  <button data-app-action="composer-close" onclick={onClose}>Close composer</button>
  <button data-app-action="composer-sent" onclick={onSent}>Sent composer</button>
</section>
