<script lang="ts">
  let {
    detail,
    loadingDetail,
    detailHeaderTitle,
    attachmentsExpanded = $bindable(true),
    savingAttachmentIndexes,
    darkFilterStyle,
    darkFilterEnabled,
    onSaveAttachment,
    formatDate,
  } = $props()
</script>

<section
  data-backup-detail
  data-key={detail?.key ?? ''}
  data-loading={loadingDetail}
  data-title={detailHeaderTitle}
  data-expanded={attachmentsExpanded}
  data-saving={[...savingAttachmentIndexes].join(',')}
  data-dark={darkFilterEnabled}
  style={darkFilterStyle}
>
  <span data-detail-date>{formatDate(detail?.date ?? '')}</span>
  <button type="button" data-backup-detail-action="toggle" onclick={() => attachmentsExpanded = !attachmentsExpanded}>toggle</button>
  <button
    type="button"
    data-backup-detail-action="save"
    onclick={() => onSaveAttachment({ index: 1, filename: 'synthetic.pdf' }, 0)}
  >save</button>
  <button
    type="button"
    data-backup-detail-action="save-fallback"
    onclick={() => onSaveAttachment({ filename: 'fallback.bin' }, 7)}
  >save fallback</button>
</section>
