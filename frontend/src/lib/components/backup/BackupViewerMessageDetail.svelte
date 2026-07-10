<script lang="ts">
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { formatFileSize } from '$lib/utils/fileSize'
  // @ts-ignore - wailsjs path
  import type { app } from '../../../../wailsjs/go/models'

  interface Props {
    detail: app.BackupViewerMessageDetail | null
    loadingDetail: boolean
    detailHeaderTitle: string
    attachmentsExpanded: boolean
    savingAttachmentIndexes: Set<number>
    darkFilterStyle: string
    darkFilterEnabled: boolean
    onSaveAttachment: (attachment: app.BackupViewerAttachment, fallbackIndex: number) => void | Promise<void>
    formatDate: (value: string) => string
  }

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
  }: Props = $props()
</script>

<section class="flex min-h-0 flex-col overflow-hidden">
  {#if loadingDetail}
    <div class="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground">
      <Icon icon="mdi:loading" class="mr-2 animate-spin" width="18" height="18" />
      {$_('backupViewer.loading')}
    </div>
  {:else if !detail}
    <div class="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 text-muted-foreground">
      <Icon icon="mdi:email-open-outline" width="42" height="42" />
      <p class="text-sm">{$_('backupViewer.selectMessage')}</p>
    </div>
  {:else}
    <div class="min-h-0 flex-1 overflow-y-auto px-6 py-5 scrollbar-thin">
      <div class="mb-5 space-y-1 rounded-md border border-border bg-muted/20 p-3 text-sm">
        <div class="grid grid-cols-[64px_1fr] gap-2">
          <span class="text-muted-foreground">{$_('backupViewer.subject')}</span>
          <span class="min-w-0 break-words font-semibold">{detailHeaderTitle}</span>
        </div>
        <div class="grid grid-cols-[64px_1fr] gap-2">
          <span class="text-muted-foreground">{$_('backupViewer.from')}</span>
          <span class="min-w-0 break-words">{detail.from?.join(', ') || '-'}</span>
        </div>
        <div class="grid grid-cols-[64px_1fr] gap-2">
          <span class="text-muted-foreground">{$_('backupViewer.to')}</span>
          <span class="min-w-0 break-words">{detail.to?.join(', ') || '-'}</span>
        </div>
        {#if detail.cc?.length}
          <div class="grid grid-cols-[64px_1fr] gap-2">
            <span class="text-muted-foreground">{$_('backupViewer.cc')}</span>
            <span class="min-w-0 break-words">{detail.cc.join(', ')}</span>
          </div>
        {/if}
        {#if detail.bcc?.length}
          <div class="grid grid-cols-[64px_1fr] gap-2">
            <span class="text-muted-foreground">{$_('backupViewer.bcc')}</span>
            <span class="min-w-0 break-words">{detail.bcc.join(', ')}</span>
          </div>
        {/if}
        <div class="grid grid-cols-[64px_1fr] gap-2">
          <span class="text-muted-foreground">{$_('backupViewer.date')}</span>
          <span>{formatDate(detail.date)}</span>
        </div>
        <div class="grid grid-cols-[64px_1fr] gap-2">
          <span class="text-muted-foreground">{$_('backupViewer.folder')}</span>
          <span>{detail.accountEmail}{detail.folderPath ? ` / ${detail.folderPath}` : ''}</span>
        </div>
        <div class="grid grid-cols-[64px_1fr] gap-2">
          <span class="text-muted-foreground">{$_('backupViewer.size')}</span>
          <span>{formatFileSize(detail.size)}</span>
        </div>
      </div>

      {#if detail.attachments?.length}
        <div class="mb-5">
          <div class="overflow-hidden rounded-md border border-border bg-muted/20">
            <button
              type="button"
              class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm font-semibold transition hover:bg-muted/40"
              aria-expanded={attachmentsExpanded}
              onclick={() => attachmentsExpanded = !attachmentsExpanded}
            >
              <Icon icon={attachmentsExpanded ? 'mdi:chevron-down' : 'mdi:chevron-right'} class="h-4 w-4 shrink-0 text-muted-foreground" />
              <span class="min-w-0 flex-1">{$_('backupViewer.attachments')}</span>
              <span class="shrink-0 tabular-nums text-xs text-muted-foreground">{detail.attachments.length}</span>
            </button>
            {#if attachmentsExpanded}
              <div class="border-t border-border">
                {#each detail.attachments as attachment, index (attachment.filename + '-' + index)}
                  {@const attachmentIndex = typeof attachment.index === 'number' ? attachment.index : index}
                  {@const isSavingAttachment = savingAttachmentIndexes.has(attachmentIndex)}
                  <button
                    type="button"
                    class="flex w-full items-center gap-3 border-b border-border px-3 py-2 text-left transition last:border-b-0 hover:bg-muted/40 disabled:cursor-wait disabled:opacity-70"
                    disabled={isSavingAttachment}
                    title={$_('attachment.download')}
                    onclick={() => onSaveAttachment(attachment, index)}
                  >
                    {#if isSavingAttachment}
                      <Icon icon="mdi:loading" class="h-4 w-4 shrink-0 animate-spin text-muted-foreground" />
                    {:else}
                      <Icon icon="mdi:paperclip" class="h-4 w-4 shrink-0 text-muted-foreground" />
                    {/if}
                    <span class="min-w-0 flex-1 truncate text-sm">{attachment.filename}</span>
                    <span class="text-xs text-muted-foreground">{attachment.contentType}</span>
                    <span class="text-xs text-muted-foreground">{formatFileSize(attachment.size)}</span>
                    <Icon icon="mdi:download" class="h-4 w-4 shrink-0 text-muted-foreground" />
                  </button>
                {/each}
              </div>
            {/if}
          </div>
        </div>
      {/if}

      <div class="backup-viewer-body rounded-md border border-border bg-background p-4" style={darkFilterStyle}>
        <div class="backup-viewer-mail-content {darkFilterEnabled ? 'backup-viewer-dark-filter' : ''}">
          {#if detail.hasHTML}
            <!-- eslint-disable-next-line svelte/no-at-html-tags -- backup viewer HTML is sanitized in Go before it reaches the UI -->
            {@html detail.bodyHTML}
          {:else if detail.bodyText}
            <pre class="whitespace-pre-wrap break-words font-sans text-sm leading-6">{detail.bodyText}</pre>
          {:else}
            <p class="text-sm text-muted-foreground">{$_('backupViewer.noBody')}</p>
          {/if}
        </div>
      </div>
    </div>
  {/if}
</section>

<style>
  :global(.backup-viewer-body) {
    max-width: 100%;
    overflow-x: auto;
    color: hsl(var(--foreground));
    font-size: 0.875rem;
    line-height: 1.6;
  }

  :global(.backup-viewer-mail-content) {
    min-height: 2rem;
    max-width: 100%;
    overflow-wrap: anywhere;
  }

  :global(.backup-viewer-body p) {
    margin: 0 0 0.75rem;
  }

  :global(.backup-viewer-body a) {
    color: hsl(var(--primary));
    text-decoration: underline;
  }

  :global(.backup-viewer-body img) {
    max-width: 100%;
    height: auto;
  }

  :global(.backup-viewer-body table) {
    width: auto !important;
    max-width: 100% !important;
    table-layout: auto;
  }

  :global(.backup-viewer-body th),
  :global(.backup-viewer-body td) {
    max-width: 100%;
    white-space: normal !important;
    overflow-wrap: anywhere;
    word-break: break-word;
  }

  :global(.backup-viewer-mail-content.backup-viewer-dark-filter) {
    background: #fff;
    color: #1a1a0a;
    color-scheme: dark;
    filter: var(--backup-viewer-content-filter);
  }

  :global(.backup-viewer-mail-content.backup-viewer-dark-filter a) {
    color: #2563eb;
  }

  :global(.backup-viewer-mail-content.backup-viewer-dark-filter img:not([data-blocked-src])),
  :global(.backup-viewer-mail-content.backup-viewer-dark-filter video),
  :global(.backup-viewer-mail-content.backup-viewer-dark-filter iframe),
  :global(.backup-viewer-mail-content.backup-viewer-dark-filter [data-no-invert]) {
    filter: var(--backup-viewer-media-filter);
  }
</style>
