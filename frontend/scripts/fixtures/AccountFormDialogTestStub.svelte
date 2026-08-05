<script lang="ts">
  let {
    editAccount,
    createdInDialog = false,
    onSubmit,
    onCancel,
  } = $props()

  const config = {
    name: 'Synthetic Account',
    email: 'synthetic@example.test',
    provider: 'imap',
  }
</script>

<section
  data-account-form-dialog
  data-account-id={editAccount?.id ?? ''}
  data-created-in-dialog={createdInDialog}
>
  <button type="button" data-account-form-submit onclick={() => onSubmit(config)}>submit</button>
  <button type="button" data-account-form-cancel onclick={onCancel}>cancel</button>
</section>
